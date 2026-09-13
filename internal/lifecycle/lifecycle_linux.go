package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/token"
	"go.yaml.in/yaml/v3"
)

const ManifestPath = "/var/lib/hostlens/install.json"

type Resource struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	State   string `json:"state"`
	Hash    string `json:"hash,omitempty"`
	Mutable bool   `json:"mutable,omitempty"`
	Owned   bool   `json:"owned"`
}
type Manifest struct {
	Version   int        `json:"version"`
	Release   string     `json:"release"`
	State     string     `json:"state"`
	Resources []Resource `json:"resources"`
}
type Manager struct {
	Root string
	// Source optionally points at an extracted release directory for
	// reconciliation of installations that predate the observer binary.
	Source string
	Run    func(context.Context, string, ...string) ([]byte, error)
	Config config.Config
}

func (m Manager) path(p string) string { return filepath.Join(m.Root, p) }
func (m Manager) command(ctx context.Context, name string, args ...string) ([]byte, error) {
	if m.Run != nil {
		return m.Run(ctx, name, args...)
	}
	if m.Root != "" && m.Root != "/" {
		return nil, errors.New("identity or service commands require injected disposable environment adapter")
	}
	var exe string
	if filepath.IsAbs(name) {
		exe = name
	}
	for _, dir := range []string{"/usr/sbin", "/usr/bin", "/sbin", "/bin"} {
		if exe != "" {
			break
		}
		p := filepath.Join(dir, name)
		if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() {
			exe = p
			break
		}
	}
	if exe == "" {
		return nil, fmt.Errorf("required command unavailable: %s", name)
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LANG=C", "LC_ALL=C"}
	cmd.WaitDelay = time.Second
	return cmd.Output()
}
func (m Manager) save(man *Manifest) error {
	b, e := json.MarshalIndent(man, "", "  ")
	if e != nil {
		return e
	}
	return token.Atomic(m.path(ManifestPath), b, 0600)
}
func (m Manager) Load() (Manifest, error) {
	var man Manifest
	b, e := os.ReadFile(m.path(ManifestPath))
	if e != nil {
		return man, e
	}
	e = json.Unmarshal(b, &man)
	if e == nil && man.Version != 1 {
		e = errors.New("unsupported installation state; migration required")
	}
	return man, e
}
func (m Manager) checkIdentities(ctx context.Context, allowMissing bool) error {
	groups := map[uint64]string{}
	for _, name := range []string{"hostlens-gateway", "hostlens-diagnostics"} {
		b, err := m.command(ctx, "getent", "group", name)
		if allowMissing && missingIdentity(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("group lookup %s: %w", name, err)
		}
		fields := strings.Split(strings.TrimSpace(string(b)), ":")
		if len(fields) != 4 || fields[0] != name {
			return fmt.Errorf("unsafe service group %s", name)
		}
		gid, err := strconv.ParseUint(fields[2], 10, 32)
		if err != nil || gid == 0 {
			return fmt.Errorf("nonzero numeric GID required for %s", name)
		}
		if previous, ok := groups[gid]; ok {
			return fmt.Errorf("service groups %s and %s share GID %d", previous, name, gid)
		}
		groups[gid] = name
		if fields[3] != "" {
			for _, member := range strings.Split(fields[3], ",") {
				if member != "hostlens-gateway" && member != "hostlens-diagnostics" {
					return fmt.Errorf("service group %s has unrelated member %q", name, member)
				}
			}
		}
	}
	seen := map[uint64]string{}
	for _, name := range []string{"hostlens-gateway", "hostlens-diagnostics"} {
		b, err := m.command(ctx, "getent", "passwd", name)
		if allowMissing && missingIdentity(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("identity lookup %s: %w", name, err)
		}
		fields := strings.Split(strings.TrimSpace(string(b)), ":")
		if len(fields) != 7 || fields[0] != name || !strings.HasSuffix(fields[6], "/nologin") {
			return fmt.Errorf("unsafe service identity %s", name)
		}
		uid, err := strconv.ParseUint(fields[2], 10, 32)
		if err != nil || uid == 0 {
			return fmt.Errorf("nonzero numeric UID required for %s", name)
		}
		if previous, ok := seen[uid]; ok {
			return fmt.Errorf("service identities %s and %s share UID %d", previous, name, uid)
		}
		seen[uid] = name
	}
	return nil
}

func (m Manager) Plan(source string) (Manifest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	man := Manifest{Version: 1, Release: contract.Version, State: "planned"}
	if _, e := os.Lstat(m.path(ManifestPath)); e == nil {
		return man, errors.New("installation manifest exists; inspect or uninstall partial installation")
	}
	for _, p := range []string{"/var/lib/hostlens", "/etc/hostlens", "/etc/hostlens/profiles", "/etc/hostlens/secrets", "/run/hostlens"} {
		_, e := os.Lstat(m.path(p))
		owned := errors.Is(e, os.ErrNotExist)
		if e != nil && !owned {
			return man, e
		}
		if !owned {
			return man, fmt.Errorf("installation conflict: pre-existing managed directory %s", p)
		}
		man.Resources = append(man.Resources, Resource{Path: p, Kind: "directory", Owned: owned})
	}
	if err := m.checkIdentities(ctx, true); err != nil {
		return man, err
	}
	for _, name := range []string{"hostlens-gateway", "hostlens-diagnostics"} {
		for _, kind := range []string{"group", "user"} {
			database := "group"
			if kind == "user" {
				database = "passwd"
			}
			_, err := m.command(ctx, "getent", database, name)
			state := "preserved"
			if missingIdentity(err) {
				state = "planned-absent"
			} else if err != nil {
				return man, fmt.Errorf("identity lookup %s: %w", name, err)
			}

			man.Resources = append(man.Resources, Resource{Path: name, Kind: kind, State: state})
		}
	}
	for _, p := range []string{"/usr/local/bin/hostlens", "/usr/local/bin/hostlens-diagnostics", "/usr/local/bin/hostlens-docker-observer", "/etc/hostlens/config.yaml", "/etc/hostlens/profiles/nginx.yaml", "/etc/hostlens/profiles/allow-all.yaml", "/etc/hostlens/profiles/docker-readonly.yaml", "/etc/systemd/system/hostlens-gateway.service", "/etc/systemd/system/hostlens-diagnostics.service"} {
		if _, e := os.Lstat(m.path(p)); e == nil {
			return man, fmt.Errorf("installation conflict: %s", p)
		}
		man.Resources = append(man.Resources, Resource{Path: p, Kind: "file", Mutable: strings.HasPrefix(p, "/etc/hostlens"), Owned: true})
	}
	for _, p := range []string{"/etc/hostlens/secrets/tokens.json", "/etc/hostlens/secrets/tokens.json.lock", "/run/hostlens/diagnostics.sock", "/run/hostlens/admin.sock"} {
		man.Resources = append(man.Resources, Resource{Path: p, Kind: "state", Mutable: true, Owned: true})
	}
	for _, name := range []string{"hostlens", "hostlens-diagnostics", "hostlens-docker-observer"} {
		if st, e := os.Stat(filepath.Join(source, name)); e != nil || !st.Mode().IsRegular() {
			return man, fmt.Errorf("source binary missing: %s", name)
		}
	}
	return man, nil
}
func (m Manager) Install(ctx context.Context, source string, start bool) error {
	man, e := m.Plan(source)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(m.path("/var/lib/hostlens"), 0700); e != nil {
		return e
	}
	man.State = "installing"
	man.Resources[0].State = "complete"
	if e = m.save(&man); e != nil {
		return e
	}
	identitiesChecked := false
	for i := range man.Resources {
		r := &man.Resources[i]
		if !r.Owned && r.State != "planned-absent" {
			r.State = "preserved"
			continue
		}
		r.State = "intent"
		if e = m.save(&man); e != nil {
			return e
		}
		switch r.Kind {
		case "directory":
			e = os.MkdirAll(m.path(r.Path), 0750)
		case "group", "user":
			database := "group"
			if r.Kind == "user" {
				database = "passwd"
			}
			_, lookup := m.command(ctx, "getent", database, r.Path)
			if !missingIdentity(lookup) {
				return fmt.Errorf("identity changed or cannot be verified before creation: %s", r.Path)
			}
			if r.Kind == "group" {
				_, e = m.command(ctx, "groupadd", "--system", r.Path)
			} else {
				_, e = m.command(ctx, "useradd", "--system", "--no-create-home", "--gid", r.Path, "--home-dir", "/nonexistent", "--shell", "/usr/sbin/nologin", r.Path)
			}
			if e == nil {
				r.Owned = true
			}

		case "file":
			if !identitiesChecked {
				if err := m.checkIdentities(ctx, false); err != nil {
					return err
				}
				identitiesChecked = true
			}
			var b []byte
			mode := os.FileMode(0644)
			switch r.Path {
			case "/usr/local/bin/hostlens", "/usr/local/bin/hostlens-diagnostics", "/usr/local/bin/hostlens-docker-observer":
				b, e = os.ReadFile(filepath.Join(source, filepath.Base(r.Path)))
				mode = 0755
			case "/etc/hostlens/config.yaml":
				b, e = yaml.Marshal(m.Config)
				mode = 0640
			case "/etc/systemd/system/hostlens-gateway.service":
				b = []byte(GatewayUnit())
			case "/etc/systemd/system/hostlens-diagnostics.service":
				b = []byte(DiagnosticsUnit(m.Config))
			default:
				b, e = os.ReadFile(filepath.Join(source, "profiles", filepath.Base(r.Path)))
			}
			if e == nil {
				e = token.Atomic(m.path(r.Path), b, mode)
				r.Hash = digest(b)
			}
		case "state":
			r.State = "reserved"
		}
		if e != nil {
			return fmt.Errorf("partial installation (%s): %w", r.Path, e)
		}
		if r.State == "intent" {
			r.State = "complete"
		}
		if e = m.save(&man); e != nil {
			return e
		}
	}
	if _, e = m.command(ctx, "chown", "root:hostlens-gateway", m.path("/etc/hostlens/secrets")); e != nil {
		return e
	}
	if e = os.Chmod(m.path("/etc/hostlens/secrets"), os.ModeSetgid|0750); e != nil {
		return e
	}
	if _, e = m.command(ctx, "chown", "hostlens-diagnostics:hostlens-gateway", m.path("/run/hostlens")); e != nil {
		return e
	}
	if e = os.Chmod(m.path("/run/hostlens"), 0770); e != nil {
		return e
	}
	if e = os.Chmod(m.path("/etc/hostlens"), 0755); e != nil {
		return e
	}
	if e = os.Chmod(m.path("/etc/hostlens/profiles"), 0755); e != nil {
		return e
	}
	if e = os.Chmod(m.path("/etc/hostlens/config.yaml"), 0644); e != nil {
		return e
	}
	if _, e = m.command(ctx, "systemctl", "daemon-reload"); e != nil {
		return e
	}
	man.State = "installed"
	if e = m.save(&man); e != nil {
		return e
	}
	if start {
		_, e = m.command(ctx, "systemctl", "start", "hostlens-diagnostics.service", "hostlens-gateway.service")
	}
	return e
}
func (m Manager) Uninstall(ctx context.Context) error {
	man, e := m.Load()
	if e != nil {
		return e
	}
	man.State = "uninstalling"
	if e = m.save(&man); e != nil {
		return e
	}
	var failures []error
	for _, svc := range []string{"hostlens-gateway.service", "hostlens-diagnostics.service", "hostlens-docker-observer.service", "hostlens-docker-observer.socket"} {
		var unit *Resource
		for i := range man.Resources {
			r := &man.Resources[i]
			if r.Path == "/etc/systemd/system/"+svc {
				unit = r
				break
			}
		}
		if unit == nil || !unit.Owned || unit.State == "" || unit.State == "removed" {
			continue
		}
		if _, err := os.Lstat(m.path(unit.Path)); errors.Is(err, os.ErrNotExist) {
			b, checkErr := m.command(ctx, "systemctl", "show", "--property=LoadState,ActiveState", svc)
			properties := string(b)
			if checkErr == nil && strings.Contains(properties, "LoadState=not-found\n") && strings.Contains(properties, "ActiveState=inactive\n") {
				continue
			}
			failures = append(failures, fmt.Errorf("cannot verify absent service %s", svc))
			continue
		}

		if _, e = m.command(ctx, "systemctl", "disable", "--now", svc); e != nil {
			failures = append(failures, fmt.Errorf("stop/disable %s: %w", svc, e))
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	// Remove owned runtime/config directories before checking account ownership.
	order := make([]int, 0, len(man.Resources))
	for i := len(man.Resources) - 1; i >= 0; i-- {
		if man.Resources[i].Kind != "user" && man.Resources[i].Kind != "group" {
			order = append(order, i)
		}
	}
	for i := len(man.Resources) - 1; i >= 0; i-- {
		if man.Resources[i].Kind == "user" {
			order = append(order, i)
		}
	}
	for i := len(man.Resources) - 1; i >= 0; i-- {
		if man.Resources[i].Kind == "group" {
			order = append(order, i)
		}
	}
	for _, i := range order {

		r := &man.Resources[i]
		if !r.Owned && r.State == "intent" && (r.Kind == "user" || r.Kind == "group") {
			database := "group"
			if r.Kind == "user" {
				database = "passwd"
			}
			if _, err := m.command(ctx, "getent", database, r.Path); !missingIdentity(err) {
				failures = append(failures, fmt.Errorf("uncertain identity creation preserved: %s", r.Path))
			}
		}
		if !r.Owned || r.State == "" || r.State == "removed" {
			continue
		}
		switch r.Kind {
		case "file", "state":
			p := m.path(r.Path)
			st, e := os.Lstat(p)
			if errors.Is(e, os.ErrNotExist) {
				break
			}
			if e != nil {
				failures = append(failures, e)
				continue
			}
			if st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
				failures = append(failures, fmt.Errorf("unexpected resource type preserved: %s", r.Path))
				continue
			}
			if r.Kind == "file" && !r.Mutable {
				b, e := os.ReadFile(p)
				if e != nil || digest(b) != r.Hash {
					failures = append(failures, fmt.Errorf("modified binary/unit preserved: %s", r.Path))
					continue
				}
			}
			if e = os.Remove(p); e != nil {
				failures = append(failures, e)
				continue
			}
		case "directory":
			if r.Path == "/var/lib/hostlens" {
				continue
			}
			if e := os.Remove(m.path(r.Path)); e != nil && !errors.Is(e, os.ErrNotExist) {
				failures = append(failures, fmt.Errorf("directory contents preserved %s: %w", r.Path, e))
				continue
			}
		case "user":
			if _, err := m.command(ctx, "getent", "passwd", r.Path); missingIdentity(err) {
				r.State = "removed"
				if e = m.save(&man); e != nil {
					return e
				}
				continue
			}
			b, e := m.command(ctx, "find", "/", "-path", "/proc", "-prune", "-o", "-path", "/sys", "-prune", "-o", "-path", "/dev", "-prune", "-o", "-user", r.Path, "-print", "-quit")
			if e != nil {
				failures = append(failures, fmt.Errorf("ownership check failed for %s; account preserved: %w", r.Path, e))
				continue
			}
			if len(strings.TrimSpace(string(b))) > 0 {
				failures = append(failures, fmt.Errorf("unexpected ownership for %s; account preserved", r.Path))
				continue
			}
			// userdel may remove a same-name private group implicitly.
			ownedGroup := false
			for _, group := range man.Resources {
				if group.Kind == "group" && group.Path == r.Path && group.Owned {
					ownedGroup = true
				}
			}
			if !ownedGroup {
				if _, err := m.command(ctx, "getent", "group", r.Path); !missingIdentity(err) {
					failures = append(failures, fmt.Errorf("account %s preserved to protect pre-existing or uncertain same-name group", r.Path))
					continue
				}
			}
			if _, e = m.command(ctx, "userdel", r.Path); e != nil {
				failures = append(failures, e)
				continue
			}
		case "group":
			if _, err := m.command(ctx, "getent", "group", r.Path); missingIdentity(err) {
				r.State = "removed"
				if e = m.save(&man); e != nil {
					return e
				}
				continue
			}
			b, checkErr := m.command(ctx, "find", "/", "-path", "/proc", "-prune", "-o", "-path", "/sys", "-prune", "-o", "-path", "/dev", "-prune", "-o", "-group", r.Path, "-print", "-quit")
			if checkErr != nil {
				failures = append(failures, fmt.Errorf("ownership check failed for %s; group preserved: %w", r.Path, checkErr))
				continue
			}
			if len(strings.TrimSpace(string(b))) > 0 {
				failures = append(failures, fmt.Errorf("unexpected ownership for %s; group preserved", r.Path))
				continue
			}
			if _, e = m.command(ctx, "groupdel", r.Path); e != nil {
				failures = append(failures, e)
				continue
			}
		}
		r.State = "removed"
		if e = m.save(&man); e != nil {
			return e
		}
	}
	if _, e = m.command(ctx, "systemctl", "daemon-reload"); e != nil {
		failures = append(failures, e)
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	dir, e := os.Open(m.path("/var/lib/hostlens"))
	if e != nil {
		return e
	}
	entries, e := dir.ReadDir(2)
	dir.Close()
	if e != nil && e != io.EOF {
		return e
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(ManifestPath) {
		return errors.New("unexpected installation-state content preserved; resolve it before retrying uninstall")
	}
	if e = os.Remove(m.path(ManifestPath)); e != nil {
		return e
	}
	if e = os.Remove(m.path("/var/lib/hostlens")); e != nil {
		// Preserve retry state if a concurrent directory addition defeated removal.
		return errors.Join(fmt.Errorf("installation-state directory retained: %w", e), m.save(&man))
	}
	return nil
}
func (m Manager) Upgrade(ctx context.Context, archive string) error {
	man, e := m.Load()
	if e != nil {
		return e
	}
	if man.State != "installed" {
		return errors.New("installation is partial; inspect manifest before upgrade")
	}
	stage, e := os.MkdirTemp(m.path("/var/lib/hostlens"), "stage-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(stage)
	rel, e := Extract(archive, stage, runtime.GOARCH)
	if e != nil {
		return e
	}
	if _, e = m.command(ctx, filepath.Join(stage, "hostlens"), "config", "validate", "--system"); e != nil {
		return fmt.Errorf("candidate config validation failed: %w", e)
	}
	active := map[string]bool{}
	for _, svc := range []string{"hostlens-diagnostics.service", "hostlens-gateway.service", "hostlens-docker-observer.service"} {
		b, err := m.command(ctx, "systemctl", "show", "--property=ActiveState", "--value", svc)
		if err != nil {
			return fmt.Errorf("cannot determine state of %s: %w", svc, err)
		}
		switch strings.TrimSpace(string(b)) {
		case "active":
			active[svc] = true
		case "inactive", "failed":
			active[svc] = false
		default:
			return fmt.Errorf("unstable or unknown state of %s; retry after it settles", svc)
		}
	}
	backup := m.path("/var/lib/hostlens/previous-" + token.Random(6))
	if e = os.Mkdir(backup, 0700); e != nil {
		return e
	}
	man.Resources = append(man.Resources, Resource{Path: strings.TrimPrefix(backup, m.Root), Kind: "directory", Owned: true, State: "complete"})
	// The observer binary may be absent when Docker diagnostics are
	// disabled; upgrades keep the rest of the artifact set coherent and
	// restore the observer from the archive unconditionally.
	presentObserver := true
	if _, e := os.Stat(m.path(observerBinary)); errors.Is(e, os.ErrNotExist) {
		presentObserver = false
	}
	for _, name := range []string{"hostlens", "hostlens-diagnostics", "hostlens-docker-observer"} {
		b, e := os.ReadFile(m.path("/usr/local/bin/" + name))
		if e != nil {
			if !errors.Is(e, os.ErrNotExist) {
				return e
			}
			// Only the observer may be absent: the other executables are
			// always installed.
			if name != "hostlens-docker-observer" {
				return e
			}
			continue
		}
		p := filepath.Join(backup, name)
		man.Resources = append(man.Resources, Resource{Path: strings.TrimPrefix(p, m.Root), Kind: "file", Owned: true, State: "intent", Hash: digest(b)})
		if e = m.save(&man); e != nil {
			return e
		}
		if e = token.Atomic(p, b, 0755); e != nil {
			return e
		}
		man.Resources[len(man.Resources)-1].State = "complete"
	}
	// An absent observer binary records its removal durably so a rollback
	// leaves the manifest consistent with the restored filesystem state.
	if !presentObserver {
		for i := range man.Resources {
			if man.Resources[i].Path == observerBinary && man.Resources[i].Kind == "file" {
				man.Resources[i].State = "removed"
			}
		}
		if e = m.save(&man); e != nil {
			return e
		}
	}
	man.State = "upgrading"
	if e = m.save(&man); e != nil {
		return e
	}
	rollback := func(cause error) error {
		recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		var failures []error
		for _, name := range []string{"hostlens", "hostlens-diagnostics", "hostlens-docker-observer"} {
			b, e := os.ReadFile(filepath.Join(backup, name))
			if errors.Is(e, os.ErrNotExist) && name == "hostlens-docker-observer" {
				// The observer was absent before the upgrade; the rollback
				// removes the replaced binary so no stray executable stays.
				if e = os.Remove(m.path("/usr/local/bin/" + name)); e != nil && !errors.Is(e, os.ErrNotExist) {
					failures = append(failures, e)
				}
				for i := range man.Resources {
					if man.Resources[i].Path == "/usr/local/bin/"+name {
						man.Resources[i].State = "removed"
					}
				}
				continue
			}
			if e == nil {
				e = token.Atomic(m.path("/usr/local/bin/"+name), b, 0755)
				for i := range man.Resources {
					if man.Resources[i].Path == "/usr/local/bin/"+name {
						man.Resources[i].Hash = digest(b)
					}
				}
			}
			if e != nil {
				failures = append(failures, e)
			}
		}
		for _, svc := range []string{"hostlens-diagnostics.service", "hostlens-gateway.service", "hostlens-docker-observer.service"} {
			if active[svc] {
				// A crashing candidate can exhaust systemd's start-rate limit.
				// Allow one recovery attempt with the restored executable.
				state, stateErr := m.command(recovery, "systemctl", "show", "--property=ActiveState", "--value", svc)
				if stateErr != nil {
					failures = append(failures, stateErr)
				} else if strings.TrimSpace(string(state)) != "inactive" {
					// Stopped units may have been unloaded; they have no failure
					// counter to reset and reset-failed would reject them.
					_, e = m.command(recovery, "systemctl", "reset-failed", svc)
					if e != nil {
						failures = append(failures, e)
					}
				}
				if _, e = m.command(recovery, "systemctl", "restart", svc); e != nil {
					failures = append(failures, e)
				} else if e = m.ready(recovery, svc); e != nil {
					failures = append(failures, e)
				}
			}
		}
		if len(failures) == 0 {
			man.State = "installed"
			if e = m.save(&man); e != nil {
				failures = append(failures, e)
			}
		}
		return errors.Join(append([]error{cause}, failures...)...)
	}
	for _, svc := range []string{"hostlens-gateway.service", "hostlens-diagnostics.service", "hostlens-docker-observer.service"} {
		if active[svc] {
			if _, e = m.command(ctx, "systemctl", "stop", svc); e != nil {
				return rollback(e)
			}
		}
	}

	for _, name := range []string{"hostlens", "hostlens-diagnostics", "hostlens-docker-observer"} {
		b, e := os.ReadFile(filepath.Join(stage, name))
		if e != nil {
			return rollback(e)
		}
		if e = token.Atomic(m.path("/usr/local/bin/"+name), b, 0755); e != nil {
			return rollback(e)
		}
		// The archive always carries the observer; upgrades adopt or update
		// its ownership record so uninstall never preserves a stray binary.
		recorded := false
		for i := range man.Resources {
			if man.Resources[i].Path == "/usr/local/bin/"+name {
				man.Resources[i].Hash = digest(b)
				if name == "hostlens-docker-observer" {
					man.Resources[i].Owned = true
					man.Resources[i].State = "complete"
				}
				recorded = true
			}
		}
		if name == "hostlens-docker-observer" && !recorded {
			man.Resources = append(man.Resources, Resource{Path: "/usr/local/bin/" + name, Kind: "file", Owned: true, State: "complete", Hash: digest(b)})
		}
	}
	for _, svc := range []string{"hostlens-diagnostics.service", "hostlens-gateway.service", "hostlens-docker-observer.service"} {
		if active[svc] {
			if _, e = m.command(ctx, "systemctl", "start", svc); e != nil {
				return rollback(e)
			}
			if e = m.ready(ctx, svc); e != nil {
				return rollback(e)
			}
		}
	}
	profiles, _ := filepath.Glob(filepath.Join(stage, "profiles/*.yaml"))
	sort.Strings(profiles)
	for _, p := range profiles {
		b, e := os.ReadFile(p)
		if e != nil {
			return rollback(e)
		}
		existing, _ := os.ReadFile(m.path("/etc/hostlens/profiles/" + filepath.Base(p)))
		if string(b) == string(existing) {
			continue
		}
		target := filepath.Join(backup, filepath.Base(p)+".candidate")
		if e = token.Atomic(target, b, 0644); e != nil {
			return rollback(e)
		}
		man.Resources = append(man.Resources, Resource{Path: strings.TrimPrefix(target, m.Root), Kind: "file", Owned: true, State: "complete", Hash: digest(b)})
		fmt.Fprintf(os.Stderr, "Profile update awaits administrator review: %s; active profile preserved\n", target)
	}
	man.State = "installed"
	man.Release = rel.Version
	return m.save(&man)
}

func (m Manager) ready(ctx context.Context, svc string) error {
	if m.Run != nil {
		_, e := m.command(ctx, "systemctl", "is-active", "--quiet", svc)
		return e
	}
	if svc == "hostlens-docker-observer.service" {
		// The observer is socket-activated and serves no /status endpoint;
		// readiness means its unit is running.
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		b, e := m.command(ctx, "systemctl", "show", "--property=ActiveState", "--value", svc)
		if e != nil {
			return e
		}
		if strings.TrimSpace(string(b)) != "active" {
			return fmt.Errorf("%s activation readiness failed", svc)
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	socket := m.Config.Socket
	if svc == "hostlens-gateway.service" {
		socket = m.Config.AdminSocket
	}
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{DisableKeepAlives: true, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}}
	for {
		req, _ := http.NewRequestWithContext(ctx, "POST", "http://unix/status", nil)
		resp, e := client.Do(req)
		if e == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				if svc != "hostlens-gateway.service" {
					return nil
				}
				address := m.Config.Server.Bind[0]
				if address == "0.0.0.0" {
					address = "127.0.0.1"
				}
				if address == "::" {
					address = "::1"
				}
				conn, e := net.DialTimeout("tcp", net.JoinHostPort(address, strconv.Itoa(m.Config.Server.Port)), time.Second)
				if e == nil {
					conn.Close()
					return nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s activation readiness failed: %w", svc, ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func missingIdentity(err error) bool {
	var exit interface{ ExitCode() int }
	return errors.As(err, &exit) && exit.ExitCode() == 2
}
