package diagnosticsapp

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Monska85/hostlens/internal/backend"
	"github.com/Monska85/hostlens/internal/contract"
	linux "github.com/Monska85/hostlens/internal/platform/linux"
	"github.com/Monska85/hostlens/internal/token"
)

func Main(args []string) error {
	if len(args) == 0 || args[0] == "--help" {
		fmt.Println("hostlens-diagnostics serve [--system] [--config PATH]\nhostlens-diagnostics version")
		return nil
	}
	if args[0] == "version" {
		fmt.Println(contract.Version)
		return nil
	}
	if args[0] != "serve" {
		return errors.New("diagnostic backend supports serve and version only")
	}
	f := flag.NewFlagSet("serve", flag.ContinueOnError)
	system := f.Bool("system", false, "root-controlled system configuration")
	path := f.String("config", "", "configuration path")
	if e := f.Parse(args[1:]); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return nil
		}
		return e
	}
	if f.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *path == "" {
		home, _ := os.UserHomeDir()
		*path = filepath.Join(home, ".config/hostlens/config.yaml")
		if *system {
			*path = "/etc/hostlens/config.yaml"
		}
	}
	load := func() (backend.Snapshot, error) {
		cfg, p, e := linux.Load(*path, *system)
		if e != nil {
			return backend.Snapshot{}, e
		}
		return backend.NewSnapshot(cfg, p), nil
	}
	snap, e := load()
	if e != nil {
		return e
	}
	cfg := snap.Config
	if cfg.Privilege == "standard" {
		for _, secret := range []string{cfg.TokenStore, cfg.Server.TLS.KeyFile} {
			if secret != "" {
				if e := linux.SecretIsMasked(secret); e != nil {
					return e
				}
			}
		}
	}

	uid, e := linux.UID(cfg.GatewayUser)
	if e != nil {
		return e
	}
	listener, e := linux.ListenUnix(cfg.Socket, uid)
	if e != nil {
		return e
	}
	defer listener.Close()
	s := backend.New(snap, load, func(s backend.Snapshot) contract.Collector {
		c := &linux.Collector{Config: s.Config, Policy: s.Policy}
		if s.Config.Docker.Enabled {
			c.Docker = linux.NewObserverClient(s.Config.Docker.ObserverSocket)
		}
		return c
	}, token.Random(12))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	server := newServer(s.Handler())
	go func() { <-ctx.Done(); server.Close() }()
	e = server.Serve(listener)
	if errors.Is(e, http.ErrServerClosed) {
		return nil
	}
	return e
}

func newServer(handler http.Handler) *http.Server {
	return &http.Server{Handler: handler, ReadHeaderTimeout: contract.IPCReadTimeout, ReadTimeout: contract.IPCReadTimeout, WriteTimeout: contract.IPCWriteTimeout, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
}
