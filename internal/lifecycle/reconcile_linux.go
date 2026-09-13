package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/token"
)

// Docker observer lifecycle constants. The observer identity, units, binary,
// and socket are the only resources reconciliation may create or remove, and
// only with recorded ownership.
const (
	observerGroup  = "hostlens-observer"
	observerUser   = "hostlens-observer"
	observerBinary = "/usr/local/bin/hostlens-docker-observer"
	observerSvc    = "hostlens-docker-observer.service"
	observerSock   = "hostlens-docker-observer.socket"
)

// ReconcileReport is the non-mutating plan or applied result of one
// reconciliation run. Disclosure precedes every mutation.
type ReconcileReport struct {
	Desired    string     `json:"desired"`
	Current    string     `json:"current"`
	Observer   bool       `json:"observer_provisioned"`
	Changed    []Resource `json:"changed,omitempty"`
	Preserved  []Resource `json:"preserved,omitempty"`
	Disclosure string     `json:"disclosure,omitempty"`
}

// Reconcile computes the complete Docker observer topology plan. With apply
// it moves the optional observer between disabled and provisioned states
// transactionally using the existing ownership, conflict, and interruption
// protections. It never installs, configures, starts, stops, or restarts
// Docker and never grants Docker access to the gateway or diagnostic
// backend.
func (m Manager) Reconcile(ctx context.Context, apply bool) (Manifest, ReconcileReport, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	man, e := m.Load()
	if e != nil {
		return man, ReconcileReport{}, fmt.Errorf("installation manifest unavailable: %w", e)
	}
	if man.State != "installed" {
		return man, ReconcileReport{}, errors.New("installation is partial; inspect manifest before reconciliation")
	}
	if e = config.ValidateLinux(m.Config); e != nil {
		return man, ReconcileReport{}, fmt.Errorf("docker configuration invalid: %w", e)
	}
	report := ReconcileReport{
		Desired: "disabled",
		Current: "disabled",
	}
	if m.observerProvisioned(&man) {
		report.Current = "provisioned"
	}
	if m.Config.Docker.Enabled {
		report.Desired = "enabled-waiting"
		if _, e := os.Lstat(m.Config.Docker.DaemonSocket); e == nil {
			report.Desired = "provisioned"
		}
		fmt.Fprintln(os.Stderr, "Docker observer access to the rootful Docker socket is root-equivalent.")
		fmt.Fprintln(os.Stderr, "Access is process-scoped, recorded in the plan, and removable by disabling the integration.")
	}
	// The plan must show every identity, unit, socket, and file change the
	// topology move performs before any mutation happens.
	report.Disclosure = m.disclosure(&man)
	if !apply {
		report.Changed = m.planChanges(&man)
		report.Observer = m.observerProvisioned(&man)
		return man, report, nil
	}
	if e = m.reconcile(ctx, &man); e != nil {
		return man, report, e
	}
	report.Observer = m.observerProvisioned(&man)
	report.Changed = m.planChanges(&man)
	return man, report, nil
}

