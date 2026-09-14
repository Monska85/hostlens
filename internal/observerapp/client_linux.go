package observerapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"syscall"

	"github.com/Monska85/hostlens/internal/dockerobs"
)

var (
	errEngineNotFound    = errors.New("engine resource not found")
	errEngineUnsupported = errors.New("engine operation unsupported by this daemon")
	errCeiling           = errors.New("observation exceeds response ceiling")
)

// syscallStat aliases the Linux stat structure used for ownership checks.
type syscallStat = syscall.Stat_t

func ensureObject(dec *json.Decoder) error {
	var extra json.RawMessage
	if err := dec.Decode(&extra); err == nil {
		return errors.New("engine response carried multiple documents")
	}
	return nil
}

// client performs fixed GET observations against one system-wide engine.
// The upstream endpoint table stays private to this package; no generic
// request helper is exposed to any caller.
type client struct {
	socket     string
	groupGID   int
	httpClient *http.Client
	// validate is the per-connection socket check. It defaults to
	// validateEngineSocket and is injectable for disposable tests.
	validate func(path string, groupGID int) error
	// negotiateMu guards lazy version negotiation; concurrent cold-start
	// observations serialize, and a failed probe retries on the next call.
	negotiateMu sync.Mutex
	// negotiated, minAPI and serverVer are fixed after the first successful
	// version probe. They are operational metadata, never retained evidence.
	negotiated string
	minAPI     string
	serverVer  string
}

func newClient(socket string, groupGID int) *client {
	c := &client{socket: socket, groupGID: groupGID, validate: validateEngineSocket}
	c.httpClient = &http.Client{Transport: &http.Transport{DialContext: c.dial, DisableKeepAlives: true}}
	return c
}

func (c *client) close() { c.httpClient.CloseIdleConnections() }

// dial validates the configured Docker socket before every connection: it
// must be a local filesystem socket owned by root with the configured
// group, without world write access. Revalidation per connection means a
// daemon restart or reconfiguration cannot silently change authority.
func (c *client) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	if e := c.validate(c.socket, c.groupGID); e != nil {
		return nil, e
	}
	var d net.Dialer
	return d.DialContext(ctx, "unix", c.socket)
}

// validateEngineSocket checks the socket entry itself. A rootful system-wide
// engine socket is root-owned; anything else is an unsupported mode.
func validateEngineSocket(path string, groupGID int) error {
	st, e := os.Lstat(path)
	if e != nil {
		return fmt.Errorf("docker socket unavailable: %w", e)
	}
	if st.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("docker daemon socket %q is not a Unix socket", path)
	}
	raw, ok := st.Sys().(*syscallStat)
	if !ok {
		return errors.New("docker socket ownership cannot be verified")
	}
	if raw.Uid != 0 {
		return errors.New("docker daemon socket must be owned by root (rootful system-wide engine)")
	}
	if int(raw.Gid) != groupGID {
		return errors.New("docker daemon socket group does not match the configured access group")
	}
	mode := st.Mode().Perm()
	if mode&0o060 == 0 {
		return errors.New("docker daemon socket denies group access")
	}
	if mode&0o007 != 0 {
		return errors.New("docker daemon socket grants world access; refusing unsafe permissions")
	}
	return nil
}

// fixedQuery builds the immutable daemon query for one typed observation.
// Only the keys below ever reach the daemon; follow mode and arbitrary
// filters have no representation.
func (c *client) fixedQuery(r dockerobs.Request) url.Values {
	q := url.Values{}
	switch r.Operation {
	case dockerobs.OpContainerList:
		q.Set("all", "1")
	case dockerobs.OpContainerStats:
		q.Set("stream", "false")
	case dockerobs.OpContainerLogs:
		q.Set("stdout", "1")
		q.Set("stderr", "1")
		q.Set("timestamps", "1")
		if !r.Logs.Since.IsZero() {
			q.Set("since", strconv.FormatInt(r.Logs.Since.Unix(), 10))
		}
		if !r.Logs.Until.IsZero() {
			q.Set("until", strconv.FormatInt(r.Logs.Until.Unix(), 10))
		}
		if r.Logs.Records > 0 {
			q.Set("tail", strconv.Itoa(r.Logs.Records))
		}
	case dockerobs.OpImageList:
		if v, e := dockerobs.APIVersion(c.negotiated); e == nil && v >= 142 {
			q.Set("shared-size", "1")
		}
	}
	return q
}

