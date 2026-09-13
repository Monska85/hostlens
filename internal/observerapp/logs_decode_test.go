package observerapp

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
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Monska85/hostlens/internal/dockerobs"
)

// modeEngine serves fixed responses per mode on one Unix socket.
type modeEngine struct {
	listener net.Listener
	path     string
	mu       sync.Mutex
	mode     string
	server   *http.Server
}

func newModeEngine(t *testing.T) *modeEngine {
	t.Helper()
	path := filepath.Join(t.TempDir(), "docker.sock")
	l, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	m := &modeEngine{listener: l, path: path}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		m.mu.Lock()
		mode := m.mode
		m.mu.Unlock()
		if req.URL.Path == "/version" && (mode == "" || mode == "windows") {
			if mode == "windows" {
				fmt.Fprint(w, `{"Version":"28.0.0","ApiVersion":"1.56","MinAPIVersion":"1.44","Os":"windows"}`)
			} else {
				fmt.Fprint(w, `{"Version":"28.0.0","ApiVersion":"1.56","MinAPIVersion":"1.44","Os":"linux"}`)
			}
			return
		}
		switch mode {
		case "notfound":
			http.Error(w, `{"message":"no such container"}`, http.StatusNotFound)
		case "badrequest":
			http.Error(w, `{"message":"bad driver"}`, http.StatusBadRequest)
		case "unimplemented":
			http.Error(w, `{"message":"driver unsupported"}`, http.StatusNotImplemented)
		case "servererror":
			http.Error(w, `{"message":"daemon refusal"}`, http.StatusInternalServerError)
		case "plainerror":
			http.Error(w, "not json at all", http.StatusInternalServerError)
		case "oversized":
			w.Write(make([]byte, dockerobs.MaxStatsBytes+64))
		case "brokenlogs":
			w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(strings.Repeat("x", 100)))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			panic("simulated handler crash mid-stream")
		case "malformed":
			w.Write([]byte("{not json"))
		case "rootless":
			fmt.Fprint(w, `{"ServerVersion":"28.0.0","Os":"linux","OSType":"linux","OperatingSystem":"Docker Desktop","SecurityOptions":["name=rootless"],"Plugins":{"Log":["json-file"]}}`)
		case "detailports":
			fmt.Fprint(w, fmt.Sprintf(`{"Id":"%s","Created":"2026-09-12T10:00:00Z","Name":"/web","Image":"sha256:%s","Config":{"Image":"nginx:1"},"State":{"Status":"running","Pid":7,"StartedAt":"2026-09-12T10:00:01Z","FinishedAt":"0001-01-01T00:00:00Z"},"RestartCount":0,"HostConfig":{"LogConfig":{"Type":"json-file"},"RestartPolicy":{"Name":"no"},"NetworkMode":"bridge"},"NetworkSettings":{"Ports":{"notaport":[{"HostPort":"1","HostIp":"127.0.0.1"}],"80/notaproto":[{"HostPort":"2"}],"80/tcp":[{"HostPort":"notaport"},{"HostPort":"8080","HostIp":"127.0.0.1"}]},"Networks":{"bridge":{"IPAddress":"172.17.0.2","MacAddress":"02:42"}}},"Mounts":[]}`, id64(1), id64(1)))
		case "manyvolumes":
			var entries []string
			for i := range dockerobs.MaxVolumeItems + 1 {
				entries = append(entries, fmt.Sprintf(`{"Name":"vol-%d","Driver":"local"}`, i))
			}
			fmt.Fprintf(w, `{"Volumes":[%s]}`, strings.Join(entries, ","))
		default:
			fmt.Fprint(w, `[]`)
		}
	})
	m.server = &http.Server{Handler: mux}
	go func() { _ = m.server.Serve(l) }()
	t.Cleanup(func() { _ = m.server.Close() })
	return m
}

func (m *modeEngine) set(mode string) {
	m.mu.Lock()
	m.mode = mode
	m.mu.Unlock()
}