// planChanges enumerates the observer topology changes between the recorded
// manifest and the desired state. In dry-run mode this is the plan; after
// apply it is the reconciliation result.
func (m *Manager) planChanges(man *Manifest) []Resource {
	changes := []Resource{}
	desiredEnabled := m.Config.Docker.Enabled
	for _, r := range man.Resources {
		isObserver := strings.Contains(r.Path, "hostlens-docker-observer") ||
			(r.Kind == "state" && strings.HasSuffix(r.Path, "docker-observer.sock"))
		if !isObserver && r.Path != observerGroup && r.Path != observerUser && r.Path != m.Config.Docker.ObserverSocket {
			continue
		}
		if desiredEnabled {
			switch {
			case r.Kind == "state" && strings.HasSuffix(r.Path, "docker-observer.sock") && r.Path != m.Config.Docker.ObserverSocket:
				// A superseded socket record is a removal, never a create.
				if r.State != "removed" {
					changes = append(changes, Resource{Path: r.Path, Kind: r.Kind, State: "remove"})
				}
			case r.State != "complete" && r.State != "preserved":
				changes = append(changes, Resource{Path: r.Path, Kind: r.Kind, State: "create", Hash: r.Hash})
			case r.Kind == "group" || r.Kind == "user":
				// Drift repair: a recorded-owned identity deleted externally
				// is recreated by apply; the plan must disclose the
				// re-granted authority.
				database := "group"
				if r.Kind == "user" {
					database = "passwd"
				}
				if _, err := m.command(context.Background(), "getent", database, r.Path); missingIdentity(err) {
					changes = append(changes, Resource{Path: r.Path, Kind: r.Kind, State: "recreate"})
				}
			case r.Kind == "file" && r.Hash != "":
				b, e := os.ReadFile(m.path(r.Path))
				if e == nil && digest(b) == r.Hash {
					continue
				}
				// Content drift is a pending repair: units rewrite from the
				// generated template; the binary restores from the source.
				state := "rewrite"
				if errors.Is(e, os.ErrNotExist) || r.Path == observerBinary {
					state = "restore"
				}
				changes = append(changes, Resource{Path: r.Path, Kind: r.Kind, State: state, Hash: r.Hash})
			}
			continue
		}
		if r.Owned && r.State != "removed" {
			// Disablement preserves the dormant identity records by design
			// (no Docker group authority remains); the plan reports their
			// preservation instead of claiming a removal apply never does.
			state := "remove"
			if r.Kind == "group" || r.Kind == "user" {
				state = "preserve"
			}
			changes = append(changes, Resource{Path: r.Path, Kind: r.Kind, State: state})
		}
	}
	if desiredEnabled {
		if len(changes) == 0 && !m.observerProvisioned(man) {
			changes = append(changes,
				Resource{Path: observerGroup, Kind: "group", State: "create"},
				Resource{Path: observerUser, Kind: "user", State: "create"},
				Resource{Path: observerBinary, Kind: "file", State: "create"},
				Resource{Path: "/etc/systemd/system/" + observerSvc, Kind: "file", State: "create"},
				Resource{Path: "/etc/systemd/system/" + observerSock, Kind: "file", State: "create"},
				Resource{Path: m.Config.Docker.ObserverSocket, Kind: "state", State: "create"},
			)
		}
		// A configured socket path that no manifest record covers yet is a
		// pending change for both fresh provisioning and path moves.
		known := false
		for _, r := range man.Resources {
			if r.Path == m.Config.Docker.ObserverSocket && r.Kind == "state" {
				known = true
			}
		}
		if m.observerProvisioned(man) && !known {
			changes = append(changes, Resource{Path: m.Config.Docker.ObserverSocket, Kind: "state", State: "create"})
		}
	}
	return changes
}

// disclosure names the root-equivalent authority being granted or removed.
func (m *Manager) disclosure(man *Manifest) string {
	if !m.Config.Docker.Enabled {
		return "Docker diagnostics disabled: the observer service, socket unit, owned binary, and IPC socket are removed; the dormant hostlens-observer identity is preserved without Docker group authority."
	}
	return fmt.Sprintf(
		"Docker diagnostics enabled: the dedicated non-login identity %s receives process-scoped access to %s through the %s supplementary group, exposed by /etc/systemd/system/%s and /etc/systemd/system/%s. This access is root-equivalent and removable by disabling the integration.",
		observerUser, m.Config.Docker.DaemonSocket, m.Config.Docker.Group, observerSvc, observerSock,
	)
}

// observerProvisioned reports whether the observer topology is provisioned.
func (m *Manager) observerProvisioned(man *Manifest) bool {
	for _, r := range man.Resources {
		if r.Path == "/etc/systemd/system/"+observerSvc && r.Owned && r.State == "complete" {
			return true
		}
	}
	return false
}

// findResource locates one recorded resource entry without appending.
func findResource(man *Manifest, path, kind string) *Resource {
	for i := range man.Resources {
		if man.Resources[i].Path == path && man.Resources[i].Kind == kind {
			return &man.Resources[i]
		}
	}
	return nil
}

// upsertResource locates the recorded observer resource entry or appends one
// with fresh ownership intent.
func upsertResource(man *Manifest, path, kind string) *Resource {
	if r := findResource(man, path, kind); r != nil {
		return r
	}
	man.Resources = append(man.Resources, Resource{Path: path, Kind: kind, Owned: true})
	return &man.Resources[len(man.Resources)-1]
}