// upstreamPath returns the fixed endpoint path for one typed observation.
// Item operations address full stable identities only.
func upstreamPath(r dockerobs.Request) (string, error) {
	switch r.Operation {
	case dockerobs.OpEngineInfo:
		return "/info", nil
	case dockerobs.OpContainerList:
		return "/containers/json", nil
	case dockerobs.OpContainerDetail:
		return "/containers/" + r.Selector + "/json", nil
	case dockerobs.OpContainerStats:
		return "/containers/" + r.Selector + "/stats", nil
	case dockerobs.OpContainerLogs:
		return "/containers/" + r.Selector + "/logs", nil
	case dockerobs.OpImageList:
		return "/images/json", nil
	case dockerobs.OpVolumeList:
		return "/volumes", nil
	case dockerobs.OpNetworkList:
		return "/networks", nil
	case dockerobs.OpDiskUsage:
		return "/system/df", nil
	default:
		return "", fmt.Errorf("unsupported observation %q", r.Operation)
	}
}

// getTyped issues one fixed GET observation with its per-endpoint ceiling.
func (c *client) getTyped(ctx context.Context, r dockerobs.Request) ([]byte, error) {
	path, e := upstreamPath(r)
	if e != nil {
		return nil, e
	}
	api := c.negotiated
	if api == "" {
		return nil, errors.New("engine API negotiation pending")
	}
	return c.get(ctx, "/v"+api+path, c.fixedQuery(r), responseCeiling(r.Operation))
}

// responseCeiling bounds every endpoint's upstream bytes.

func responseCeiling(operation string) int64 {
	switch operation {
	case dockerobs.OpEngineInfo:
		return dockerobs.MaxInfoBytes
	case dockerobs.OpContainerDetail:
		return dockerobs.MaxInspectBytes
	case dockerobs.OpContainerStats:
		return dockerobs.MaxStatsBytes
	case dockerobs.OpContainerLogs:
		return dockerobs.MaxLogBytes
	case dockerobs.OpDiskUsage:
		return dockerobs.MaxDiskUsageBytes
	default:
		return dockerobs.MaxListBytes
	}
}

// get issues one fixed GET request against the exact given path. The
// method, Host header, and headers are fixed; the caller passes the fully
// versioned endpoint path from the private observation table.
func (c *client) get(ctx context.Context, path string, query url.Values, ceiling int64) ([]byte, error) {
	u := url.URL{Scheme: "http", Host: "docker", Path: path}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	// The daemon requires a Host header; the value is fixed and carries no
	// caller-controlled data.
	req.Host = "docker"
	req.Header.Set("User-Agent", "hostlens-docker-observer/1")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, context.Cause(ctx)
		}
		return nil, fmt.Errorf("engine request failed: %w", err)
	}
	defer func() {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
	}()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, errEngineNotFound
	case http.StatusNotImplemented, http.StatusBadRequest:
		return nil, fmt.Errorf("%w: %s", errEngineUnsupported, engineErrorMessage(resp.Body))
	default:
		return nil, fmt.Errorf("engine request rejected (%d): %s", resp.StatusCode, engineErrorMessage(resp.Body))
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, ceiling+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, context.Cause(ctx)
		}
		return nil, fmt.Errorf("engine response unavailable: %w", err)
	}
	if int64(len(b)) > ceiling {
		return nil, fmt.Errorf("%w: %d bytes", errCeiling, ceiling)
	}
	return b, nil
}

func engineErrorMessage(body io.Reader) string {
	b, _ := io.ReadAll(io.LimitReader(body, 1024))
	var payload struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(b, &payload) == nil && payload.Message != "" {
		return payload.Message
	}
	return "engine response had no usable message"
}

