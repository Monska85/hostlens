package gateway

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type secretKey struct{}
type peerKey struct{}
type clientKey struct{}

func ClientIP(peer, header string, trusted []string) string {
	a, e := netip.ParseAddr(peer)
	if e != nil {
		return peer
	}
	isTrusted := func(ip netip.Addr) bool {
		for _, s := range trusted {
			if p, e := netip.ParsePrefix(s); e == nil && p.Contains(ip) {
				return true
			}
			if p, e := netip.ParseAddr(s); e == nil && p == ip {
				return true
			}
		}
		return false
	}
	if !isTrusted(a) || header == "" {
		return peer
	}
	chain := strings.Split(header, ",")
	if len(chain) > 64 {
		return peer
	}
	parsed := make([]netip.Addr, len(chain))
	for i, s := range chain {
		p, e := netip.ParseAddr(strings.TrimSpace(s))
		if e != nil {
			return peer
		}
		parsed[i] = p
	}
	for i := len(parsed) - 1; i >= 0; i-- {
		if !isTrusted(parsed[i]) {
			return parsed[i].String()
		}
	}
	return peer
}
func (c *Coordinator) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		c.mu.Lock()
		cfg := c.Active.Config
		if c.running >= cfg.Limits.Concurrent {
			c.mu.Unlock()
			http.Error(w, "overload", http.StatusServiceUnavailable)
			return
		}
		c.running++
		c.mu.Unlock()
		defer func() { c.mu.Lock(); c.running--; c.mu.Unlock() }()
		ctx, cancel := context.WithTimeout(r.Context(), cfg.Limits.ToolTimeout)
		defer cancel()
		operationContext := ctx
		// Leave time for the SDK to serialize an explicit operation timeout.
		responseContext, cancelResponse := context.WithTimeout(r.Context(), cfg.Limits.ToolTimeout+5*time.Second)
		defer cancelResponse()
		deadline, _ := ctx.Deadline()
		controller := http.NewResponseController(w)
		if err := controller.SetReadDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
			http.Error(w, "request deadline unavailable", http.StatusInternalServerError)
			return
		}
		if err := controller.SetWriteDeadline(deadline.Add(5 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			http.Error(w, "response deadline unavailable", http.StatusInternalServerError)
			return
		}
		r = r.WithContext(ctx)
		peer, _, _ := net.SplitHostPort(r.RemoteAddr)
		client := ClientIP(peer, r.Header.Get(cfg.Server.ClientIPHeader), cfg.Server.TrustedProxies)
		auth := r.Header.Values("Authorization")
		if len(auth) != 1 || !strings.HasPrefix(auth[0], "Bearer ") {
			c.Log.Error("authentication_failed", "peer_ip", peer, "client_ip", client)
			http.Error(w, "authentication required", 401)
			return
		}
		secret := strings.TrimPrefix(auth[0], "Bearer ")
		identity, e := c.Tokens.Verify(secret)
		if e != nil {
			c.Log.Error("authentication_failed", "peer_ip", peer, "client_ip", client)
			http.Error(w, "invalid credential", 401)
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" {
			ok := origin == "http://"+r.Host || origin == "https://"+r.Host
			for _, allowed := range cfg.Server.AllowedOrigins {
				if origin == allowed {
					ok = true
				}
			}
			if !ok {
				c.Log.Error("origin_denied", "token_id", identity.ID, "peer_ip", peer)
				http.Error(w, "origin denied", 403)
				return
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, int64(cfg.Limits.RequestBytes))
		if r.ContentLength > int64(cfg.Limits.RequestBytes) {
			http.Error(w, "request too large", 413)
			return
		}
		ctx = context.WithValue(responseContext, secretKey{}, secret)
		ctx = context.WithValue(ctx, peerKey{}, peer)
		ctx = context.WithValue(ctx, clientKey{}, client)
		r = r.WithContext(ctx)
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return c.MCP(operationContext, identity) }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: int64(cfg.Limits.RequestBytes), PropagateRequestCancellation: true})
		handler.ServeHTTP(w, r)
	})
}
func Listen(cfg config.Config) ([]net.Listener, error) {
	var tlsConfig *tls.Config
	if cfg.Server.TLS.Enabled {
		pair, e := tls.LoadX509KeyPair(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile)
		if e != nil {
			return nil, e
		}
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair}}
	}
	listeners := []net.Listener{}
	for _, bind := range cfg.Server.Bind {
		family := "tcp4"
		if strings.Contains(bind, ":") {
			family = "tcp6"
		}
		l, e := net.Listen(family, net.JoinHostPort(bind, strconv.Itoa(cfg.Server.Port)))
		if e != nil {
			for _, opened := range listeners {
				opened.Close()
			}
			return nil, e
		}
		if tlsConfig != nil {
			l = tls.NewListener(l, tlsConfig)
		}
		listeners = append(listeners, l)
	}
	return listeners, nil
}
func Serve(ctx context.Context, listeners []net.Listener, handler http.Handler, cfg config.Config) error {
	// Handlers apply the active operation timeout; fallback deadlines also cover
	// requests rejected before admission and never constrain a valid reload.
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: time.Minute + 5*time.Second, WriteTimeout: time.Minute + 5*time.Second, IdleTimeout: cfg.Limits.IdleTimeout, MaxHeaderBytes: 16384}
	ch := make(chan error, len(listeners))
	for _, l := range listeners {
		go func() { ch <- server.Serve(l) }()
	}
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	case e := <-ch:
		server.Close()
		if errors.Is(e, http.ErrServerClosed) {
			return nil
		}
		return e
	}
}
