package observerapp

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/dockerobs"
)

// recordingEngine is a fake system-wide engine. It records every request and
// rejects any method, path, or query outside the read allowlist.
type recordingEngine struct {
	listener net.Listener
	path     string
	mu       sync.Mutex
	requests []string
	server   *http.Server
}

func newRecordingEngine(t *testing.T) *recordingEngine {
	t.Helper()
	path := filepath.Join(t.TempDir(), "docker.sock")
	l, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	r := &recordingEngine{listener: l, path: path}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.requests = append(r.requests, req.Method+" "+req.URL.Path+"?"+req.URL.RawQuery)
		r.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if req.Method != http.MethodGet {
			http.Error(w, "only GET allowed", http.StatusMethodNotAllowed)
			return
		}
		if req.Host != "docker" {
			http.Error(w, "unexpected host header", http.StatusBadRequest)
			return
		}
		switch req.URL.Path {
		case "/version":
			fmt.Fprint(w, `{"Version":"28.0.0","ApiVersion":"1.56","MinAPIVersion":"1.44","Os":"linux"}`)
		case "/v1.51/version":
			fmt.Fprint(w, `{"Version":"28.0.0","ApiVersion":"1.56","MinAPIVersion":"1.44","Os":"linux"}`)
		case "/v1.51/info":
			fmt.Fprint(w, `{"ServerVersion":"28.0.0","Os":"linux","OSType":"linux","Architecture":"amd64","KernelVersion":"6.1","Driver":"overlay2","CgroupDriver":"systemd","CgroupVersion":"2","LoggingDriver":"json-file","Containers":2,"ContainersRunning":1,"ContainersStopped":1,"Images":3,"OperatingSystem":"TestOS","SecurityOptions":[],"Plugins":{"Log":["json-file","none"]}}`)
		case "/v1.51/containers/json":
			fmt.Fprint(w, `[{"Id":"`+id64(1)+`","Names":["/web"],"Image":"nginx:1","ImageID":"sha256:`+id64(1)+`","State":"running","Status":"Up 1 minute","Created":1757700000,"Ports":[],"Mounts":[{"Type":"volume","Name":"data","Destination":"/data","RW":true}],"NetworkSettings":{"Networks":{"bridge":{"Name":"bridge"}}},"Env":["SECRET=x"],"Config":{"Env":["SECRET=x"],"Labels":{"leak":"me"}},"futureField":true}]`)
		case "/v1.51/volumes":
			fmt.Fprint(w, `{"Volumes":[{"Name":"data","Driver":"local","Scope":"local","CreatedAt":"2026-09-12T10:00:00Z","Labels":{}}]}`)
		case "/v1.51/networks":
			fmt.Fprint(w, `[{"Id":"`+id64(2)+`","Name":"bridge","Driver":"bridge","Scope":"local","Internal":false,"EnableIPv6":false,"IPAM":{"Config":[{"Subnet":"172.17.0.0/16"}]},"Labels":{}}]`)
		case "/v1.51/images/json":
			fmt.Fprint(w, `[{"Id":"sha256:`+id64(1)+`","RepoTags":["nginx:1"],"RepoDigests":["nginx@sha256:aaa"],"Created":1757700000,"Size":100,"SharedSize":40,"Containers":1}]`)
		case "/v1.51/system/df":
			fmt.Fprint(w, `{"LayersSize":1000,"Images":[{"Id":"sha256:`+id64(1)+`","RepoTags":["nginx:1"],"Created":1757700000,"Size":100,"SharedSize":40}],"Containers":[{"Id":"`+id64(1)+`","SizeRootFs":10,"SizeRw":1}],"Volumes":[{"Name":"data","Driver":"local","UsageData":{"RefCount":1,"Size":20}}],"BuildCache":[{"Size":5},{"Size":6}]}`)
		case "/v1.51/containers/" + id64(1) + "/json":
			fmt.Fprint(w, `{"Id":"`+id64(1)+`","Created":"2026-09-12T10:00:00Z","Name":"/web","Image":"sha256:`+id64(1)+`","Config":{"Image":"nginx:1","Healthcheck":{"Test":["CMD","curl","-f","http://localhost"]},"Env":["SECRET=leak"],"Cmd":["nginx"],"Labels":{"topsecret":"value"}},"State":{"Status":"running","OOMKilled":false,"Pid":42,"StartedAt":"2026-09-12T10:00:01Z","FinishedAt":"0001-01-01T00:00:00Z","Health":{"Status":"healthy","FailingStreak":0,"Log":[{"Start":"2026-09-12T10:00:00Z","End":"2026-09-12T10:00:01Z","ExitCode":0,"Output":"health command output"}]}},"RestartCount":2,"HostConfig":{"LogConfig":{"Type":"json-file","Config":{"max-size":"10m"}},"RestartPolicy":{"Name":"on-failure"},"NetworkMode":"bridge","PidsLimit":100,"NanoCpus":1000000000,"CpuShares":512,"Memory":536870912,"MemorySwap":-1},"NetworkSettings":{"Ports":{"80/tcp":[{"HostIp":"127.0.0.1","HostPort":"8080"}]},"Networks":{"bridge":{"IPAddress":"172.17.0.2","MacAddress":"02:42:ac:11:00:02"}}},"Mounts":[]}`)
		case "/v1.51/containers/" + id64(1) + "/stats":
			fmt.Fprint(w, `{"read":"2026-09-12T10:00:02Z","preread":"2026-09-12T09:59:59Z","cpu_stats":{"cpu_usage":{"total_usage":2000000000},"system_cpu_usage":10000000000,"online_cpus":2},"precpu_stats":{"cpu_usage":{"total_usage":1000000000},"system_cpu_usage":9000000000},"memory_stats":{"usage":1000,"limit":2000,"stats":{"inactive_file":100}},"blkio_stats":{"io_service_bytes_recursive":[{"op":"Read","value":5},{"op":"Write","value":7}]},"networks":{"eth0":{"rx_bytes":10,"tx_bytes":20}},"pids_stats":{"current":3,"limit":100}}`)
		case "/v1.51/containers/" + id64(1) + "/logs":
			w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
			for _, frame := range []struct {
				stream byte
				text   string
			}{
				{1, "2026-09-12T10:00:00.000000000Z first stdout\n"},
				{2, "2026-09-12T10:00:01.000000000Z second stderr\n"},
				{3, "system frame skipped\n"},
			} {
				payload := []byte(frame.text)
				header := make([]byte, 8)
				header[0] = frame.stream
				binary.BigEndian.PutUint32(header[4:8], uint32(len(payload)))
				_, _ = w.Write(header)
				_, _ = w.Write(payload)
			}
		default:
			http.Error(w, "unexpected path", http.StatusForbidden)
		}
	})
	r.server = &http.Server{Handler: mux}
	go func() { _ = r.server.Serve(l) }()
	t.Cleanup(func() { _ = r.server.Close() })
	return r
}