func newTestClient(t *testing.T, engine *modeEngine) *client {
	t.Helper()
	c := newClient(engine.path, 0)
	c.validate = allowSocketType
	if err := c.negotiate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestLogsItemErrorStatuses(t *testing.T) {
	t.Parallel()

	engine := newModeEngine(t)
	c := newTestClient(t, engine)
	request := dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerLogs, Selector: id64(7), Logs: dockerobs.LogOptions{Records: 5, MaxBytes: 512}}

	engine.set("notfound")
	response := c.containerLogs(context.Background(), request)
	if !response.Failed || response.Issue != "not_found" {
		t.Fatalf("404 must mark not_found: %+v", response)
	}

	engine.set("badrequest")
	response = c.containerLogs(context.Background(), request)
	if !response.Failed || response.Issue != "unsupported_driver" {
		t.Fatalf("400 must mark unsupported_driver: %+v", response)
	}

	engine.set("unimplemented")
	response = c.containerLogs(context.Background(), request)
	if !response.Failed || response.Issue != "unsupported_driver" {
		t.Fatalf("501 must mark unsupported_driver: %+v", response)
	}

	engine.set("servererror")
	response = c.containerLogs(context.Background(), request)
	if !response.Failed || !strings.Contains(response.Reason, "daemon refusal") {
		t.Fatalf("500 daemon message lost: %+v", response)
	}

	engine.set("plainerror")
	response = c.containerLogs(context.Background(), request)
	if !response.Failed || !strings.Contains(response.Reason, "no usable message") {
		t.Fatalf("500 fallback lost: %+v", response)
	}
}

func TestLogsStalledEngineReportsContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docker.sock")
	l, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	block := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"Version":"28.0.0","ApiVersion":"1.56","MinAPIVersion":"1.44","Os":"linux"}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { <-block })
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(l) }()
	t.Cleanup(func() { close(block); _ = server.Close() })

	c := newClient(path, 0)
	c.validate = allowSocketType
	if e := c.negotiate(context.Background()); e != nil {
		t.Fatal(e)
	}
	saved := observationBudget
	observationBudget = 60 * time.Millisecond
	t.Cleanup(func() { observationBudget = saved })
	response := c.containerLogs(context.Background(), dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerLogs, Selector: id64(7)})
	if !response.Failed {
		t.Fatal("stalled log observation returned evidence")
	}
}

func TestLogsNegotiationPending(t *testing.T) {
	t.Parallel()

	c := newClient(filepath.Join(t.TempDir(), "docker.sock"), 0)
	response := c.containerLogs(context.Background(), dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerLogs, Selector: id64(7)})
	if !response.Failed || !strings.Contains(response.Reason, "negotiation pending") {
		t.Fatalf("un-negotiated logs accepted: %+v", response)
	}
	if _, e := c.getTyped(context.Background(), dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerList}); e == nil || !strings.Contains(e.Error(), "negotiation pending") {
		t.Fatalf("un-negotiated getTyped accepted: %v", e)
	}
}

