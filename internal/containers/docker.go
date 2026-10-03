package containers

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	dockertypes "github.com/moby/moby/api/types/container"
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
	Health     string    `json:"health,omitempty"`
	Restarts   *int      `json:"restarts,omitempty"`
	OOMKilled  *bool     `json:"oom_killed,omitempty"`
}

type Stats struct {
	ObservedAt       time.Time `json:"observed_at"`
	ID               string    `json:"id"`
	CPUUsageNanos    *uint64   `json:"cpu_usage_nanos,omitempty"`
	MemoryUsageBytes *uint64   `json:"memory_usage_bytes,omitempty"`
	MemoryLimitBytes *uint64   `json:"memory_limit_bytes,omitempty"`
	PIDs             *uint64   `json:"pids,omitempty"`
}

type LogChunk struct {
	Stream string `json:"stream"`
	Text   string `json:"text"`
}

type Logs struct {
	ObservedAt time.Time  `json:"observed_at"`
	ID         string     `json:"id"`
	Chunks     []LogChunk `json:"chunks"`
	Limit      int        `json:"limit"`
	Truncated  bool       `json:"truncated,omitempty"`
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
	Stats(context.Context, string) (Stats, error)
	Logs(context.Context, string, int) (Logs, error)
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
		return len(parts) == 3 && parts[0] == "containers" && validID(parts[1]) && (parts[2] == "json" || parts[2] == "stats" || parts[2] == "logs")
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
	limit := maxDockerResponse
	if strings.HasSuffix(req.URL.Path, "/logs") {
		limit = 512 << 10
	}
	if strings.HasSuffix(req.URL.Path, "/stats") {
		limit = 1 << 20
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(limit)+1))
	closeErr := resp.Body.Close()
	if err != nil || closeErr != nil {
		return nil, errors.Join(err, closeErr)
	}
	if len(data) > limit {
		if !strings.HasSuffix(req.URL.Path, "/logs") {
			return nil, errors.New("Docker response exceeds byte ceiling")
		}
		resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(data[:limit]), limitReader{}))
		return resp, nil
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
	if result.Container.ID != id {
		return Container{}, errors.New("Docker returned a different container")
	}
	state := ""
	status := ""
	health := ""
	var oomKilled *bool
	if result.Container.State != nil {
		state = string(result.Container.State.Status)
		status = state
		oomKilled = &result.Container.State.OOMKilled
		if result.Container.State.Health != nil {
			health = string(result.Container.State.Health.Status)
		}
	}
	image := ""
	if result.Container.Config != nil {
		image = result.Container.Config.Image
	}
	return Container{ObservedAt: time.Now().UTC(), ID: result.Container.ID, Names: []string{result.Container.Name}, Image: image, State: state, Status: status, Health: health, Restarts: &result.Container.RestartCount, OOMKilled: oomKilled}, nil
}

func (d dockerClient) Stats(ctx context.Context, id string) (Stats, error) {
	if !validID(id) {
		return Stats{}, errors.New("full container ID required")
	}
	response, err := d.api.ContainerStats(ctx, id, client.ContainerStatsOptions{Stream: false})
	if err != nil {
		return Stats{}, fmt.Errorf("Docker stats unavailable: %w", err)
	}
	defer response.Body.Close()
	var sample dockertypes.StatsResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, maxDockerResponse+1)).Decode(&sample); err != nil {
		return Stats{}, fmt.Errorf("Docker stats invalid: %w", err)
	}
	if sample.ID != id {
		return Stats{}, errors.New("Docker stats returned a different container")
	}
	if sample.Read.IsZero() {
		return Stats{}, errors.New("Docker stats omitted observation time")
	}
	return Stats{ObservedAt: sample.Read, ID: id, CPUUsageNanos: &sample.CPUStats.CPUUsage.TotalUsage, MemoryUsageBytes: &sample.MemoryStats.Usage, MemoryLimitBytes: &sample.MemoryStats.Limit, PIDs: &sample.PidsStats.Current}, nil
}

var errLogLimit = errors.New("log output ceiling reached")

type limitReader struct{}

func (limitReader) Read([]byte) (int, error) { return 0, errLogLimit }

