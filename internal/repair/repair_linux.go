//go:build linux

package repair

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/Monska85/hostlens/internal/auth"
	"github.com/Monska85/hostlens/internal/containers"
	"github.com/Monska85/hostlens/internal/core"
	"github.com/Monska85/hostlens/internal/localipc"
	"github.com/Monska85/hostlens/internal/server"
	"github.com/Monska85/hostlens/internal/status"
)

const maxFrame = 1 << 16

type request struct {
	Secret string `json:"secret"`
	Tool   string `json:"tool"`
	Target string `json:"target"`
}

type response struct {
	Service   *status.Service       `json:"service,omitempty"`
	Container *containers.Container `json:"container,omitempty"`
	Invoked   *bool                 `json:"invoked,omitempty"`
	Completed *bool                 `json:"completed,omitempty"`
	Issue     string                `json:"issue,omitempty"`
}

type Server struct {
	Socket     string
	ConfigPath string
	GatewayUID uint32
	SharedGID  int
	Tokens     auth.Store
	Services   status.ServiceRestarter
	Containers containers.Restarter
	Audit      io.Writer
}

type peerKey struct{}

type auditEvent struct {
	Time      time.Time `json:"time"`
	RequestID string    `json:"request_id"`
	TokenID   string    `json:"token_id,omitempty"`
	Tool      string    `json:"tool,omitempty"`
	Target    string    `json:"target,omitempty"`
	Decision  string    `json:"decision"`
	Outcome   string    `json:"outcome"`
}

func (s Server) record(event auditEvent) error {
	writer := s.Audit
	if writer == nil {
		writer = os.Stderr
	}
	event.Time = time.Now().UTC()
	return json.NewEncoder(writer).Encode(event)
}