func TestUpstreamPathAndCeilings(t *testing.T) {
	t.Parallel()

	for op, want := range map[string]string{
		dockerobs.OpEngineInfo:      "/info",
		dockerobs.OpContainerList:   "/containers/json",
		dockerobs.OpContainerDetail: "/containers/" + id64(1) + "/json",
		dockerobs.OpContainerStats:  "/containers/" + id64(1) + "/stats",
		dockerobs.OpContainerLogs:   "/containers/" + id64(1) + "/logs",
		dockerobs.OpImageList:       "/images/json",
		dockerobs.OpVolumeList:      "/volumes",
		dockerobs.OpNetworkList:     "/networks",
		dockerobs.OpDiskUsage:       "/system/df",
	} {
		got, e := upstreamPath(dockerobs.Request{Operation: op, Selector: id64(1)})
		if e != nil || got != want {
			t.Fatalf("upstreamPath(%s) = %q, %v", op, got, e)
		}
	}
	if _, e := upstreamPath(dockerobs.Request{Operation: "unknown_op"}); e == nil {
		t.Fatal("unknown operation accepted")
	}
	for op, want := range map[string]int64{
		dockerobs.OpEngineInfo:      dockerobs.MaxInfoBytes,
		dockerobs.OpContainerDetail: dockerobs.MaxInspectBytes,
		dockerobs.OpContainerStats:  dockerobs.MaxStatsBytes,
		dockerobs.OpContainerLogs:   dockerobs.MaxLogBytes,
		dockerobs.OpDiskUsage:       dockerobs.MaxDiskUsageBytes,
		dockerobs.OpImageList:       dockerobs.MaxListBytes,
	} {
		if got := responseCeiling(op); got != want {
			t.Fatalf("ceiling(%s) = %d, want %d", op, got, want)
		}
	}
	if responseCeiling("unknown_op") != dockerobs.MaxListBytes {
		t.Fatal("default ceiling lost")
	}
	if _, e := c2GetTypedUnsupported(); e == nil {
		t.Fatal("unsupported getTyped accepted")
	}
}

// c2GetTypedUnsupported exercises the unsupported-operation rejection of
// getTyped with a negotiated client.
func c2GetTypedUnsupported() ([]byte, error) {
	c := newClient("/absent.sock", 0)
	c.negotiated = "1.51"
	return c.getTyped(context.Background(), dockerobs.Request{Version: 1, Operation: "unknown_op"})
}

func TestDecodeGuardsRejectMalformedAndExtraDocuments(t *testing.T) {
	t.Parallel()

	if _, _, e := decodeList[map[string]any]([]byte("{not json"), 10); e == nil {
		t.Fatal("malformed list accepted")
	}
	if _, _, e := decodeList[struct{ N int }]([]byte(`[{"n":1},{"n":"not-a-number"}]`), 10); e == nil {
		t.Fatal("malformed list item accepted")
	}
	if _, _, e := decodeList[map[string]any]([]byte(`[{"n":1}] {"n":2}`), 10); e == nil || !strings.Contains(e.Error(), "multiple documents") {
		t.Fatalf("extra document accepted: %v", e)
	}
	if _, _, e := decodeList[map[string]any]([]byte(`[{"n":1}`), 10); e == nil || !strings.Contains(e.Error(), "malformed") {
		t.Fatalf("unclosed array accepted: %v", e)
	}
	items, truncated, e := decodeList[map[string]any]([]byte(`[{"n":1},{"n":2},{"n":3}]`), 2)
	if e != nil || len(items) != 2 || !truncated {
		t.Fatalf("truncation lost: %d items, %v, %v", len(items), truncated, e)
	}
	empty, truncated, e := decodeList[map[string]any]([]byte(`[]`), 10)
	if e != nil || len(empty) != 0 || truncated {
		t.Fatalf("empty list mishandled: %v %v %v", empty, truncated, e)
	}

	if e := decodeObject([]byte("{bad"), &struct{}{}); e == nil || !strings.Contains(e.Error(), "malformed") {
		t.Fatalf("malformed object accepted: %v", e)
	}
	if e := decodeObject([]byte(`{"a":1}{"b":2}`), &map[string]any{}); e == nil || !strings.Contains(e.Error(), "multiple documents") {
		t.Fatalf("multiple documents accepted: %v", e)
	}
	if e := decodeObject([]byte(`{"a":1}`), &map[string]any{}); e != nil {
		t.Fatalf("single object rejected: %v", e)
	}

	dec := json.NewDecoder(strings.NewReader(`{"a":1} {"b":2}`))
	var first map[string]any
	if e := dec.Decode(&first); e != nil {
		t.Fatal(e)
	}
	if e := ensureObject(dec); e == nil {
		t.Fatal("trailing document accepted")
	}
	dec = json.NewDecoder(strings.NewReader(`{"a":1}`))
	if e := dec.Decode(&first); e != nil {
		t.Fatal(e)
	}
	if e := ensureObject(dec); e != nil {
		t.Fatalf("single document rejected: %v", e)
	}
}

