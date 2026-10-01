package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/config"
)

func enabledDockerConfig(t *testing.T) config.Config {
	t.Helper()
	c := config.DefaultsLinux(true)
	c.Docker.Enabled = true
	if e := config.ValidateLinux(c); e != nil {
		t.Fatal(e)
	}
	return c
}

// reconcileFixture installs a baseline HostLens system before reconciling.
func reconcileFixture(t *testing.T, cfg config.Config) (Manager, string, *systemFixture) {
	t.Helper()
	m, source, sys := setup(t)
	if e := m.Install(context.Background(), source, false); e != nil {
		t.Fatal(e)
	}
	m.Config = cfg
	return m, source, sys
}

func TestReconcilePlanDoesNotMutate(t *testing.T) {
	m, _, sys := reconcileFixture(t, enabledDockerConfig(t))
	before, e := m.Load()
	if e != nil {
		t.Fatal(e)
	}
	callsBefore := len(sys.calls)
	_, report, e := m.Reconcile(context.Background(), false)
	if e != nil {
		t.Fatal(e)
	}
	if report.Desired == "disabled" {
		t.Fatal("desired state must reflect enabled configuration")
	}
	if report.Observer {
		t.Fatal("plan must not claim provisioning before apply")
	}
	after, e := m.Load()
	if e != nil {
		t.Fatal(e)
	}
	if len(before.Resources) != len(after.Resources) {
		t.Fatal("plan preview mutated the manifest")
	}
	for _, unit := range []string{"hostlens-docker-observer.service", "hostlens-docker-observer.socket"} {
		if _, e := os.Stat(m.path("/etc/systemd/system/" + unit)); !errors.Is(e, os.ErrNotExist) {
			t.Fatal("plan preview created unit")
		}
	}
	for _, call := range sys.calls[callsBefore:] {
		if stringsContainsAny(call, "groupadd", "useradd", "enable", "daemon-reload") {
			t.Fatal("plan preview mutated the system", call)
		}
	}
}

func TestReconcileProvisionsObserverWithoutTouchingDocker(t *testing.T) {
	m, _, sys := reconcileFixture(t, enabledDockerConfig(t))
	_, report, e := m.Reconcile(context.Background(), true)
	if e != nil {
		t.Fatal(e)
	}
	if !report.Observer {
		t.Fatal("observer not provisioned")
	}
	for _, unit := range []string{"hostlens-docker-observer.service", "hostlens-docker-observer.socket"} {
		if _, e := os.Stat(m.path("/etc/systemd/system/" + unit)); e != nil {
			t.Fatalf("unit missing: %v", e)
		}
	}
	if _, e := os.Stat(m.path(observerBinary)); e != nil {
		t.Fatalf("binary missing: %v", e)
	}
	man, e := m.Load()
	if e != nil {
		t.Fatal(e)
	}
	owned := map[string]bool{}
	for _, r := range man.Resources {
		if stringsContainsAny(r.Path, "hostlens-docker-observer", "hostlens-observer") && !r.Owned {
			t.Fatal("observer resource lacks ownership record", r.Path)
		}
		owned[r.Path] = true
	}
	for _, required := range []string{observerBinary, "/etc/systemd/system/hostlens-docker-observer.service", "/etc/systemd/system/hostlens-docker-observer.socket", "hostlens-observer"} {
		if !owned[required] {
			t.Fatal("missing ownership record", required)
		}
	}
	for _, call := range sys.calls {
		// HostLens never controls Docker through reconciliation.
		if call == "systemctl start docker.service" || call == "systemctl stop docker.service" || call == "systemctl restart docker.service" {
			t.Fatal("reconciliation controlled Docker", call)
		}
	}
	unit, _ := os.ReadFile(m.path("/etc/systemd/system/hostlens-docker-observer.service"))
	for _, required := range []string{"Requisite=docker.service", "After=docker.service", "PartOf=docker.service", "SupplementaryGroups=docker", "User=hostlens-observer"} {
		if !strings.Contains(string(unit), required) {
			t.Fatalf("observer unit missing %s", required)
		}
	}
	if strings.Contains(string(unit), "ConditionPathIsSocket") {
		t.Fatal("observer unit uses unsupported ConditionPathIsSocket")
	}
	socket, _ := os.ReadFile(m.path("/etc/systemd/system/hostlens-docker-observer.socket"))
	for _, required := range []string{"SocketUser=hostlens-diagnostics", "SocketGroup=hostlens-gateway", "SocketMode=0660"} {
		if !strings.Contains(string(socket), required) {
			t.Fatalf("socket unit missing %s", required)
		}
	}
}

