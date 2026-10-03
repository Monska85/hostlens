//go:build linux

package observer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Monska85/hostlens/internal/containers"
	"github.com/Monska85/hostlens/internal/localipc"
)

const maxMessageBytes = 1 << 20

type peerKey struct{}

type Server struct {
	Socket     string
	GatewayUID uint32
	SharedGID  int
	Docker     containers.Reader
}

func (s Server) Run(ctx context.Context) error {
	if s.Socket == "" || s.Docker == nil {
		return errors.New("observer socket and Docker reader required")
	}
	listener, err := net.Listen("unix", s.Socket)
	if err != nil {
		return err
	}
	defer listener.Close()
	created, err := os.Lstat(s.Socket)
	if err != nil {
		return err
	}
	defer func() {
		if info, err := os.Lstat(s.Socket); err == nil && os.SameFile(created, info) {
			os.Remove(s.Socket)
		}
	}()
	if err := os.Chmod(s.Socket, 0660); err != nil {
		return err
	}
	if s.SharedGID > 0 {
		if err := os.Chown(s.Socket, -1, s.SharedGID); err != nil {
			return err
		}
	}
	server := &http.Server{ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 4096}
	server.ConnContext = func(ctx context.Context, conn net.Conn) context.Context {
		uid, err := localipc.PeerUID(conn)
		if err != nil {
			return context.WithValue(ctx, peerKey{}, uint32(^uint32(0)))
		}
		return context.WithValue(ctx, peerKey{}, uid)
	}
	slots := make(chan struct{}, 8)
	server.Handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(writer, "busy", http.StatusServiceUnavailable)
			return
		}
		if request.Context().Value(peerKey{}) != s.GatewayUID {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		if request.Method != http.MethodGet || request.ContentLength != 0 {
			http.Error(writer, "method denied", http.StatusMethodNotAllowed)
			return
		}
		var value any
		var err error
		switch {
		case request.URL.Path == "/engine":
			value, err = s.Docker.Engine(request.Context())
		case request.URL.Path == "/containers":
			value, err = s.Docker.Containers(request.Context())
		case strings.HasPrefix(request.URL.Path, "/container/"):
			id := strings.TrimPrefix(request.URL.Path, "/container/")
			value, err = s.Docker.Container(request.Context(), id)
		default:
			http.NotFound(writer, request)
			return
		}
		if err != nil {
			http.Error(writer, "Docker observation unavailable", http.StatusServiceUnavailable)
			return
		}
		data, err := json.Marshal(value)
		if err != nil || len(data) > maxMessageBytes {
			http.Error(writer, "observation exceeds limit", http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Write(data)
	})
	return localipc.ServeUntilStopped(ctx, server, listener, 15*time.Second)
}

type Client struct{ http *http.Client }

func NewClient(socket string, expectedUID uint32) *Client {
	transport := &http.Transport{DisableKeepAlives: true}
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, "unix", socket)
		if err != nil {
			return nil, err
		}
		uid, err := localipc.PeerUID(conn)
		if err != nil || uid != expectedUID {
			conn.Close()
			return nil, errors.New("observer peer identity mismatch")
		}
		return conn, nil
	}
	return &Client{http: &http.Client{Transport: transport, Timeout: 10 * time.Second}}
}

func (c *Client) get(ctx context.Context, endpoint string, result any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://observer.localhost"+endpoint, nil)
	if err != nil {
		return err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("observer unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > maxMessageBytes {
		return fmt.Errorf("observer refused observation: HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxMessageBytes+1))
	if err != nil || len(data) > maxMessageBytes {
		return errors.New("observer response exceeds limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(result); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("trailing observer response")
	}
	return nil
}

func (c *Client) Engine(ctx context.Context) (containers.Engine, error) {
	var result containers.Engine
	err := c.get(ctx, "/engine", &result)
	return result, err
}

func (c *Client) Containers(ctx context.Context) (containers.Inventory, error) {
	var result containers.Inventory
	err := c.get(ctx, "/containers", &result)
	return result, err
}

func (c *Client) Container(ctx context.Context, id string) (containers.Container, error) {
	if !containers.ValidID(id) {
		return containers.Container{}, errors.New("invalid container ID")
	}
	var result containers.Container
	err := c.get(ctx, "/container/"+id, &result)
	return result, err
}
