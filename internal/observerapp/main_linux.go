package observerapp

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
)

// Main runs the isolated Docker observer. The process is only useful when an
// administrator enabled and reconciled Docker diagnostics; it refuses to
// start from a disabled or invalid configuration.
func Main(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Println("hostlens-docker-observer serve [--system] [--config PATH]\nhostlens-docker-observer version")
		return nil
	}
	if args[0] == "version" {
		fmt.Println(contract.Version)
		return nil
	}
	if args[0] != "serve" {
		return errors.New("docker observer supports serve and version only")
	}
	f := flag.NewFlagSet("serve", flag.ContinueOnError)
	// The observer only serves the system configuration; --system is
	// accepted for command consistency with the other executables.
	_ = f.Bool("system", true, "system installation (always enabled)")
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
		*path = "/etc/hostlens/config.yaml"
	}
	b, e := os.ReadFile(*path)
	if e != nil {
		return e
	}
	var cfg config.Config
	if e := config.Decode(b, &cfg); e != nil {
		return fmt.Errorf("observer configuration invalid: %w", e)
	}
	if e := config.ValidateLinux(cfg); e != nil {
		return fmt.Errorf("observer configuration invalid: %w", e)
	}
	if !cfg.Docker.Enabled {
		return errors.New("docker diagnostics are not enabled; observer refuses to start")
	}
	uid, e := resolveUID(cfg.DiagnosticsUser)
	if e != nil {
		return fmt.Errorf("diagnostic backend identity unavailable: %w", e)
	}
	gid, e := resolveGID(cfg.Docker.Group)
	if e != nil {
		return fmt.Errorf("docker access group unavailable: %w", e)
	}
	var listener net.Listener
	if os.Getenv("LISTEN_FDS") != "" {
		// Socket activation: the manager owns the IPC socket and passes the
		// listening file descriptor; the observer never touches the runtime
		// directory of the diagnostic backend.
		listener, e = ListenInherited(uid)
	} else {
		if e := os.MkdirAll(filepath.Dir(cfg.Docker.ObserverSocket), 0770); e != nil {
			return e
		}
		listener, e = ListenIPC(cfg.Docker.ObserverSocket, uid)
	}
	if e != nil {
		return e
	}
	defer listener.Close()
	server := &http.Server{
		Handler:           NewServer(cfg.Docker.DaemonSocket, gid).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    8192,
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	go func() { <-ctx.Done(); server.Close() }()
	e = server.Serve(listener)
	if errors.Is(e, http.ErrServerClosed) {
		return nil
	}
	return e
}