func TestObserveRejectsUnsupportedOperation(t *testing.T) {
	t.Parallel()

	engine := newRecordingEngine(t)
	s := NewServer(engine.path, 0)
	defer s.Close()
	s.engine.validate = allowSocketType
	response := s.engine.observe(context.Background(), dockerobs.Request{Version: 1, Operation: "mutate_op"})
	if !response.Failed || !strings.Contains(response.Reason, "unsupported observation") {
		t.Fatalf("unsupported operation accepted: %+v", response)
	}
}

func TestRunSlotUnavailableReportsFailure(t *testing.T) {
	t.Parallel()

	engine := newRecordingEngine(t)
	s := NewServer(engine.path, 0)
	defer s.Close()
	// Fill the single disk-accounting slot; a second disk run must report
	// its own unavailability instead of queueing.
	s.diskRuns <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	response := s.run(ctx, dockerobs.Request{Version: 1, Operation: dockerobs.OpDiskUsage})
	if !response.Failed || !strings.Contains(response.Reason, "slot unavailable") {
		t.Fatalf("full slot accepted work: %+v", response)
	}
	<-s.diskRuns
}

func TestParseDockerTimeRejectsGarbage(t *testing.T) {
	t.Parallel()

	if parseDockerTime("") != nil {
		t.Fatal("empty timestamp must stay nil")
	}
	if parseDockerTime("not-a-time") != nil {
		t.Fatal("garbage timestamp must stay nil")
	}
	if parseDockerTime("2026-09-13T10:00:00Z") == nil {
		t.Fatal("valid timestamp lost")
	}
}

func frame(stream byte, payload string) []byte {
	header := make([]byte, 8)
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:8], uint32(len(payload)))
	return append(header, []byte(payload)...)
}

func concatFrames(frames ...[]byte) []byte {
	var out []byte
	for _, f := range frames {
		out = append(out, f...)
	}
	return out
}

func TestDecodeLogStreamNonTTYBranches(t *testing.T) {
	t.Parallel()

	// Complete stream: stdout, stderr, a system frame, and an untimestamped
	// content frame that still carries a readable message.
	body := concatFrames(
		frame(1, "2026-09-13T10:00:00Z first\n"),
		frame(2, "2026-09-13T10:00:01Z second\n"),
		frame(3, "ignored system frame"),
		frame(1, "no timestamp here\n"),
	)
	page, e := decodeLogStream(bytes.NewReader(body), 4096, false)
	if e != nil {
		t.Fatal(e)
	}
	if len(page.Records) != 3 || page.Skipped != 1 || page.Dropped != 0 {
		t.Fatalf("frame accounting lost: %d records, %d skipped, %d dropped", len(page.Records), page.Skipped, page.Dropped)
	}
	if page.Records[1].Stream != "stderr" || page.Records[1].Timestamp == nil {
		t.Fatalf("stderr timestamp lost: %+v", page.Records[1])
	}
	if page.Records[2].Message != "no timestamp here" {
		t.Fatalf("untimestamped frame content lost: %+v", page.Records[2])
	}
	if page.FirstEvent == nil || page.LastEvent == nil || !page.FirstEvent.Before(*page.LastEvent) {
		t.Fatalf("event window lost: %+v", page)
	}

	// Truncated header: the partial prefix must not fabricate a record.
	page, e = decodeLogStream(strings.NewReader("\x01\x00\x00"), 4096, false)
	if e != nil || len(page.Records) != 0 {
		t.Fatalf("partial header accepted: %v %+v", e, page)
	}

	// Payload shorter than the header claims: the frame is dropped quietly.
	page, e = decodeLogStream(strings.NewReader("\x01\x00\x00\x00\x00\x00\x00\x20short"), 4096, false)
	if e != nil || len(page.Records) != 0 {
		t.Fatalf("short payload fabricated a record: %v %+v", e, page)
	}

	// Oversized declared payload: report truncation instead of buffering.
	huge := make([]byte, 8)
	binary.BigEndian.PutUint32(huge[4:8], uint32(1<<20))
	page, e = decodeLogStream(strings.NewReader(string(huge)), 1024, false)
	if e != nil || !page.Truncated || len(page.Records) != 0 {
		t.Fatalf("oversized payload accepted: %v %+v", e, page)
	}

	// Ceiling reached mid-payload: truncation is explicit.
	long := strings.Repeat("a", 300)
	page, e = decodeLogStream(bytes.NewReader(append(frame(1, long), frame(1, long)...)), 320, false)
	if e != nil || !page.Truncated || len(page.Records) != 1 {
		t.Fatalf("ceiling truncation lost: %v %+v", e, page)
	}

	// Record ceiling: extra frames beyond MaxLogRecords are truncation only.
	var burst strings.Builder
	for range dockerobs.MaxLogRecords + 2 {
		burst.Write(frame(1, "2026-09-13T10:00:00Z line\n"))
	}
	page, e = decodeLogStream(strings.NewReader(burst.String()), 16<<20, false)
	if e != nil || len(page.Records) != dockerobs.MaxLogRecords || !page.Truncated {
		t.Fatalf("record ceiling lost: %d records, truncated=%v", len(page.Records), page.Truncated)
	}
}

