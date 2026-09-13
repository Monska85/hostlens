package linux

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/dockerobs"
)

// DockerObserver transport. The diagnostic backend reaches the isolated
// observer through this typed client only; it never receives the Docker
// socket path of the daemon, a Docker group credential, or a generic request
// helper.
type DockerObserver interface {
	Observe(ctx context.Context, request dockerobs.Request) (dockerobs.Response, error)
}

type observerClient struct {
	path   string
	client *http.Client
}

func newObserverClient(path string) DockerObserver {
	return &observerClient{
		path: path,
		client: &http.Client{
			Timeout: contract.IPCClientTimeout,
			Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", path)
			}, DisableKeepAlives: true},
		},
	}
}

func (o *observerClient) Observe(ctx context.Context, request dockerobs.Request) (dockerobs.Response, error) {
	b, e := json.Marshal(request)
	if e != nil {
		return dockerobs.Response{}, e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix/observe", bytes.NewReader(b))
	if e != nil {
		return dockerobs.Response{}, e
	}
	req.Header.Set("Content-Type", "application/json")
	resp, e := o.client.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return dockerobs.Response{}, context.Cause(ctx)
		}
		return dockerobs.Response{}, errors.New("docker observer unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return dockerobs.Response{}, fmt.Errorf("observer rejected observation (%d): %s", resp.StatusCode, string(detail))
	}
	var response dockerobs.Response
	if e := json.NewDecoder(io.LimitReader(resp.Body, dockerobs.MaxListBytes+64<<10)).Decode(&response); e != nil {
		if ctx.Err() != nil {
			return dockerobs.Response{}, context.Cause(ctx)
		}
		return dockerobs.Response{}, fmt.Errorf("observer response unavailable: %w", e)
	}
	return response, nil
}

// NewObserverClient prepares the typed observer transport for composition.
// It is only wired when the configuration enables Docker diagnostics.
func NewObserverClient(path string) DockerObserver { return newObserverClient(path) }

// dockerToolKinds maps every registered Docker tool to its policy kind.
var dockerToolKinds = map[string]string{
	"get_docker_info":            "daemon",
	"list_docker_containers":     "containers",
	"get_docker_container":       "container",
	"get_docker_container_stats": "stats",
	"list_docker_images":         "images",
	"list_docker_volumes":        "volumes",
	"list_docker_networks":       "networks",
	"get_docker_disk_usage":      "disk_usage",
	"query_docker_logs":          "logs",
}

// dockerCapabilities reports which Docker tools are supported by the live
// observer, the engine, and the active grants. Denial wins at discovery
// time, and no probe contacts the observer when no Docker class is granted.
// A stalled observer stays bounded by its probe deadline and never blocks
// unrelated discovery.
func (c *Collector) dockerCapabilities(ctx context.Context) map[string]bool {
	out := map[string]bool{}
	if !c.dockerEnabled() {
		return out
	}
	kindActive := map[string]bool{}
	for _, k := range c.Policy.ActiveKinds("docker") {
		kindActive[k] = true
	}
	if len(kindActive) == 0 {
		return out
	}
	probe, err := c.dockerProbe(ctx)
	if err != nil || probe.Engine == nil || len(probe.Engine.UnsupportedReasons) > 0 {
		return out
	}
	for tool, kind := range dockerToolKinds {
		switch kind {
		case "daemon", "containers", "images", "volumes", "networks", "disk_usage":
			// Collection forms keep deny precedence at discovery.
			out[tool] = kindActive[kind] && c.Policy.Allowed("docker", kind, false)
		default:
			out[tool] = kindActive[kind] && !c.Policy.DockerKindDenied(kind)
		}
	}
	return out
}

// dockerProbe bounds one capability probe.
func (c *Collector) dockerProbe(ctx context.Context) (dockerobs.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.Docker.Observe(ctx, dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpEngineInfo})
}

func (c *Collector) dockerEnabled() bool {
	return c.Config.Docker.Enabled && c.Docker != nil
}

func (c *Collector) dockerUnavailable(ctx context.Context, err error) contract.Result {
	r := contract.Failure("docker_unavailable")
	if err != nil && (errors.Is(err, context.Canceled) || ctx.Err() != nil) {
		r = contract.Failure("cancelled_or_timeout")
	}
	return r
}

func (c *Collector) observe(ctx context.Context, request dockerobs.Request) (dockerobs.Response, error) {
	if err := ctx.Err(); err != nil {
		return dockerobs.Response{}, err
	}
	return c.Docker.Observe(ctx, request)
}