// decodeList streams a bounded JSON array element by element so oversized
// inventories stop before they accumulate. Truncation is reported, never
// silently dropped, and trailing garbage is rejected.
func decodeList[T any](data []byte, maxItems int) ([]T, bool, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if _, err := dec.Token(); err != nil {
		return nil, false, fmt.Errorf("engine list malformed: %w", err)
	}
	items := make([]T, 0, min(maxItems, 64))
	truncated := false
	for dec.More() {
		if len(items) >= maxItems {
			truncated = true
			break
		}
		var item T
		if err := dec.Decode(&item); err != nil {
			return items, true, fmt.Errorf("engine list malformed: %w", err)
		}
		items = append(items, item)
	}
	if !truncated {
		if _, err := dec.Token(); err != nil {
			return items, truncated, fmt.Errorf("engine list malformed: %w", err)
		}
		if err := ensureObject(dec); err != nil {
			return items, truncated, err
		}
	}
	return items, truncated, nil
}

func decodeObject(data []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("engine response malformed: %w", err)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err == nil {
		return errors.New("engine response carried multiple documents")
	}
	return nil
}

// ensureNegotiated performs the version probe once per process under the
// negotiation mutex; a failed probe stays retriable.
func (c *client) ensureNegotiated(ctx context.Context) error {
	c.negotiateMu.Lock()
	defer c.negotiateMu.Unlock()
	if c.negotiated != "" {
		return nil
	}
	return c.negotiate(ctx)
}

// negotiate reads the daemon version and selects the highest Engine API
// version supported by both sides, refusing incompatible daemons explicitly.
func (c *client) negotiate(ctx context.Context) error {
	data, err := c.get(ctx, "/version", nil, dockerobs.MaxInfoBytes)
	if err != nil {
		return fmt.Errorf("engine version unavailable: %w", err)
	}
	var daemon struct {
		Version    string `json:"Version"`
		APIVersion string `json:"ApiVersion"`
		MinAPI     string `json:"MinAPIVersion"`
		Os         string `json:"Os"`
	}
	if err := json.Unmarshal(data, &daemon); err != nil {
		return fmt.Errorf("engine version malformed: %w", err)
	}
	if daemon.Os != "" && daemon.Os != "linux" {
		return fmt.Errorf("engine reports unsupported operating system %q", daemon.Os)
	}
	negotiated, err := dockerobs.Negotiate(daemon.APIVersion, daemon.MinAPI, dockerobs.APICeiling)
	if err != nil {
		return err
	}
	c.negotiated = negotiated
	c.minAPI = daemon.MinAPI
	c.serverVer = daemon.Version
	return nil
}

// observe performs one typed operation and projects the response into the
// shared contract. Raw daemon structures never leave this package. The
// observation budget bounds negotiation and the operation alike, so a
// stalled daemon cannot hold a worker slot past its deadline.
func (c *client) observe(ctx context.Context, r dockerobs.Request) dockerobs.Response {
	ctx, cancel := observeContext(ctx, observationBudget)
	defer cancel()
	if e := c.ensureNegotiated(ctx); e != nil {
		return fail(e)
	}
	switch r.Operation {
	case dockerobs.OpEngineInfo:
		return c.engineInfo(ctx)
	case dockerobs.OpContainerList:
		return c.containerList(ctx)
	case dockerobs.OpContainerDetail:
		return c.containerDetail(ctx, r)
	case dockerobs.OpContainerStats:
		return c.containerStats(ctx, r)
	case dockerobs.OpContainerLogs:
		return c.containerLogs(ctx, r)
	case dockerobs.OpImageList:
		return c.imageList(ctx, r)
	case dockerobs.OpVolumeList:
		return c.volumeList(ctx)
	case dockerobs.OpNetworkList:
		return c.networkList(ctx)
	case dockerobs.OpDiskUsage:
		return c.diskUsage(ctx)
	default:
		return fail(fmt.Errorf("unsupported observation %q", r.Operation))
	}
}

func fail(err error) dockerobs.Response {
	return dockerobs.Response{Failed: true, Reason: err.Error()}
}

func notFound(err error) dockerobs.Response {
	if errors.Is(err, errEngineNotFound) {
		return dockerobs.Response{Failed: true, Reason: err.Error(), Issue: "not_found"}
	}
	return fail(err)
}
