package diagnosticsapp

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/backend"
	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	linux "github.com/Monska85/hostlens/internal/platform/linux"
)

type deadlineCollector struct{}

func (deadlineCollector) Capabilities(context.Context) map[string]bool { return nil }
func (deadlineCollector) Collect(ctx context.Context, _ string, _ any) contract.Result {
	<-ctx.Done()
	return contract.Failure("timeout")
}

func TestIPCAllowsBodyReadAndMaximumOperationBeforeResponse(t *testing.T) {
	// Scale the production transport and operation budgets together to exercise
	// the full boundary without adding a minute to every test suite.
	const scale = 20
	cfg := config.DefaultsLinux(false)
	cfg.Limits.ToolTimeout = contract.MaxToolTimeout / scale
	be := backend.New(backend.Snapshot{Config: cfg, Generation: "one"}, nil, func(backend.Snapshot) contract.Collector { return deadlineCollector{} }, "test")
	server := newServer(be.Handler())
	server.ReadHeaderTimeout /= scale
	server.ReadTimeout /= scale
	server.WriteTimeout /= scale
	path := filepath.Join(t.TempDir(), "ipc.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	defer func() {
		server.Close()
		if err := <-done; err != http.ErrServerClosed {
			t.Error(err)
		}
	}()
	client := linux.UnixClient(path)
	client.Timeout /= scale
	reader, writer := io.Pipe()
	defer reader.Close()
	request, err := http.NewRequest("POST", "http://unix/call", reader)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"version":1,"generation":"one","tool":"get_os_info","args":{}}`
	request.ContentLength = int64(len(body))
	wrote := make(chan error, 1)
	go func() {
		if _, err := io.WriteString(writer, body[:1]); err != nil {
			wrote <- err
			writer.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
		_, err := io.WriteString(writer, body[1:])
		writer.Close()
		wrote <- err
	}()
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var result contract.Result
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || len(result.Issues) != 1 || result.Issues[0].Code != "timeout" {
		t.Fatalf("lost explicit operation timeout: %+v", result)
	}
	if err := <-wrote; err != nil {
		t.Fatal(err)
	}
}