func TestReconcileDisableRemovesOwnedObserverResources(t *testing.T) {
	m, _, _ := reconcileFixture(t, enabledDockerConfig(t))
	if _, _, e := m.Reconcile(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	disabled := enabledDockerConfig(t)
	disabled.Docker.Enabled = false
	if e := config.ValidateLinux(disabled); e != nil {
		t.Fatal(e)
	}
	m.Config = disabled
	_, report, e := m.Reconcile(context.Background(), true)
	if e != nil {
		t.Fatal(e)
	}
	if report.Observer {
		t.Fatal("observer still provisioned")
	}
	for _, unit := range []string{"hostlens-docker-observer.service", "hostlens-docker-observer.socket"} {
		if _, e := os.Stat(m.path("/etc/systemd/system/" + unit)); !errors.Is(e, os.ErrNotExist) {
			t.Fatal("observer unit preserved after disablement")
		}
	}
	if _, e := os.Stat(m.path(observerBinary)); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("owned observer binary preserved after disablement")
	}
	man, e := m.Load()
	if e != nil {
		t.Fatal(e)
	}
	// Dormant identity records stay; they hold no Docker group authority.
	for _, r := range man.Resources {
		if r.Path == "hostlens-observer" && r.State == "removed" {
			t.Fatal("dormant identity record removed unexpectedly")
		}
	}
}