type logCollector struct {
	chunks []LogChunk
	used   int
}

type logWriter struct {
	collector *logCollector
	stream    string
}

func completeDockerFrames(raw []byte) (int, bool) {
	for at := 0; at < len(raw); {
		if len(raw)-at < 8 {
			return at, false
		}
		size := binary.BigEndian.Uint32(raw[at+4 : at+8])
		if uint64(size) > uint64(len(raw)-at-8) {
			return at, false
		}
		at += 8 + int(size)
		if at == len(raw) {
			return at, true
		}
	}
	return 0, true
}

func validDockerFrames(raw []byte) bool {
	_, complete := completeDockerFrames(raw)
	return complete
}

func (w logWriter) Write(data []byte) (int, error) {
	remaining := (64 << 10) - w.collector.used
	if remaining <= 0 || len(w.collector.chunks) >= 200 {
		return 0, errLogLimit
	}
	n := min(len(data), remaining)
	w.collector.chunks = append(w.collector.chunks, LogChunk{Stream: w.stream, Text: string(data[:n])})
	w.collector.used += n
	if n < len(data) {
		return n, errLogLimit
	}
	return n, nil
}

func keepLatestLines(chunks []LogChunk, limit int) ([]LogChunk, bool) {
	lines := 0
	for _, chunk := range chunks {
		lines += strings.Count(chunk.Text, "\n")
	}
	if len(chunks) > 0 && !strings.HasSuffix(chunks[len(chunks)-1].Text, "\n") {
		lines++
	}
	if lines <= limit {
		return chunks, false
	}
	drop := lines - limit
	kept := make([]LogChunk, 0, len(chunks))
	for _, chunk := range chunks {
		for drop > 0 && chunk.Text != "" {
			at := strings.IndexByte(chunk.Text, '\n')
			if at < 0 {
				chunk.Text = ""
				break
			}
			chunk.Text = chunk.Text[at+1:]
			drop--
		}
		if chunk.Text != "" {
			kept = append(kept, chunk)
		}
	}
	return kept, true
}

func (d dockerClient) Logs(ctx context.Context, id string, limit int) (Logs, error) {
	if !validID(id) || limit < 1 || limit > 100 {
		return Logs{}, errors.New("full container ID and log limit 1 to 100 required")
	}
	inspect, err := d.api.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil || inspect.Container.ID != id {
		return Logs{}, errors.New("Docker container unavailable or changed")
	}
	now := time.Now().UTC()
	response, err := d.api.ContainerLogs(ctx, id, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Since: now.Add(-15 * time.Minute).Format(time.RFC3339Nano), Until: now.Format(time.RFC3339Nano), Timestamps: true, Tail: strconv.Itoa(limit + 1)})
	if err != nil {
		return Logs{}, fmt.Errorf("Docker logs unavailable: %w", err)
	}
	defer response.Close()
	raw, err := io.ReadAll(io.LimitReader(response, (512<<10)+1))
	transportTruncated := errors.Is(err, errLogLimit)
	if err != nil && !transportTruncated || len(raw) > 512<<10 {
		return Logs{}, errors.New("Docker logs exceed byte ceiling")
	}
	collector := &logCollector{chunks: make([]LogChunk, 0, limit)}
	stdout, stderr := logWriter{collector, "stdout"}, logWriter{collector, "stderr"}
	if inspect.Container.Config != nil && inspect.Container.Config.Tty {
		_, err = io.Copy(stdout, bytes.NewReader(raw))
	} else {
		complete, valid := completeDockerFrames(raw)
		if !valid && !transportTruncated {
			return Logs{}, errors.New("invalid Docker log frame")
		}
		if !valid {
			raw = raw[:complete]
		}
		_, err = stdcopy.StdCopy(stdout, stderr, bytes.NewReader(raw))
	}
	if err != nil && !errors.Is(err, errLogLimit) {
		return Logs{}, errors.New("Docker logs invalid")
	}
	chunks, countTruncated := keepLatestLines(collector.chunks, limit)
	return Logs{ObservedAt: now, ID: id, Chunks: chunks, Limit: limit, Truncated: transportTruncated || errors.Is(err, errLogLimit) || countTruncated}, nil
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
