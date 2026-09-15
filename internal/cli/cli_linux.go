package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Monska85/hostlens/internal/backend"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/gateway"
	linux "github.com/Monska85/hostlens/internal/platform/linux"
	"github.com/Monska85/hostlens/internal/token"
	"golang.org/x/sys/unix"
)

// maxAdminResponseBytes bounds any administrative IPC response the CLI reads.
const maxAdminResponseBytes = 65536

func flags(name string) *flag.FlagSet { return flag.NewFlagSet(name, flag.ContinueOnError) }
func Main(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Println("HostLens " + contract.Version + "\nCommands: serve, config validate, policy explain PATH, reconcile --system, status, reload, token create|list|update|revoke|rotate, install, upgrade, uninstall, version\nUse COMMAND --help for flags. Every command accepts --config PATH and --system where applicable.")
		return nil
	}
	if args[0] == "version" {
		fmt.Println(contract.Version)
		return nil
	}
	if args[0] == "install" || args[0] == "upgrade" || args[0] == "uninstall" {
		return lifecycleCLI(args)
	}
	if args[0] == "reconcile" {
		return reconcileCLI(args)
	}
	cmd := args[0]
	rest := args[1:]
	sub := ""
	if cmd == "token" || cmd == "config" || cmd == "policy" {
		if len(rest) == 0 {
			return errors.New("subcommand required")
		}
		sub = rest[0]
		rest = rest[1:]
	}
	f := flags(cmd)
	system := f.Bool("system", false, "use root-controlled system configuration")
	path := f.String("config", "", "configuration path")
	name := f.String("name", "", "token client name")
	roles := f.String("roles", "", "comma-separated roles: health, inspect, diagnostics, metrics")
	expires := f.String("expires", "", "required RFC3339 token expiry or 'never'")
	overlap := f.String("overlap-until", "", "required RFC3339 old-token overlap deadline (a finite date)")
	id := f.String("id", "", "public token ID")
	all := f.Bool("all", false, "include inactive token metadata")
	recursive := f.Bool("recursive", false, "bounded recursive policy explanation")
	var target string
	if cmd == "policy" && len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		target = rest[0]
		rest = rest[1:]
	}
	if e := f.Parse(rest); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return nil
		}
		return e
	}
	if f.NArg() > 0 {
		if target == "" && cmd == "policy" && f.NArg() == 1 {
			target = f.Arg(0)
		} else {
			return errors.New("unexpected positional arguments")
		}
	}
	if *path == "" {
		home, _ := os.UserHomeDir()
		*path = filepath.Join(home, ".config/hostlens/config.yaml")
		if *system {
			*path = "/etc/hostlens/config.yaml"
		}
	}
	abs, e := filepath.Abs(*path)
	if e != nil {
		return e
	}
	*path = abs
	load := func() (backend.Snapshot, error) {
		cfg, p, e := linux.Load(*path, *system)
		s := backend.Snapshot{Config: cfg, Policy: p}
		if e == nil {
			s = backend.NewSnapshot(s.Config, s.Policy)
		}
		return s, e
	}
	snap, e := load()
	if e != nil {
		return e
	}
	uid := os.Getuid()
	if *system {
		uid = 0
	}
	switch cmd {
	case "config":
		if sub != "validate" {
			return errors.New("unknown config command")
		}
		fmt.Println("configuration valid; effective policy " + snap.Fingerprint)
		return nil
	case "token":
		store := token.Store{Path: snap.Config.TokenStore, AdminUID: uid}
		// The literal never selects the non-expiring sentinel; the token
		// package keeps time.Time semantics for every other value.
		parseTime := func(s string) (time.Time, error) {
			if s == "never" {
				return time.Time{}, nil
			}
			return time.Parse(time.RFC3339, s)
		}
		roleList := strings.Split(*roles, ",")
		switch sub {
		case "create":
			t, e := parseTime(*expires)
			if e != nil {
				return e
			}
			r, secret, e := store.Create(*name, roleList, t)
			if e != nil {
				return e
			}
			return output(map[string]any{"token": displayToken(r), "secret": secret})
		case "list":
			rs, e := store.List(*all)
			if e != nil {
				return e
			}
			shown := make([]map[string]any, len(rs))
			for i, r := range rs {
				shown[i] = displayToken(r)
			}
			return output(shown)
		case "update", "revoke":
			return store.Update(*id, roleList, sub == "revoke")
		case "rotate":
			t, e := parseTime(*expires)
			if e != nil {
				return e
			}
			o, e := parseTime(*overlap)
			if e != nil {
				return e
			}
			r, secret, e := store.Rotate(*id, t, o)
			if e != nil {
				return e
			}
			return output(map[string]any{"token": displayToken(r), "secret": secret})
		default:
			return errors.New("unknown token command")
		}
	case "status", "reload":
		if os.Geteuid() != uid {
			return errors.New("local administrator identity required")
		}
		resp, e := linux.UnixClient(snap.Config.AdminSocket).Post("http://unix/"+cmd, "application/json", nil)
		if e != nil {
			return e
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("administrative operation failed: %d", resp.StatusCode)
		}
		_, e = io.Copy(os.Stdout, io.LimitReader(resp.Body, maxAdminResponseBytes))
		return e
	case "policy":
		if sub != "explain" || target == "" {
			return errors.New("policy explain PATH required")
		}
		return explain(snap, *path, target, *recursive, uid)
	case "serve":
		return serve(snap, load, uid)
	default:
		return errors.New("unknown command")
	}
}
func output(v any) error { e := json.NewEncoder(os.Stdout); e.SetIndent("", "  "); return e.Encode(v) }