func TestReconcileMovesObserverSocketPath(t *testing.T) {
	m, _, sys := reconcileFixture(t, enabledDockerConfig(t))
	if _, _, e := m.Reconcile(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	// Move the IPC path and rerun: the socket unit is rewritten, the socket
	// unit is restarted so the new path binds, and the superseded socket
	// record and file are removed.
	callsBefore := len(sys.calls)
	moved := enabledDockerConfig(t)
	moved.Docker.ObserverSocket = "/run/hostlens/docker-observer-moved.sock"
	m.Config = moved
	_, report, e := m.Reconcile(context.Background(), false)
	if e != nil {
		t.Fatal(e)
	}
	foundMove := false
	for _, change := range report.Changed {
		if change.State == "remove" || change.State == "create" {
			foundMove = true
		}
	}
	if !foundMove {
		t.Fatalf("moved socket path not disclosed: %+v", report.Changed)
	}
	if _, _, e := m.Reconcile(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	unit, e := os.ReadFile(m.path("/etc/systemd/system/hostlens-docker-observer.socket"))
	if e != nil || !strings.Contains(string(unit), "docker-observer-moved.sock") {
		t.Fatal("socket unit not moved")
	}
	man, e := m.Load()
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range man.Resources {
		if r.Kind == "state" && strings.HasSuffix(r.Path, "docker-observer.sock") && r.Path != moved.Docker.ObserverSocket && r.State != "removed" {
			t.Fatalf("superseded socket record kept: %s %s", r.Path, r.State)
		}
	}
	restarted := false
	for _, call := range sys.calls[callsBefore:] {
		if strings.Contains(call, "restart hostlens-docker-observer.socket") {
			restarted = true
		}
	}
	if !restarted {
		t.Fatal("socket unit not restarted after the path move")
	}
	// The applied report must describe the actual move: the new path
	// adopted, the superseded path removed, and a re-run plan empty.
	man, e = m.Load()
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range man.Resources {
		if r.Path == moved.Docker.ObserverSocket && r.Kind == "state" && r.State != "complete" {
			t.Fatalf("new socket record not adopted: %s %s", r.Path, r.State)
		}
	}
	_, report, e = m.Reconcile(context.Background(), false)
	if e != nil {
		t.Fatal(e)
	}
	if len(report.Changed) != 0 {
		t.Fatalf("re-run after a completed move disclosed phantom changes: %+v", report.Changed)
	}
	// A second move must remove the previous custom path as well.
	m.Config.Docker.ObserverSocket = "/run/hostlens/another-observer-path.sock"
	if _, _, e := m.Reconcile(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	man, e = m.Load()
	if e != nil {
		t.Fatal(e)
	}
	if r := findResource(&man, moved.Docker.ObserverSocket, "state"); r == nil || r.State != "removed" {
		t.Fatalf("superseded custom socket retained: %+v", r)
	}
}

func TestReconcileDisableAndReEnableReportsHonestState(t *testing.T) {
	m, source, _ := reconcileFixture(t, enabledDockerConfig(t))
	m.Source = source
	if _, _, e := m.Reconcile(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	disabled := enabledDockerConfig(t)
	disabled.Docker.Enabled = false
	if e := config.ValidateLinux(disabled); e != nil {
		t.Fatal(e)
	}
	m.Config = disabled
	if _, _, e := m.Reconcile(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	// Re-enable: recreation after a real disablement is disclosed honestly,
	// and the applied report converges to an empty plan.
	m.Config = enabledDockerConfig(t)
	if _, _, e := m.Reconcile(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	_, report, e := m.Reconcile(context.Background(), false)
	if e != nil {
		t.Fatal(e)
	}
	if len(report.Changed) != 0 {
		t.Fatalf("re-enable left phantom disclosures: %+v", report.Changed)
	}
	man, e := m.Load()
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range man.Resources {
		if r.Kind == "state" && r.Path == m.Config.Docker.ObserverSocket && r.State != "complete" {
			t.Fatalf("socket record not re-adopted after re-enable: %s %s", r.Path, r.State)
		}
	}
}

func TestReconcileRejectsPreexistingObserverIdentity(t *testing.T) {
	m, _, sys := reconcileFixture(t, enabledDockerConfig(t))
	m.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "getent" && args[0] == "group" && args[1] == "hostlens-observer" {
			return []byte("hostlens-observer:x:900:\n"), nil
		}
		return sys.run(ctx, name, args...)
	}
	_, _, e := m.Reconcile(context.Background(), true)
	if e == nil || !strings.Contains(e.Error(), "pre-existing observer identity") {
		t.Fatalf("conflict ignored: %v", e)
	}
	if _, e := os.Stat(m.path("/etc/systemd/system/hostlens-docker-observer.service")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("units written despite identity conflict")
	}
}

func TestReconcileRejectsUnrecordedObserverResourcesBeforeMutation(t *testing.T) {
	for _, path := range []string{
		observerBinary,
		"/etc/systemd/system/" + observerSvc,
		"/etc/systemd/system/" + observerSock,
		"/run/hostlens/docker-observer.sock",
	} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			m, _, sys := reconcileFixture(t, enabledDockerConfig(t))
			man, err := m.Load()
			if err != nil {
				t.Fatal(err)
			}
			for i := len(man.Resources) - 1; i >= 0; i-- {
				if man.Resources[i].Path == path {
					man.Resources = append(man.Resources[:i], man.Resources[i+1:]...)
				}
			}
			if err := m.save(&man); err != nil {
				t.Fatal(err)
			}
			if path != observerBinary {
				if err := os.WriteFile(m.path(path), []byte("unowned unit"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(m.path(ManifestPath))
			if err != nil {
				t.Fatal(err)
			}
			calls := len(sys.calls)
			for _, apply := range []bool{false, true} {
				if _, _, err := m.Reconcile(context.Background(), apply); err == nil || !strings.Contains(err.Error(), "lacks ownership") {
					t.Fatalf("apply=%t accepted unowned path: %v", apply, err)
				}
			}
			if len(sys.calls) != calls {
				t.Fatalf("conflict reached service or identity commands: %v", sys.calls[calls:])
			}
			after, err := os.ReadFile(m.path(ManifestPath))
			if err != nil || string(after) != string(before) {
				t.Fatal("conflict changed manifest", err)
			}
		})
	}
}

func TestReconcileRejectsResourceReplacedAfterDisablement(t *testing.T) {
	m, source, sys := reconcileFixture(t, enabledDockerConfig(t))
	m.Source = source
	if _, _, err := m.Reconcile(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	m.Config.Docker.Enabled = false
	if _, _, err := m.Reconcile(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.path(observerBinary), []byte("unowned executable"), 0755); err != nil {
		t.Fatal(err)
	}
	m.Config.Docker.Enabled = true
	calls := len(sys.calls)
	for _, apply := range []bool{false, true} {
		if _, _, err := m.Reconcile(context.Background(), apply); err == nil || !strings.Contains(err.Error(), "lacks ownership") {
			t.Fatalf("apply=%t accepted replaced binary: %v", apply, err)
		}
	}
	if len(sys.calls) != calls {
		t.Fatal("replaced binary reached service or identity commands")
	}
}

func TestReconcileRejectsReplacedOwnedObserverFiles(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		replace    func(string) error
	}{
		{"symlink binary", observerBinary, func(path string) error { return os.Symlink("/etc/passwd", path) }},
		{"directory unit", "/etc/systemd/system/" + observerSvc, func(path string) error { return os.Mkdir(path, 0755) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, source, sys := reconcileFixture(t, enabledDockerConfig(t))
			m.Source = source
			if _, _, err := m.Reconcile(context.Background(), true); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(m.path(tc.path)); err != nil {
				t.Fatal(err)
			}
			if err := tc.replace(m.path(tc.path)); err != nil {
				t.Fatal(err)
			}
			calls := len(sys.calls)
			if _, _, err := m.Reconcile(context.Background(), true); err == nil || !strings.Contains(err.Error(), "unexpected observer resource type") {
				t.Fatalf("replaced resource accepted: %v", err)
			}
			if len(sys.calls) != calls {
				t.Fatal("replaced resource reached mutation commands")
			}
		})
	}
}

func TestReconcileMissingBinaryNeedsSourceBeforeMutation(t *testing.T) {
	m, _, sys := reconcileFixture(t, enabledDockerConfig(t))
	if err := os.Remove(m.path(observerBinary)); err != nil {
		t.Fatal(err)
	}
	calls := len(sys.calls)
	if _, _, err := m.Reconcile(context.Background(), true); err == nil || !strings.Contains(err.Error(), "supply --source") {
		t.Fatalf("missing binary accepted: %v", err)
	}
	if len(sys.calls) != calls {
		t.Fatal("missing binary reached mutation commands")
	}
}

func TestReconcileRejectsOversizedSourceBeforeMutation(t *testing.T) {
	m, source, sys := reconcileFixture(t, enabledDockerConfig(t))
	if err := os.Remove(m.path(observerBinary)); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(filepath.Join(source, "hostlens-docker-observer"), (64<<20)+1); err != nil {
		t.Fatal(err)
	}
	m.Source = source
	calls := len(sys.calls)
	if _, _, err := m.Reconcile(context.Background(), true); err == nil || !strings.Contains(err.Error(), "64 MiB") {
		t.Fatalf("oversized source accepted: %v", err)
	}
	if len(sys.calls) != calls {
		t.Fatal("oversized source reached mutation commands")
	}
}

func TestReconcileResumesMatchingIntentResources(t *testing.T) {
	m, _, _ := reconcileFixture(t, enabledDockerConfig(t))
	man, err := m.Load()
	if err != nil {
		t.Fatal(err)
	}
	for i := range man.Resources {
		if man.Resources[i].Path == observerBinary {
			man.Resources[i].State = "intent"
		}
	}
	unit := "/etc/systemd/system/" + observerSvc
	if err := os.WriteFile(m.path(unit), []byte(ObserverUnit(m.Config)), 0644); err != nil {
		t.Fatal(err)
	}
	man.Resources = append(man.Resources, Resource{Path: unit, Kind: "file", Owned: true, State: "intent"})
	if err := m.save(&man); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Reconcile(context.Background(), true); err != nil {
		t.Fatalf("matching interrupted resources could not resume: %v", err)
	}
	man, err = m.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{observerBinary, unit} {
		r := findResource(&man, path, "file")
		if r == nil || r.State != "complete" {
			t.Fatalf("resource did not complete: %s %+v", path, r)
		}
	}
}

func TestDisablePreservesReplacedObserverResources(t *testing.T) {
	m, _, sys := reconcileFixture(t, enabledDockerConfig(t))
	if _, _, err := m.Reconcile(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	unit := "/etc/systemd/system/" + observerSvc
	if err := os.WriteFile(m.path(unit), []byte("unrelated service"), 0644); err != nil {
		t.Fatal(err)
	}
	socket := m.Config.Docker.ObserverSocket
	if err := os.WriteFile(m.path(socket), []byte("unrelated endpoint"), 0644); err != nil {
		t.Fatal(err)
	}
	m.Config.Docker.Enabled = false
	if _, _, err := m.Reconcile(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{unit, socket} {
		if _, err := os.Lstat(m.path(path)); err != nil {
			t.Fatalf("replaced path removed: %s: %v", path, err)
		}
	}
	man, err := m.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ path, kind string }{{unit, "file"}, {socket, "state"}} {
		r := findResource(&man, item.path, item.kind)
		if r == nil || r.State != "preserved" {
			t.Fatalf("replaced path not preserved: %s %+v", item.path, r)
		}
	}
	if !stringsContainsAny(strings.Join(sys.calls, "\n"), "disable --now "+observerSvc) {
		t.Fatal("observer service was not disabled")
	}
}

func TestReconcileSuppliesObserverBinaryFromSource(t *testing.T) {
	m, source, sys := reconcileFixture(t, enabledDockerConfig(t))
	if e := os.Remove(m.path(observerBinary)); e != nil {
		t.Fatal(e)
	}
	// An upgrade from a release without the observer leaves no binary;
	// reconciliation restores it from the release source with ownership.
	man, e := m.Load()
	if e != nil {
		t.Fatal(e)
	}
	m.Source = source
	_, report, e := m.Reconcile(context.Background(), true)
	if e != nil {
		t.Fatal(e)
	}
	if !report.Observer {
		t.Fatal("observer not provisioned from source")
	}
	st, err := os.Stat(m.path(observerBinary))
	if err != nil || st.Mode().Perm() != 0755 {
		t.Fatalf("observer binary not restored: %v", err)
	}
	found := false
	man, _ = m.Load()
	for _, r := range man.Resources {
		if r.Path == observerBinary && r.Owned && r.State == "complete" && r.Hash != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("observer binary lacks ownership record")
	}
	_ = sys
}

func TestReconcileWithoutManifestRefused(t *testing.T) {
	m, _, _ := setup(t)
	m.Config = enabledDockerConfig(t)
	if _, _, e := m.Reconcile(context.Background(), false); e == nil {
		t.Fatal("reconciled without installation")
	}
}

func TestObserverUnitsDeclareDependencyDirection(t *testing.T) {
	c := config.DefaultsLinux(true)
	c.Docker.Enabled = true
	socket := ObserverSocketUnit(c)
	for _, required := range []string{"ListenStream=" + c.Docker.ObserverSocket, "SocketUser=hostlens-diagnostics", "SocketMode=0660", "RemoveOnStop=yes"} {
		if !strings.Contains(socket, required) {
			t.Fatalf("socket unit missing %s", required)
		}
	}
	if !strings.Contains(ObserverUnit(c), "SupplementaryGroups=docker") {
		t.Fatal("observer unit misses process-scoped Docker group")
	}
}

func stringsContainsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