// reconcileIdentity ensures the dedicated non-login observer identity exists
// with recorded ownership. A pre-existing identity without ownership is a
// conflict HostLens refuses to adopt or edit.
func (m *Manager) reconcileIdentity(ctx context.Context, man *Manifest) error {
	for _, spec := range []struct{ name, kind, database string }{
		{observerGroup, "group", "group"},
		{observerUser, "user", "passwd"},
	} {
		r := findResource(man, spec.name, spec.kind)
		_, err := m.command(ctx, "getent", spec.database, spec.name)
		if missingIdentity(err) {
			if r == nil {
				man.Resources = append(man.Resources, Resource{Path: spec.name, Kind: spec.kind, Owned: true, State: "intent"})
				r = &man.Resources[len(man.Resources)-1]
			}
			if !r.Owned {
				return fmt.Errorf("installation conflict: uncertain observer identity %s", spec.name)
			}
			if r.State == "complete" {
				// Recorded ownership with a missing identity means the
				// account was removed externally; recreate it.
				r.State = "intent"
			}
			if r.State == "intent" {
				if e := m.save(man); e != nil {
					return e
				}
				var createErr error
				if spec.kind == "group" {
					_, createErr = m.command(ctx, "groupadd", "--system", spec.name)
				} else {
					if _, lookup := m.command(ctx, "getent", "group", observerGroup); missingIdentity(lookup) {
						return fmt.Errorf("observer group must exist before the service identity: %s", observerGroup)
					}
					_, createErr = m.command(ctx, "useradd", "--system", "--no-create-home", "--gid", observerGroup, "--home-dir", "/nonexistent", "--shell", "/usr/sbin/nologin", spec.name)
				}
				if createErr != nil {
					return fmt.Errorf("partial reconciliation (%s): %w", spec.name, createErr)
				}
				r.Owned = true
				r.State = "complete"
				if e := m.save(man); e != nil {
					return e
				}
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("observer identity lookup %s: %w", spec.name, err)
		}
		if r == nil || !r.Owned {
			return fmt.Errorf("installation conflict: pre-existing observer identity %s", spec.name)
		}
		r.State = "preserved"
	}
	return m.save(man)
}

// reconcileBinary records or replaces the observer binary. Fresh installs
// and upgrades provision it; reconciliation adopts the installed binary, or
// copies it from a supplied release source when upgrading from a release
// that predates the observer. A content-drifted binary is never silently
// adopted: the operator must restore it explicitly.
func (m *Manager) reconcileBinary(ctx context.Context, man *Manifest) error {
	r := upsertResource(man, observerBinary, "file")
	if b, e := os.ReadFile(m.path(observerBinary)); e == nil {
		if r.Hash != "" && digest(b) != r.Hash {
			return fmt.Errorf("modified observer binary preserved: %s; restore the verified release with reconcile --source before applying", observerBinary)
		}
		if r.Hash == "" {
			r.Hash = digest(b)
			r.Owned = true
			r.State = "complete"
			return m.save(man)
		}
		r.State = "complete"
		return m.save(man)
	}
	if m.Source == "" {
		return fmt.Errorf("observer binary missing: %s (supply --source with the extracted release after upgrading from a release without the observer)", observerBinary)
	}
	b, e := os.ReadFile(filepath.Join(m.Source, "hostlens-docker-observer"))
	if e != nil {
		return fmt.Errorf("source observer binary missing: %w", e)
	}
	r.Owned = true
	r.State = "intent"
	r.Hash = digest(b)
	if e = m.save(man); e != nil {
		return e
	}
	if e = token.Atomic(m.path(observerBinary), b, 0755); e != nil {
		return fmt.Errorf("partial reconciliation (%s): %w", observerBinary, e)
	}
	r.State = "complete"
	return m.save(man)
}

// reconcileEnabled provisions the observer identity, binary, units, and
// socket activation. It never touches Docker.
func (m *Manager) reconcileEnabled(ctx context.Context, man *Manifest) error {
	if e := m.reconcileIdentity(ctx, man); e != nil {
		return e
	}
	if e := m.checkIdentities(ctx, false); e != nil {
		return e
	}
	if e := m.reconcileBinary(ctx, man); e != nil {
		return e
	}
	units := map[string][]byte{
		"/etc/systemd/system/" + observerSvc:  []byte(ObserverUnit(m.Config)),
		"/etc/systemd/system/" + observerSock: []byte(ObserverSocketUnit(m.Config)),
	}
	socketChanged := false
	for _, unit := range []string{"/etc/systemd/system/" + observerSock, "/etc/systemd/system/" + observerSvc} {
		r := upsertResource(man, unit, "file")
		body := units[unit]
		if r.State != "complete" || m.pathContentChanged(r, body) {
			r.State = "intent"
			r.Owned = true
			if e := m.save(man); e != nil {
				return e
			}
			if e := token.Atomic(m.path(unit), body, 0644); e != nil {
				return fmt.Errorf("partial reconciliation (%s): %w", unit, e)
			}
			r.Hash = digest(body)
			r.State = "complete"
			if unit == "/etc/systemd/system/"+observerSock {
				socketChanged = true
			}
			if e := m.save(man); e != nil {
				return e
			}
		}
	}
	// Superseded socket resources from a previous configured path must not
	// survive an enabled reconciliation.
	for i := range man.Resources {
		r := &man.Resources[i]
		if r.Kind == "state" && strings.HasSuffix(r.Path, "docker-observer.sock") && r.Path != m.Config.Docker.ObserverSocket && r.Owned && r.State != "removed" {
			if e := os.Remove(m.path(r.Path)); e != nil && !errors.Is(e, os.ErrNotExist) {
				return fmt.Errorf("superseded observer socket removal failed (%s): %w", r.Path, e)
			}
			r.State = "removed"
		}
	}
	// The live-path socket record is re-adopted regardless of a prior
	// removal so disable-and-re-enable cycles report honestly afterwards.
	socketRecord := upsertResource(man, m.Config.Docker.ObserverSocket, "state")
	if socketRecord.State == "" || socketRecord.State == "removed" {
		socketRecord.State = "complete"
	}
	if e := m.save(man); e != nil {
		return e
	}
	if _, e := m.command(ctx, "systemctl", "daemon-reload"); e != nil {
		return e
	}
	if _, e := m.command(ctx, "systemctl", "enable", "--now", observerSock); e != nil {
		return fmt.Errorf("observer socket activation failed: %w", e)
	}
	if socketChanged {
		// enable --now does not restart a listening unit; a moved IPC path
		// must rebind or the backend dials the superseded socket forever.
		if _, e := m.command(ctx, "systemctl", "restart", observerSock); e != nil {
			return fmt.Errorf("observer socket restart failed: %w", e)
		}
	}
	return nil
}

// reconcileDisabled stops and disables the observer, removes owned units and
// the socket, and leaves the dormant identity without Docker group
// authority. Uncertain ownership is preserved for administrator review.
func (m *Manager) reconcileDisabled(ctx context.Context, man *Manifest) error {
	changed := 0
	for _, unit := range []string{observerSvc, observerSock} {
		r := findResource(man, "/etc/systemd/system/"+unit, "file")
		if r == nil || !r.Owned || r.State == "removed" {
			continue
		}
		if _, e := os.Lstat(m.path(r.Path)); e == nil {
			if _, e = m.command(ctx, "systemctl", "disable", "--now", unit); e != nil {
				return fmt.Errorf("observer disablement failed: %w", e)
			}
			if e = os.Remove(m.path(r.Path)); e != nil && !errors.Is(e, os.ErrNotExist) {
				return fmt.Errorf("observer unit removal failed (%s): %w", unit, e)
			}
		}
		r.State = "removed"
		changed++
	}
	// Any HostLens-owned observer socket record is removed, including paths
	// superseded by a configuration change.
	for i := range man.Resources {
		r := &man.Resources[i]
		if r.Kind == "state" && strings.HasSuffix(r.Path, "docker-observer.sock") && r.Owned && r.State != "removed" {
			if _, e := os.Lstat(m.path(r.Path)); !errors.Is(e, os.ErrNotExist) {
				if e = os.Remove(m.path(r.Path)); e != nil && !errors.Is(e, os.ErrNotExist) {
					return fmt.Errorf("observer resource removal failed (%s): %w", r.Path, e)
				}
			}
			r.State = "removed"
		}
	}
	for _, spec := range []struct{ path, kind string }{
		{m.Config.Docker.ObserverSocket, "state"},
		{observerBinary, "file"},
	} {
		r := findResource(man, spec.path, spec.kind)
		if r == nil || !r.Owned || r.State == "removed" {
			continue
		}
		if _, e := os.Lstat(m.path(spec.path)); errors.Is(e, os.ErrNotExist) {
			r.State = "removed"
			changed++
			continue
		}
		if spec.kind == "file" {
			b, e := os.ReadFile(m.path(spec.path))
			if e != nil || r.Hash == "" || digest(b) != r.Hash {
				// Uncertain ownership blocks removal and stays for review.
				r.State = "preserved"
				continue
			}
		}
		if e := os.Remove(m.path(spec.path)); e != nil && !errors.Is(e, os.ErrNotExist) {
			return fmt.Errorf("observer resource removal failed (%s): %w", spec.path, e)
		}
		r.State = "removed"
		changed++
	}
	if changed > 0 {
		if _, e := m.command(ctx, "systemctl", "daemon-reload"); e != nil {
			return e
		}
	}
	return m.save(man)
}

func (m *Manager) reconcile(ctx context.Context, man *Manifest) error {
	if m.Config.Docker.Enabled {
		if e := m.reconcileEnabled(ctx, man); e != nil {
			return e
		}
	} else if e := m.reconcileDisabled(ctx, man); e != nil {
		return e
	}
	man.Release = contract.Version
	return m.save(man)
}

func (m *Manager) pathContentChanged(r *Resource, body []byte) bool {
	if r.Hash == "" {
		return true
	}
	b, e := os.ReadFile(m.path(r.Path))
	if e != nil {
		return true
	}
	return digest(b) != digest(body)
}