func id64(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, 64)
	for i := range b {
		b[i] = hex[(n+i)%16]
	}
	return string(b)
}

// snapshot returns the recorded request transcript.
func (r *recordingEngine) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.requests...)
}

// expect verifies the exact upstream request sequence.
func (r *recordingEngine) expect(t *testing.T, allowlist []string) {
	t.Helper()
	requests := r.snapshot()
	if len(requests) != len(allowlist) {
		t.Fatalf("upstream requests %v exceed the allowlist %v", requests, allowlist)
	}
	for i, want := range allowlist {
		if requests[i] != want {
			t.Fatalf("request %d = %q, want %q", i, requests[i], want)
		}
	}
}

func TestObserverRegistryUsesOnlyReadAllowlist(t *testing.T) {
	engine := newRecordingEngine(t)
	s := NewServer(engine.path, 0)
	defer s.Close()
	// Disposable tests replace the root-ownership requirement with a
	// socket-type check; production keeps validateEngineSocket.
	c := s.engine
	c.validate = func(path string, groupGID int) error {
		st, e := os.Lstat(path)
		if e != nil || st.Mode()&os.ModeSocket == 0 {
			return errors.New("engine socket is not a Unix socket")
		}
		return nil
	}
	id := id64(1)
	for _, operation := range []string{
		dockerobs.OpEngineInfo, dockerobs.OpContainerList, dockerobs.OpContainerDetail,
		dockerobs.OpContainerStats, dockerobs.OpContainerLogs, dockerobs.OpImageList,
		dockerobs.OpVolumeList, dockerobs.OpNetworkList, dockerobs.OpDiskUsage,
	} {
		engine.mu.Lock()
		engine.requests = nil
		engine.mu.Unlock()
		response := s.run(context.Background(), dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: operation, Selector: id, Logs: dockerobs.LogOptions{Records: 2, MaxBytes: 4096}})
		if response.Failed {
			t.Fatalf("%s failed: %s", operation, response.Reason)
		}
		for _, request := range engine.snapshot() {
			if request == "GET /version?" {
				continue // one-time negotiation probe
			}
			if !strings.HasPrefix(request, "GET /v1.51/") {
				t.Fatalf("non-GET or unversioned upstream request: %s", request)
			}
			if strings.Contains(request, "follow=") {
				t.Fatalf("log observation requested follow mode: %s", request)
			}
		}
	}
	// One disk usage observation is a single daemon accounting request;
	// correlation happens in the diagnostic backend.
	engine.mu.Lock()
	engine.requests = nil
	engine.mu.Unlock()
	response := s.run(context.Background(), dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpDiskUsage})
	if response.Failed {
		t.Fatal(response.Reason)
	}
	engine.expect(t, []string{"GET /v1.51/system/df?"})
}

