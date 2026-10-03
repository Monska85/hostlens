//go:build linux

package localipc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// ServeUntilStopped waits for admitted handlers to finish before the worker
// exits. A timed-out shutdown closes connections to cancel remaining requests.
func ServeUntilStopped(ctx context.Context, server *http.Server, listener net.Listener, timeout time.Duration) error {
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	var serveErr error
	serveReturned := false
	select {
	case serveErr = <-served:
		serveReturned = true
		if ctx.Err() == nil {
			return serveErr
		}
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	shutdownErr := server.Shutdown(shutdown)
	if shutdownErr != nil {
		server.Close()
	}
	if !serveReturned {
		serveErr = <-served
	}
	if shutdownErr != nil {
		return fmt.Errorf("worker shutdown: %w", shutdownErr)
	}
	if errors.Is(serveErr, http.ErrServerClosed) {
		return nil
	}
	return serveErr
}
