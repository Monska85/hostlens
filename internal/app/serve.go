package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os/signal"
	"time"

	"github.com/Monska85/hostlens/internal/auth"
	"github.com/Monska85/hostlens/internal/core"
	"github.com/Monska85/hostlens/internal/server"
	"github.com/spf13/cobra"
)

func runServe(cmd *cobra.Command, configPath string) error {
	config, policy, err := core.Load(configPath, server.ToolEffects())
	if err != nil {
		return err
	}
	if !gatewayIdentity(config) {
		return errors.New("gateway identity mismatch")
	}
	host, docker, repairs, err := platformServices(config)
	if err != nil {
		return err
	}
	app := server.New(Version, auth.Store{Path: config.TokenStore}, policy, config.ReadOnly, host, docker, repairs)
	ctx, stop := signal.NotifyContext(cmd.Context(), platformSignals()...)
	defer stop()
	listener, err := net.Listen("tcp", config.Listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	httpServer := &http.Server{Handler: app.Transport, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 50 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	done := make(chan error, 1)
	go func() {
		if config.TLSCert != "" {
			done <- httpServer.ServeTLS(listener, config.TLSCert, config.TLSKey)
		} else {
			done <- httpServer.Serve(listener)
		}
	}()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdown)
	}
}