func TestDecodeLogStreamTTYBranches(t *testing.T) {
	t.Parallel()

	body := "2026-09-13T10:00:00Z first\nnot-a-timestamp line\n2026-09-13T10:00:01Z second\n\n"
	page, e := decodeLogStream(strings.NewReader(body), 4096, true)
	if e != nil {
		t.Fatal(e)
	}
	if len(page.Records) != 3 || page.Dropped != 1 {
		t.Fatalf("tty decoding lost: %+v", page)
	}
	if page.Records[0].Stream != "stdout" || page.Records[1].Message != "not-a-timestamp line" {
		t.Fatalf("tty record content lost: %+v", page.Records)
	}

	page, e = decodeLogStream(strings.NewReader(strings.Repeat("x", 200)+"\n"), 64, true)
	if e != nil || !page.Truncated {
		t.Fatalf("tty ceiling truncation lost: %v %+v", e, page)
	}

	// An unreadable stream surfaces its error instead of pretending success.
	page, e = decodeLogStream(errReader{}, 4096, true)
	if e == nil {
		t.Fatal("reader failure swallowed")
	}
	if page == nil || page.Truncated {
		t.Fatalf("tty reader failure misclassified: %+v", page)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("device failure") }

func TestTimestampedSanitizesAndPrefixes(t *testing.T) {
	t.Parallel()

	record, ok := timestamped([]byte("2026-09-13T10:00:00.123456789Z hello\r\n"))
	if !ok || record.Stream != "stdout" || record.Message != "hello" {
		t.Fatalf("timestamped record lost: %+v %v", record, ok)
	}
	record, ok = timestamped([]byte("bare line\n"))
	if !ok || record.Timestamp != nil || record.Message != "bare line" {
		t.Fatalf("untimestamped record lost: %+v %v", record, ok)
	}
	if _, ok = timestamped([]byte("")); ok {
		t.Fatal("empty payload must not become a record")
	}
	record, ok = timestamped([]byte("\xff\xfe"))
	if !ok {
		t.Fatal("sanitized invalid-utf8 payload must still carry content")
	}
	if !strings.ContainsRune(record.Message, utf8.RuneError) {
		t.Fatalf("invalid utf8 must be sanitized: %q", record.Message)
	}
}

func TestByteBoundedProbeSemantics(t *testing.T) {
	t.Parallel()

	// Exact-ceiling response ends cleanly: the probe sees EOF, not truncation.
	exact := &byteBounded{r: strings.NewReader("abcdef"), remaining: 6}
	buf := make([]byte, 4)
	n, e := exact.Read(buf)
	if n != 4 || e != nil {
		t.Fatalf("first read = %d, %v", n, e)
	}
	n, e = exact.Read(buf)
	if n != 2 || e != nil {
		t.Fatalf("tail read = %d, %v", n, e)
	}
	if n, e = exact.Read(buf); n != 0 || e != io.EOF {
		t.Fatalf("probe read = %d, %v", n, e)
	}
	if n, e = exact.Read(buf); n != 0 || e != io.EOF {
		t.Fatalf("post-probe read = %d, %v", n, e)
	}

	// More content beyond the ceiling: the probe reports the ceiling.
	over := &byteBounded{r: strings.NewReader("abcdef!"), remaining: 6}
	if _, e = io.ReadAll(over); e != nil && !errors.Is(e, errByteCeiling) {
		t.Fatalf("ceiling read = %v", e)
	}
}

func TestProjectionHelpersEdgeInputs(t *testing.T) {
	t.Parallel()

	if unixTimePtr(0) != nil || unixTimePtr(-5) != nil {
		t.Fatal("non-positive timestamps must stay nil")
	}
	if unixTimePtr(1757700000) == nil {
		t.Fatal("positive timestamp lost")
	}
	ports := []struct {
		IP          string `json:"IP"`
		PrivatePort uint16 `json:"PrivatePort"`
		PublicPort  uint16 `json:"PublicPort"`
		Type        string `json:"Type"`
	}{{IP: "127.0.0.1", PrivatePort: 80, PublicPort: 8080, Type: "tcp"}, {PrivatePort: 53, Type: "udp"}}
	mapped := portSummary(ports)
	if len(mapped) != 2 || mapped[0].PublicPort != 8080 || mapped[1].PublicPort != 0 {
		t.Fatalf("port projection lost: %+v", mapped)
	}
	if mapped := portSummary(nil); len(mapped) != 0 {
		t.Fatal("empty ports must project to an empty slice")
	}
}

func TestObserverDetailPortParsingContinues(t *testing.T) {
	t.Parallel()

	engine := newModeEngine(t)
	c := newTestClient(t, engine)
	engine.set("detailports")
	response := c.containerDetail(context.Background(), dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerDetail, Selector: id64(1)})
	if response.Failed {
		t.Fatal(response.Reason)
	}
	if len(response.Detail.Ports) != 2 {
		t.Fatalf("malformed port bindings must be skipped: %+v", response.Detail.Ports)
	}
	var public []uint16
	for _, p := range response.Detail.Ports {
		public = append(public, p.PublicPort)
	}
	if public[0] == public[1] || (public[0] != 8080 && public[1] != 8080) {
		t.Fatalf("valid binding must survive alongside the rejected one: %+v", response.Detail.Ports)
	}
	if len(response.Detail.Endpoints) != 1 {
		t.Fatalf("endpoints lost: %+v", response.Detail.Endpoints)
	}
}

