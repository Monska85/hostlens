package containers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/moby/moby/client"
)

const maxDockerResponse = 8 << 20

type Engine struct {
	ObservedAt time.Time `json:"observed_at"`
	Version    string    `json:"version,omitempty"`
	API        string    `json:"api_version,omitempty"`
	OS         string    `json:"os,omitempty"`
	Arch       string    `json:"architecture,omitempty"`
}

type Container struct {
	ObservedAt time.Time `json:"observed_at"`
	ID         string    `json:"id"`
	Names      []string  `json:"names,omitempty"`
	Image      string    `json:"image,omitempty"`
	State      string    `json:"state,omitempty"`
	Status     string    `json:"status,omitempty"`
}

type Inventory struct {
	ObservedAt time.Time   `json:"observed_at"`
	Containers []Container `json:"containers"`
	Limit      int         `json:"limit"`
}

type Reader interface {
	Engine(context.Context) (Engine, error)
	Containers(context.Context) (Inventory, error)
	Container(context.Context, string) (Container, error)
}

type Restarter interface {
	Restart(context.Context, string) (RestartOutcome, error)
}

type RestartOutcome struct {
	Container *Container `json:"container,omitempty"`
	Invoked   *bool      `json:"invoked,omitempty"`
	Completed *bool      `json:"completed,omitempty"`
	Issue     string     `json:"issue,omitempty"`
}

func known(value bool) *bool { return &value }

type dockerClient struct{ api *client.Client }

type cappedTransport struct {
	base  http.RoundTripper
	write bool
}

func allowedPath(method, raw string, write bool) bool {
	parts := strings.Split(strings.Trim(raw, "/"), "/")
	if len(parts) == 1 && (method == http.MethodGet || method == http.MethodHead && parts[0] == "_ping") && (parts[0] == "_ping" || parts[0] == "version") {
		return true
	}
	if len(parts) == 0 || !strings.HasPrefix(parts[0], "v1.") {
		return false
	}
	parts = parts[1:]
	if method == http.MethodGet {
		if len(parts) == 1 && (parts[0] == "_ping" || parts[0] == "version" || parts[0] == "info") {
			return true
		}
		if len(parts) == 2 && parts[0] == "containers" && parts[1] == "json" {
			return true
		}
		return len(parts) == 3 && parts[0] == "containers" && validID(parts[1]) && parts[2] == "json"
	}
	return write && method == http.MethodPost && len(parts) == 3 && parts[0] == "containers" && validID(parts[1]) && parts[2] == "restart"
}

func validID(id string) bool {
	if len(id) != 64 {
		return false
	}
	for _, char := range id {
		if char < '0' || char > '9' && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func ValidID(id string) bool { return validID(id) }

func (t cappedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !allowedPath(req.Method, req.URL.Path, t.write) {
		return nil, errors.New("Docker operation outside fixed contract")
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDockerResponse+1))
	closeErr := resp.Body.Close()
	if err != nil || closeErr != nil {
		return nil, errors.Join(err, closeErr)
	}
	if len(data) > maxDockerResponse {
		return nil, errors.New("Docker response exceeds byte ceiling")
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))
	return resp, nil
}

func socketDialer(socket string) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		before, err := os.Lstat(socket)
		if err != nil || before.Mode()&os.ModeSocket == 0 || before.Mode().Perm()&0007 != 0 {
			return nil, errors.New("Docker socket unavailable or unsafe")
		}
		if !socketOwnedByRoot(before) {
			return nil, errors.New("Docker socket is not root owned")
		}
		conn, err := (&net.Dialer{}).DialContext(ctx, "unix", socket)
		if err != nil {
			return nil, err
		}
		after, err := os.Lstat(socket)
		if err != nil || !os.SameFile(before, after) {
			conn.Close()
			return nil, errors.New("Docker socket changed during connection")
		}
		return conn, nil
	}
}

