//go:build linux

package containers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRestartResponseLossLeavesInvocationUnknown(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("root-owned Docker socket fixture runs in disposable container")
	}
	const id = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	socket := filepath.Join(t.TempDir(), "docker.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := os.Chmod(socket, 0660); err != nil {
		t.Fatal(err)
	}
	restarted := make(chan struct{}, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/containers/"+id+"/json"):
			writer.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(writer, `{"Id":%q,"Name":"/fixture","State":{"Status":"running"}}`, id)
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/containers/"+id+"/restart"):
			restarted <- struct{}{}
			connection, _, err := writer.(http.Hijacker).Hijack()
			if err == nil {
				connection.Close()
			}
		default:
			http.NotFound(writer, request)
		}
	})}
	go server.Serve(listener)
	defer server.Close()
	restarter, err := NewRestarter(socket)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	outcome, err := restarter.Restart(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-restarted:
	default:
		t.Fatal("restart POST was not received")
	}
	if outcome.Invoked != nil || outcome.Completed != nil || outcome.Issue != "restart_outcome_unknown" {
		t.Fatalf("lost response was reported as a known outcome: %+v", outcome)
	}
}