// displayToken renders a token record for humans: the zero-expiry sentinel
// becomes "never" instead of a raw zero timestamp. The store keeps time.Time.
func displayToken(r token.Record) map[string]any {
	expires := r.Expires.Format(time.RFC3339)
	if r.Expires.IsZero() {
		expires = "never"
	}
	return map[string]any{"id": r.ID, "name": r.Name, "roles": r.Roles, "expires": expires, "status": r.Status}
}
func serve(snap backend.Snapshot, load backend.Loader, adminUID int) error {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	cfg := snap.Config
	restart, e := gateway.RestartFingerprint(snap)
	if e != nil {
		return e
	}
	c := &gateway.Coordinator{Active: snap, Load: load, HTTP: linux.UnixClient(cfg.Socket), Tokens: token.Store{Path: cfg.TokenStore}, Instance: token.Random(12), Log: log, Restart: restart}
	c.Log = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: c}))
	admin, e := linux.ListenUnix(cfg.AdminSocket, uint32(adminUID))
	if e != nil {
		return e
	}
	defer admin.Close()
	adminServer := &http.Server{Handler: c.Admin(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: time.Minute, WriteTimeout: time.Minute}
	defer adminServer.Close()
	go adminServer.Serve(admin)
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-hup:
				if e := c.Reload(ctx); e != nil {
					log.Error("reload_failed", "outcome", "rejected")
				} else {
					log.Info("reload", "outcome", "activated")
				}
			}
		}
	}()
	listeners, e := gateway.Listen(cfg)
	if e != nil {
		return e
	}
	return gateway.Serve(ctx, listeners, c.Handler(), cfg)
}
func explain(s backend.Snapshot, configPath, target string, recursive bool, uid int) error {
	comparison := "UNKNOWN"
	var live backend.Status
	if os.Geteuid() == uid {
		resp, e := linux.UnixClient(s.Config.AdminSocket).Post("http://unix/status", "application/json", nil)
		if e == nil {
			defer resp.Body.Close()
			if resp.StatusCode == 200 && json.NewDecoder(io.LimitReader(resp.Body, maxAdminResponseBytes)).Decode(&live) == nil {
				comparison = "DIFFERENT"
				if live.Fingerprint == s.Fingerprint {
					comparison = "MATCH"
				}
			}
		}
	}
	entries := []map[string]any{}
	truncated := false
	traversalIssues := []string{}
	// Docker resource targets are policy identities, not filesystem paths.
	// They explain every matching allow and deny rule with provenance and
	// the resolved decision, without claiming any path resolution.
	if dockerTarget, ok := strings.CutPrefix(target, "docker:"); ok {
		decision := "denied"
		if s.Policy.Allowed("docker", dockerTarget, false) {
			decision = "allowed"
		}
		entries = append(entries, map[string]any{
			"target":   target,
			"decision": decision,
			"matches":  s.Policy.Matches("docker", dockerTarget),
		})
		return output(map[string]any{"comparison": comparison, "evaluates": "effective docker policy and MCP read-only setting; not OS access", "config": configPath, "fingerprint": s.Fingerprint, "mcp_read_only": s.Config.MCP.ReadOnly, "instance": live, "entries": entries, "truncated": truncated, "traversal_issues": traversalIssues})
	}
	target, e := filepath.Abs(target)
	if e != nil {
		return e
	}
	inspect := func(p string) {
		resolved, e := filepath.EvalSymlinks(p)
		decision := "denied"
		if s.Policy.Allowed("files", p, false) {
			decision = "allowed"
		}
		item := map[string]any{"path": p, "decision": decision, "matches": s.Policy.Matches("files", p)}
		if e != nil {
			item["resolution"] = "unresolved"
			item["decision"] = "unevaluable"
		} else {
			item["resolved"] = resolved
			item["resolved_matches"] = s.Policy.Matches("files", resolved)
			if !s.Policy.Allowed("files", resolved, false) {
				item["decision"] = "denied"
			}
			if resolved != p {
				item["safe_read"] = "symlinks rejected"
			}
		}
		entries = append(entries, item)
	}
	inspect(target)
	if recursive {
		ctx, cancel := context.WithTimeout(context.Background(), s.Config.Limits.ExplainTimeout)
		defer cancel()
		truncated, traversalIssues = explainDescendants(ctx, target, s.Config.Limits.ExplainEntries-1, inspect)
	}
	return output(map[string]any{"comparison": comparison, "evaluates": "effective disk policy and MCP read-only setting; not OS access or complete configuration equality", "config": configPath, "fingerprint": s.Fingerprint, "mcp_read_only": s.Config.MCP.ReadOnly, "instance": live, "entries": entries, "truncated": truncated, "traversal_issues": traversalIssues})
}

func explainDescendants(ctx context.Context, root string, remaining int, inspect func(string)) (bool, []string) {
	st, err := os.Lstat(root)
	if err != nil {
		return true, []string{root + ": traversal unavailable"}
	}
	if !st.IsDir() {
		return false, nil
	}
	pending := []string{root}
	issues := []string{}
	truncated := false
	for len(pending) > 0 {
		if ctx.Err() != nil || remaining == 0 {
			return true, issues
		}
		dir := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		f, err := os.OpenFile(dir, os.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
		if err != nil {
			issues = append(issues, dir+": traversal unavailable")
			truncated = true
			continue
		}
		for {
			if ctx.Err() != nil || remaining == 0 {
				f.Close()
				return true, issues
			}
			children, err := f.ReadDir(min(remaining, 64))
			for _, child := range children {
				if ctx.Err() != nil {
					f.Close()
					return true, issues
				}
				path := filepath.Join(dir, child.Name())
				inspect(path)
				remaining--
				if child.IsDir() {
					pending = append(pending, path)
				}
			}
			if err != nil {
				if err != io.EOF {
					issues = append(issues, dir+": traversal unavailable")
					truncated = true
				}
				break
			}
		}
		f.Close()
	}
	return truncated, issues
}