func newDockerClient(socket string, write bool) (dockerClient, error) {
	if socket == "" || !strings.HasPrefix(socket, "/") {
		return dockerClient{}, errors.New("absolute local Docker socket required")
	}
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.DialContext = socketDialer(socket)
	base.Proxy = nil
	base.DisableKeepAlives = true // revalidate the socket on every request
	base.TLSClientConfig = nil
	base.MaxResponseHeaderBytes = 32 << 10
	base.MaxConnsPerHost = 8
	timeout := 10 * time.Second
	if write {
		timeout = 30 * time.Second
	}
	httpClient := &http.Client{Transport: cappedTransport{base: base, write: write}, Timeout: timeout}
	api, err := client.New(client.WithHost("unix://"+socket), client.WithHTTPClient(httpClient))
	if err != nil {
		return dockerClient{}, err
	}
	return dockerClient{api: api}, nil
}

func NewReader(socket string) (Reader, error) { return newDockerClient(socket, false) }

func NewRestarter(socket string) (Restarter, error) { return newDockerClient(socket, true) }

func (d dockerClient) Engine(ctx context.Context) (Engine, error) {
	version, err := d.api.ServerVersion(ctx, client.ServerVersionOptions{})
	if err != nil {
		return Engine{}, fmt.Errorf("Docker version unavailable: %w", err)
	}
	return Engine{ObservedAt: time.Now().UTC(), Version: version.Version, API: version.APIVersion, OS: version.Os, Arch: version.Arch}, nil
}

func (d dockerClient) Containers(ctx context.Context) (Inventory, error) {
	list, err := d.api.ContainerList(ctx, client.ContainerListOptions{All: true, Limit: 501})
	if err != nil {
		return Inventory{}, fmt.Errorf("Docker inventory unavailable: %w", err)
	}
	result := Inventory{ObservedAt: time.Now().UTC(), Containers: make([]Container, 0, min(len(list.Items), 500)), Limit: 500}
	for _, item := range list.Items[:min(len(list.Items), 500)] {
		result.Containers = append(result.Containers, Container{ObservedAt: result.ObservedAt, ID: item.ID, Names: item.Names, Image: item.Image, State: string(item.State), Status: item.Status})
	}
	return result, nil
}

func (d dockerClient) Container(ctx context.Context, id string) (Container, error) {
	if !validID(id) {
		return Container{}, errors.New("full container ID required")
	}
	result, err := d.api.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return Container{}, fmt.Errorf("Docker container unavailable: %w", err)
	}
	if result.Container.ID == "" || !strings.HasPrefix(result.Container.ID, id) {
		return Container{}, errors.New("Docker returned a different container")
	}
	state := ""
	status := ""
	if result.Container.State != nil {
		state = string(result.Container.State.Status)
		status = state
	}
	image := ""
	if result.Container.Config != nil {
		image = result.Container.Config.Image
	}
	return Container{ObservedAt: time.Now().UTC(), ID: result.Container.ID, Names: []string{result.Container.Name}, Image: image, State: state, Status: status}, nil
}

func (d dockerClient) Restart(ctx context.Context, id string) (RestartOutcome, error) {
	if !validID(id) {
		return RestartOutcome{}, errors.New("stable container ID required")
	}
	before, err := d.Container(ctx, id)
	if err != nil {
		return RestartOutcome{}, err
	}
	if before.ID != id {
		return RestartOutcome{}, errors.New("restart requires the full stable container ID")
	}
	seconds := 10
	if _, err := d.api.ContainerRestart(ctx, id, client.ContainerRestartOptions{Timeout: &seconds}); err != nil {
		return RestartOutcome{Issue: "restart_outcome_unknown"}, nil
	}
	after, err := d.Container(ctx, id)
	if err != nil {
		return RestartOutcome{Invoked: known(true), Completed: known(true), Issue: "post_observation_unavailable"}, nil
	}
	return RestartOutcome{Container: &after, Invoked: known(true), Completed: known(true)}, nil
}