func TestEngineInfoUnsupportedModesAreReported(t *testing.T) {
	t.Parallel()

	engine := newModeEngine(t)
	c := newTestClient(t, engine)
	engine.set("rootless")
	response := c.engineInfo(context.Background())
	if response.Failed {
		t.Fatal(response.Reason)
	}
	if len(response.Engine.UnsupportedReasons) != 2 {
		t.Fatalf("rootless and desktop reasons lost: %+v", response.Engine.UnsupportedReasons)
	}
}

func TestNegotiateRefusesNonLinuxDaemons(t *testing.T) {
	t.Parallel()

	engine := newModeEngine(t)
	newTestClient(t, engine)
	engine.set("windows")
	// A fresh client repeats the probe against the switched engine.
	other := newClient(engine.path, 0)
	other.validate = allowSocketType
	if e := other.negotiate(context.Background()); e == nil || !strings.Contains(e.Error(), "unsupported operating system") {
		t.Fatalf("windows daemon accepted: %v", e)
	}
}

func TestEngineRefusalsMarkEveryOperation(t *testing.T) {
	t.Parallel()

	engine := newModeEngine(t)
	c := newTestClient(t, engine)
	ops := map[string]func() dockerobs.Response{
		dockerobs.OpContainerList: func() dockerobs.Response { return c.containerList(context.Background()) },
		dockerobs.OpContainerDetail: func() dockerobs.Response {
			return c.containerDetail(context.Background(), dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerDetail, Selector: id64(1)})
		},
		dockerobs.OpContainerStats: func() dockerobs.Response {
			return c.containerStats(context.Background(), dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerStats, Selector: id64(1)})
		},
		dockerobs.OpImageList: func() dockerobs.Response {
			return c.imageList(context.Background(), dockerobs.Request{Version: 1, Operation: dockerobs.OpImageList})
		},
		dockerobs.OpVolumeList:  func() dockerobs.Response { return c.volumeList(context.Background()) },
		dockerobs.OpNetworkList: func() dockerobs.Response { return c.networkList(context.Background()) },
		dockerobs.OpDiskUsage:   func() dockerobs.Response { return c.diskUsage(context.Background()) },
		dockerobs.OpEngineInfo:  func() dockerobs.Response { return c.engineInfo(context.Background()) },
	}
	for name, run := range ops {
		engine.set("notfound")
		response := run()
		if !response.Failed {
			t.Fatalf("%s: 404 must fail the observation: %+v", name, response)
		}
		if name != dockerobs.OpEngineInfo && response.Issue != "not_found" {
			t.Fatalf("%s: 404 must stay a bounded gap: %+v", name, response)
		}
		engine.set("malformed")
		if response := run(); !response.Failed || !strings.Contains(response.Reason, "malformed") {
			t.Fatalf("%s: malformed daemon payload accepted: %+v", name, response)
		}
		engine.set("servererror")
		if response := run(); !response.Failed || !strings.Contains(response.Reason, "500") {
			t.Fatalf("%s: 500 rejection lost: %+v", name, response)
		}
	}
}