func TestObserverProjectionsExcludeSecrets(t *testing.T) {
	engine := newRecordingEngine(t)
	s := NewServer(engine.path, 0)
	defer s.Close()
	s.engine.validate = func(path string, _ int) error {
		st, e := os.Lstat(path)
		if e != nil || st.Mode()&os.ModeSocket == 0 {
			return errors.New("engine socket is not a Unix socket")
		}
		return nil
	}
	response := s.run(context.Background(), dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpContainerList})
	if response.Failed {
		t.Fatal(response.Reason)
	}
	b, e := json.Marshal(response)
	if e != nil {
		t.Fatal(e)
	}
	for _, secret := range []string{"SECRET", "leak", "futureField", "Labels"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("secret-bearing metadata crossed the boundary: %s", secret)
		}
	}
	detail := s.run(context.Background(), dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpContainerDetail, Selector: id64(1)})
	if detail.Failed {
		t.Fatal(detail.Reason)
	}
	b, _ = json.Marshal(detail)
	for _, secret := range []string{"SECRET", "health command output", "max-size", "topsecret", "curl"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("detail projection leaked %q", secret)
		}
	}
	if detail.Detail.HealthCheck != "configured" {
		t.Fatal("health check fact missing")
	}
	stats := s.run(context.Background(), dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpContainerStats, Selector: id64(1)})
	if stats.Failed || stats.Stats == nil {
		t.Fatal(stats.Reason)
	}
	if stats.Stats.CPUPercent == nil {
		t.Fatal("paired CPU sample must supply a rate")
	}
	logs := s.run(context.Background(), dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpContainerLogs, Selector: id64(1), Logs: dockerobs.LogOptions{Records: 10, MaxBytes: 4096}})
	if logs.Failed || logs.Logs == nil {
		t.Fatal(logs.Reason)
	}
	if len(logs.Logs.Records) != 2 || logs.Logs.Skipped != 1 {
		t.Fatalf("decoded records %d skipped %d", len(logs.Logs.Records), logs.Logs.Skipped)
	}
	if logs.Logs.Records[0].Stream != "stdout" || logs.Logs.Records[1].Stream != "stderr" {
		t.Fatal("stream meaning lost")
	}
}

func TestObserverRejectsInvalidTypedRequests(t *testing.T) {
	engine := newRecordingEngine(t)
	s := NewServer(engine.path, 0)
	defer s.Close()
	rejected := []dockerobs.Request{
		{Version: 2, Operation: dockerobs.OpEngineInfo},
		{Version: dockerobs.ProtocolVersion, Operation: "mutate_something"},
		{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpContainerDetail, Selector: "web"},
		{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpContainerStats},
		{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpContainerLogs, Logs: dockerobs.LogOptions{MaxBytes: dockerobs.MaxLogBytes + 1}},
	}
	for _, request := range rejected {
		if e := request.Validate(); e == nil {
			t.Fatalf("invalid typed request accepted: %+v", request)
		}
	}
}

func TestObserverVersionFloorRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docker.sock")
	l, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"Version":"19.03","ApiVersion":"1.40","MinAPIVersion":"1.40","Os":"linux"}`)
	})
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(l) }()
	defer server.Close()
	s := NewServer(path, 0)
	defer s.Close()
	s.engine.validate = func(path string, _ int) error {
		st, e := os.Lstat(path)
		if e != nil || st.Mode()&os.ModeSocket == 0 {
			return errors.New("engine socket is not a Unix socket")
		}
		return nil
	}
	response := s.run(context.Background(), dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpEngineInfo})
	if !response.Failed || !strings.Contains(response.Reason, "below the verified") {
		t.Fatalf("incompatible engine accepted: %+v", response)
	}
}

func TestObserverSocketValidation(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "not-a-socket")
	if e := os.WriteFile(regular, []byte("x"), 0644); e != nil {
		t.Fatal(e)
	}
	if e := validateEngineSocket(regular, 0); e == nil {
		t.Fatal("regular file accepted as engine socket")
	}
	missing := filepath.Join(dir, "absent.sock")
	if e := validateEngineSocket(missing, 0); e == nil {
		t.Fatal("missing socket accepted")
	}
}

func TestObserverQueryConstruction(t *testing.T) {
	// The logs endpoint must not accept caller-controlled keys: the request
	// only carries bounds, and the daemon query is fixed here.
	engine := newRecordingEngine(t)
	c := newClient(engine.path, 0)
	q := c.fixedQuery(dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerLogs, Logs: dockerobs.LogOptions{Since: timeUnix(0), Until: timeUnix(60), Records: 5}})
	if q.Get("stdout") != "1" || q.Get("stderr") != "1" || q.Get("timestamps") != "1" {
		t.Fatal("stream selection lost")
	}
	if q.Get("since") != "0" || q.Get("until") != "60" || q.Get("tail") != "5" {
		t.Fatal("window bounds lost")
	}
	if _, ok := q["follow"]; ok {
		t.Fatal("follow mode requested")
	}
	list := c.fixedQuery(dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerList})
	if list.Get("all") != "1" {
		t.Fatal("container list lost the all flag")
	}
}

func timeUnix(seconds int64) time.Time {
	return time.Unix(seconds, 0)
}