func (s Server) handle(writer http.ResponseWriter, httpRequest *http.Request) {
	event := auditEvent{RequestID: rand.Text(), Decision: "denied", Outcome: "denied"}
	deny := func(code int, message string) {
		if err := s.record(event); err != nil {
			http.Error(writer, "repair audit unavailable", http.StatusServiceUnavailable)
			return
		}
		http.Error(writer, message, code)
	}
	if httpRequest.Context().Value(peerKey{}) != s.GatewayUID || httpRequest.Method != http.MethodPost || httpRequest.URL.Path != "/restart" {
		deny(http.StatusForbidden, "denied")
		return
	}
	data, err := io.ReadAll(io.LimitReader(httpRequest.Body, maxFrame+1))
	if err != nil || len(data) > maxFrame {
		deny(http.StatusBadRequest, "request exceeds limit")
		return
	}
	var input request
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		deny(http.StatusBadRequest, "invalid request")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		deny(http.StatusBadRequest, "invalid request")
		return
	}
	config, policy, err := core.Load(s.ConfigPath, server.ToolEffects())
	if err != nil || config.ReadOnly || config.GatewayUID != s.GatewayUID || config.RepairUID != uint32(os.Geteuid()) || config.TokenStore != s.Tokens.Path || config.RepairSocket != s.Socket {
		deny(http.StatusForbidden, "repair disabled")
		return
	}
	token, err := s.Tokens.Verify(input.Secret)
	if err != nil {
		deny(http.StatusForbidden, "identity denied")
		return
	}
	event.TokenID = token.ID
	authorizedRole := false
	for _, role := range token.Roles {
		if role == string(core.RoleRepair) {
			authorizedRole = true
		}
	}
	if !authorizedRole || !policy.Allows(core.EffectRepair, input.Tool, input.Target) {
		deny(http.StatusForbidden, "repair denied")
		return
	}
	event.Tool, event.Target = input.Tool, input.Target
	event.Decision, event.Outcome = "allowed", "started"
	if err := s.record(event); err != nil {
		http.Error(writer, "repair audit unavailable", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(httpRequest.Context(), 30*time.Second)
	defer cancel()
	var output response
	switch input.Tool {
	case "restart_service":
		if s.Services == nil {
			err = errors.New("service repair unavailable")
		} else {
			var result status.RestartOutcome
			result, err = s.Services.RestartService(ctx, input.Target)
			output.Service, output.Invoked, output.Completed, output.Issue = result.Service, result.Invoked, result.Completed, result.Issue
		}
	case "restart_container":
		if s.Containers == nil {
			err = errors.New("container repair unavailable")
		} else {
			var result containers.RestartOutcome
			result, err = s.Containers.Restart(ctx, input.Target)
			output.Container, output.Invoked, output.Completed, output.Issue = result.Container, result.Invoked, result.Completed, result.Issue
		}
	default:
		err = errors.New("unknown repair")
	}
	if err != nil {
		event.Outcome = "failed"
		if s.record(event) != nil {
			http.Error(writer, "repair audit unavailable; outcome unknown", http.StatusServiceUnavailable)
			return
		}
		http.Error(writer, "repair failed", http.StatusServiceUnavailable)
		return
	}
	event.Outcome = "completed"
	if output.Issue != "" {
		event.Outcome = output.Issue
	}
	if s.record(event) != nil {
		http.Error(writer, "repair audit unavailable; outcome unknown", http.StatusServiceUnavailable)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(output)
}

func (s Server) Run(ctx context.Context) error {
	if s.Socket == "" || s.ConfigPath == "" {
		return errors.New("repair socket and configuration required")
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
	slots := make(chan struct{}, 1)
	httpServer := &http.Server{ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 40 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 4096, Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(writer, "busy", http.StatusServiceUnavailable)
			return
		}
		s.handle(writer, request)
	})}
	httpServer.ConnContext = func(ctx context.Context, conn net.Conn) context.Context {
		uid, err := localipc.PeerUID(conn)
		if err != nil {
			uid = ^uint32(0)
		}
		return context.WithValue(ctx, peerKey{}, uid)
	}
	return localipc.ServeUntilStopped(ctx, httpServer, listener, 35*time.Second)
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
			return nil, errors.New("repair peer identity mismatch")
		}
		return conn, nil
	}
	return &Client{http: &http.Client{Transport: transport, Timeout: 35 * time.Second}}
}

func (c *Client) call(ctx context.Context, secret, tool, target string, result any) error {
	body, err := json.Marshal(request{Secret: secret, Tool: tool, Target: target})
	if err != nil {
		return err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://repair.localhost/restart", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, err := c.http.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("repair unavailable: %w", err)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK || httpResponse.ContentLength > maxFrame {
		return fmt.Errorf("repair refused: HTTP %d", httpResponse.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(httpResponse.Body, maxFrame+1))
	if err != nil || len(data) > maxFrame {
		return errors.New("repair response exceeds limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(result); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("invalid repair response")
	}
	return nil
}

func (c *Client) RestartService(ctx context.Context, secret, target string) (status.RestartOutcome, error) {
	var result response
	err := c.call(ctx, secret, "restart_service", target, &result)
	if err == nil && result.Completed != nil && *result.Completed && result.Issue == "" && result.Service == nil {
		err = errors.New("missing service result")
	}
	if err != nil {
		return status.RestartOutcome{}, err
	}
	return status.RestartOutcome{Service: result.Service, Invoked: result.Invoked, Completed: result.Completed, Issue: result.Issue}, nil
}

func (c *Client) RestartContainer(ctx context.Context, secret, target string) (containers.RestartOutcome, error) {
	var result response
	err := c.call(ctx, secret, "restart_container", target, &result)
	if err == nil && result.Completed != nil && *result.Completed && result.Issue == "" && result.Container == nil {
		err = errors.New("missing container result")
	}
	if err != nil {
		return containers.RestartOutcome{}, err
	}
	return containers.RestartOutcome{Container: result.Container, Invoked: result.Invoked, Completed: result.Completed, Issue: result.Issue}, nil
}
