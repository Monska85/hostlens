package gateway

import (
	"context"
	"errors"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/telemetry"
	"github.com/Monska85/hostlens/internal/token"
	dto "github.com/prometheus/client_model/go"
)

func (c *Coordinator) telemetry() *telemetry.Registry {
	c.metricsOnce.Do(func() { c.metrics = telemetry.New("gateway") })
	return c.metrics
}
func (c *Coordinator) scrape(w http.ResponseWriter, r *http.Request, cfg config.Config) {
	ctx, cancel := context.WithTimeout(r.Context(), telemetry.ScrapeTimeout)
	defer cancel()
	deadline, _ := ctx.Deadline()
	controller := http.NewResponseController(w)
	for _, set := range []func(time.Time) error{controller.SetReadDeadline, controller.SetWriteDeadline} {
		if err := set(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
			http.Error(w, "scrape deadline unavailable", 500)
			return
		}
	}

	if !cfg.Metrics.Enabled {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "GET required", 405)
		return
	}
	if !c.scrapes.Acquire() {
		c.telemetry().Reject("metrics", "overload")
		http.Error(w, "scrape overload", 503)
		return
	}
	registry := c.telemetry()
	registry.StartHTTP("metrics")
	reply, err := c.scrapes.Run(ctx, func() telemetry.Reply {
		defer registry.EndHTTP("metrics")
		requestID := token.Random(12)
		requestIdentity := ""
		peer, _, _ := net.SplitHostPort(r.RemoteAddr)
		client := ClientIP(peer, r.Header.Get(cfg.Server.ClientIPHeader), cfg.Server.TrustedProxies)
		audit := func(event, identity string) {
			c.Log.Error(event, "component", "gateway", "route", "metrics", "request_id", requestID, "token_id", identity, "peer_ip", peer, "client_ip", client)
		}

		if !cfg.Metrics.AllowAnonymous {
			auth := r.Header.Values("Authorization")
			if len(auth) != 1 || !strings.HasPrefix(auth[0], "Bearer ") {
				audit("authentication_failed", "")
				return telemetry.Reply{Status: 401}
			}
			identity, err := c.Tokens.Verify(strings.TrimPrefix(auth[0], "Bearer "))
			if err != nil {
				audit("authentication_failed", "")
				return telemetry.Reply{Status: 401}
			}
			requestIdentity = identity.ID
			if !slices.Contains(identity.Roles, "metrics") {
				audit("authorization_denied", identity.ID)
				return telemetry.Reply{Status: 403}
			}
		}
		if ctx.Err() != nil {
			return telemetry.Reply{Status: 503}
		}
		families, err := registry.Gather()
		if err != nil {
			return telemetry.Reply{Status: 500}
		}
		backend, err := c.backendMetrics(ctx)
		if err != nil {
			audit("backend_telemetry_unavailable", requestIdentity)
		}
		families = mergeMetrics(families, backend)
		families = append(families, telemetry.BackendUp(err == nil))
		body, err := telemetry.Encode(families)
		if err != nil {
			return telemetry.Reply{Status: 500}
		}
		return telemetry.Reply{Body: body, Status: 200}
	})
	if err != nil {
		http.Error(w, "scrape deadline exceeded", 503)
		return
	}
	if reply.Status != 200 {
		http.Error(w, http.StatusText(reply.Status), reply.Status)
		return
	}
	w.Header().Set("Content-Type", telemetry.ContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(reply.Body)
}
func (c *Coordinator) backendMetrics(ctx context.Context) ([]*dto.MetricFamily, error) {
	ctx, cancel := context.WithTimeout(ctx, telemetry.BackendTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", "http://unix/telemetry", nil)
	if err != nil {
		return nil, err
	}
	if c.HTTP == nil {
		return nil, errors.New("backend unavailable")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("backend telemetry rejected")
	}
	return telemetry.DecodeBackend(resp.Body)
}
func mergeMetrics(gateway, backend []*dto.MetricFamily) []*dto.MetricFamily {
	byName := make(map[string]*dto.MetricFamily, len(gateway))
	for _, f := range gateway {
		byName[f.GetName()] = f
	}
	for _, f := range backend {
		if existing := byName[f.GetName()]; existing != nil {
			existing.Metric = append(existing.Metric, f.Metric...)
		} else {
			gateway = append(gateway, f)
		}
	}
	return gateway
}

type measuredResponse struct {
	http.ResponseWriter
	status int
}

func (w *measuredResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *measuredResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *measuredResponse) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *measuredResponse) Flush() {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
func (c *Coordinator) measureHTTP(w http.ResponseWriter, route string) (http.ResponseWriter, func()) {
	measured := &measuredResponse{ResponseWriter: w}
	started := time.Now()
	return measured, func() {
		status := measured.status
		if status == 0 {
			status = 200
		}
		c.telemetry().HTTP(route, status, time.Since(started))
	}
}
