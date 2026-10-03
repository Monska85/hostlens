//go:build linux

package localipc

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestShutdownWaitsForAdmittedRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	worker := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		writer.WriteHeader(http.StatusOK)
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- ServeUntilStopped(ctx, worker, listener, 5*time.Second) }()
	requestDone := make(chan error, 1)
	go func() {
		client := http.Client{Transport: &http.Transport{Proxy: nil}}
		response, err := client.Get("http://" + listener.Addr().String())
		if err == nil {
			response.Body.Close()
		}
		requestDone <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not enter handler")
	}
	cancel()
	select {
	case err := <-done:
		t.Fatalf("worker returned before admitted handler: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-requestDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("request did not finish")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("shutdown failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not finish after request")
	}
}