func TestVolumeListReportsTruncation(t *testing.T) {
	t.Parallel()

	engine := newModeEngine(t)
	c := newTestClient(t, engine)
	engine.set("manyvolumes")
	response := c.volumeList(context.Background())
	if response.Failed {
		t.Fatal(response.Reason)
	}
	if len(response.Volumes) != dockerobs.MaxVolumeItems || !response.Truncated {
		t.Fatalf("volume truncation lost: %d volumes, truncated=%v", len(response.Volumes), response.Truncated)
	}
}

func TestDecodeListFirstTokenFailure(t *testing.T) {
	t.Parallel()

	if _, _, e := decodeList[struct{ N int }]([]byte("nope"), 10); e == nil || !strings.Contains(e.Error(), "malformed") {
		t.Fatalf("garbage stream accepted: %v", e)
	}
}

func TestAcceptSurfacesListenerErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	l, e := net.Listen("unix", filepath.Join(dir, "o.sock"))
	if e != nil {
		t.Fatal(e)
	}
	rejecting := checkedListener{l, 1}
	errCh := make(chan error, 1)
	go func() {
		_, e := rejecting.Accept()
		errCh <- e
	}()
	l.Close()
	select {
	case e := <-errCh:
		if e == nil {
			t.Fatal("closed listener accepted a connection")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listener close did not surface")
	}
}

func TestLogsMalformedStreamFailsClosed(t *testing.T) {
	t.Parallel()

	engine := newModeEngine(t)
	c := newTestClient(t, engine)
	engine.set("brokenlogs")
	response := c.containerLogs(context.Background(), dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerLogs, Selector: id64(7), Logs: dockerobs.LogOptions{MaxBytes: 512}})
	if !response.Failed || !strings.Contains(response.Reason, "log stream unavailable") {
		t.Fatalf("broken stream accepted: %+v", response)
	}
}
