package lifecycle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/config"
)

type absentIdentity struct{}

func (absentIdentity) Error() string { return "not found" }
func (absentIdentity) ExitCode() int { return 2 }

type systemFixture struct {
	failed string
	calls  []string
	users  map[string]int
	groups map[string]int
}

func (s *systemFixture) run(_ context.Context, name string, args ...string) ([]byte, error) {
	s.calls = append(s.calls, name+" "+strings.Join(args, " "))
	if name == "getent" {
		if args[0] == "passwd" {
			if uid, ok := s.users[args[1]]; ok {
				return []byte(fmt.Sprintf("%s:x:%d:%d::/nonexistent:/usr/sbin/nologin\n", args[1], uid, uid)), nil
			}
		} else if gid, ok := s.groups[args[1]]; ok {
			return []byte(fmt.Sprintf("%s:x:%d:\n", args[1], gid)), nil
		}
		return nil, absentIdentity{}
	}
	if name == "systemctl" && len(args) > 1 && args[0] == "show" && args[1] == "--property=ActiveState" {
		return []byte("inactive\n"), nil
	}
	if name == "systemctl" && len(args) > 0 && args[0] == "is-active" {
		return nil, errors.New("not found or inactive")
	}
	if strings.Contains(name+" "+strings.Join(args, " "), s.failed) && s.failed != "" {
		return nil, errors.New("injected lifecycle failure")
	}
	identity := args[len(args)-1]
	switch name {
	case "useradd":
		s.users[identity] = 200 + len(s.users)
	case "groupadd":
		s.groups[identity] = 200 + len(s.groups)
	case "userdel":
		delete(s.users, identity)
	case "groupdel":
		delete(s.groups, identity)
	}
	return nil, nil
}
func setup(t *testing.T) (Manager, string, *systemFixture) {
	root := t.TempDir()
	source := t.TempDir()
	for _, p := range []string{"usr/local/bin", "etc/systemd/system"} {
		if err := os.MkdirAll(filepath.Join(root, p), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(source, "profiles"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hostlens", "hostlens-diagnostics"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte("old executable"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"nginx", "allow-all"} {
		if err := os.WriteFile(filepath.Join(source, "profiles", name+".yaml"), []byte("profiles: []\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	sys := &systemFixture{users: map[string]int{}, groups: map[string]int{}}
	return Manager{Root: root, Run: sys.run, Config: config.DefaultsLinux(true)}, source, sys
}
func TestInstallStoppedManifestAndUninstallPreservation(t *testing.T) {
	m, source, sys := setup(t)
	if e := m.Install(context.Background(), source, false); e != nil {
		t.Fatal(e)
	}
	for _, call := range sys.calls {
		if strings.Contains(call, "systemctl start") {
			t.Fatal("implicitly started service")
		}
	}
	man, e := m.Load()
	if e != nil || man.State != "installed" {
		t.Fatal(e, man.State)
	}
	if err := os.WriteFile(m.path("/etc/hostlens/unexpected.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if e = m.Uninstall(context.Background()); e == nil {
		t.Fatal("unexpected content not reported")
	}
	if b, e := os.ReadFile(m.path("/etc/hostlens/unexpected.txt")); e != nil || string(b) != "keep" {
		t.Fatal("unexpected file deleted")
	}
	if err := os.Remove(m.path("/etc/hostlens/unexpected.txt")); err != nil {
		t.Fatal(err)
	}
	if e = m.Uninstall(context.Background()); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(m.path(ManifestPath)); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("manifest retained")
	}
}
func TestInstallFailureDurablyTracksIntent(t *testing.T) {
	m, source, sys := setup(t)
	sys.failed = "useradd"
	if e := m.Install(context.Background(), source, false); e == nil {
		t.Fatal("injection ignored")
	}
	man, e := m.Load()
	if e != nil || man.State != "installing" {
		t.Fatal(e)
	}
	intent := false
	for _, r := range man.Resources {
		if r.Kind == "user" && r.State == "intent" {
			intent = true
		}
	}
	if !intent {
		t.Fatal("missing recovery intent")
	}
	if _, e = m.Plan(source); e == nil {
		t.Fatal("partial installation overwritten")
	}
}
func archive(t *testing.T, files map[string][]byte, mutate func(*Release)) string {
	t.Helper()
	rel := Release{Version: "candidate", Schema: 1, Architecture: runtime.GOARCH, Checksums: map[string]string{}}
	for name, b := range files {
		rel.Checksums[name] = digest(b)
	}
	if mutate != nil {
		mutate(&rel)
	}
	b, _ := json.Marshal(rel)
	files["release.json"] = b
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, b := range files {
		mode := int64(0644)
		if name == "hostlens" || name == "hostlens-diagnostics" {
			mode = 0755
		}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(b)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "release.tar.gz")
	if err := os.WriteFile(p, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestArchiveVerification(t *testing.T) {
	files := func() map[string][]byte {
		return map[string][]byte{"hostlens": []byte("binary"), "hostlens-diagnostics": []byte("backend")}
	}
	for _, tt := range []struct {
		name   string
		mutate func(*Release)
	}{{"valid", nil}, {"checksum", func(r *Release) { r.Checksums["hostlens"] = "bad" }}, {"schema", func(r *Release) { r.Schema = 2 }}, {"architecture", func(r *Release) { r.Architecture = "unsupported" }}} {
		t.Run(tt.name, func(t *testing.T) {
			_, e := Extract(archive(t, files(), tt.mutate), t.TempDir(), runtime.GOARCH)
			if (e == nil) != (tt.name == "valid") {
				t.Fatal(e)
			}
		})
	}
	bad := files()
	bad["../escape"] = []byte("bad")
	if _, e := Extract(archive(t, bad, nil), t.TempDir(), runtime.GOARCH); e == nil {
		t.Fatal("traversal accepted")
	}
}
func TestUpgradePreservesPolicyAndTracksPrevious(t *testing.T) {
	m, source, _ := setup(t)
	if e := m.Install(context.Background(), source, false); e != nil {
		t.Fatal(e)
	}
	legacyProfile := m.path("/etc/hostlens/profiles/web-common.yaml")
	if _, err := os.Stat(legacyProfile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fresh install created obsolete profile: %v", err)
	}
	customPolicy := []byte("allow:\n  files: [/srv/site/config.yaml]\n")
	if err := os.WriteFile(legacyProfile, customPolicy, 0644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(m.path("/etc/hostlens/config.yaml"))
	bundle := archive(t, map[string][]byte{"hostlens": []byte("new executable"), "hostlens-diagnostics": []byte("new backend"), "profiles/nginx.yaml": []byte("allow:\n  files: [/**]\n")}, nil)
	if e := m.Upgrade(context.Background(), bundle); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(m.path("/etc/hostlens/config.yaml"))
	if !bytes.Equal(before, after) {
		t.Fatal("configuration replaced")
	}
	profile, _ := os.ReadFile(m.path("/etc/hostlens/profiles/nginx.yaml"))
	if strings.Contains(string(profile), "/**") {
		t.Fatal("profile silently broadened")
	}
	if preserved, err := os.ReadFile(legacyProfile); err != nil || !bytes.Equal(preserved, customPolicy) {
		t.Fatalf("administrator profile changed during upgrade: %v", err)
	}
	if err := os.Remove(legacyProfile); err != nil {
		t.Fatal(err)
	}
	man, _ := m.Load()
	if man.State != "installed" || man.Release != "candidate" {
		t.Fatal(man)
	}
	if e := m.Uninstall(context.Background()); e != nil {
		t.Fatal(e)
	}
}
func TestContainmentUnits(t *testing.T) {
	c := config.DefaultsLinux(true)
	c.Server.TLS.KeyFile = "/opt/custom/key.pem"
	standard := DiagnosticsUnit(c)
	for _, required := range []string{"AmbientCapabilities=CAP_DAC_READ_SEARCH", "ProtectSystem=strict", "InaccessiblePaths=", "/opt/custom/key.pem", "RestrictAddressFamilies=AF_UNIX"} {
		if !strings.Contains(standard, required) {
			t.Fatalf("missing %s", required)
		}
	}
	c.Privilege = "restricted"
	if strings.Contains(DiagnosticsUnit(c), "CAP_DAC_READ_SEARCH") {
		t.Fatal("restricted capability")
	}
	if strings.Contains(GatewayUnit(), "CAP_DAC_READ_SEARCH") {
		t.Fatal("gateway privilege")
	}
}

func TestFailedActivationRestoresBinariesAndMetadata(t *testing.T) {
	m, source, sys := setup(t)
	if e := m.Install(context.Background(), source, false); e != nil {
		t.Fatal(e)
	}
	failed := false
	m.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "systemctl" && strings.HasPrefix(joined, "show --property=ActiveState") {
			return []byte("active\n"), nil
		}
		if name == "systemctl" && strings.HasPrefix(joined, "is-active") {
			return nil, nil
		}
		if name == "systemctl" && joined == "start hostlens-gateway.service" && !failed {
			failed = true
			return nil, errors.New("candidate failed activation")
		}
		return sys.run(ctx, name, args...)
	}
	bundle := archive(t, map[string][]byte{"hostlens": []byte("new executable"), "hostlens-diagnostics": []byte("new backend")}, nil)
	if e := m.Upgrade(context.Background(), bundle); e == nil {
		t.Fatal("activation failure hidden")
	}
	b, _ := os.ReadFile(m.path("/usr/local/bin/hostlens"))
	if string(b) != "old executable" {
		t.Fatal("previous executable not restored")
	}
	man, _ := m.Load()
	if man.State != "installed" {
		t.Fatal("rollback state not restored")
	}
	for _, r := range man.Resources {
		if r.Path == "/usr/local/bin/hostlens" && r.Hash != digest(b) {
			t.Fatal("rollback ownership hash not restored")
		}
	}
}

func TestPreexistingManagedDirectoryIsUntouched(t *testing.T) {
	m, source, _ := setup(t)
	path := m.path("/etc/hostlens")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := m.Install(context.Background(), source, false); err == nil {
		t.Fatal("adopted external directory")
	}
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm() != 0700 {
		t.Fatal("changed external directory", err)
	}
	if _, err := os.Stat(m.path(ManifestPath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("mutated before conflict")
	}
}

func TestInterruptedInstallPreservesUnvisitedIdentities(t *testing.T) {
	m, source, sys := setup(t)
	m.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "getent" && args[1] == "hostlens-diagnostics" {
			if args[0] == "passwd" {
				return []byte("hostlens-diagnostics:x:120:120::/nonexistent:/usr/sbin/nologin\n"), nil
			}
			return []byte("hostlens-diagnostics:x:120:\n"), nil
		}
		return sys.run(ctx, name, args...)
	}
	sys.failed = "useradd"
	if err := m.Install(context.Background(), source, false); err == nil {
		t.Fatal("failure ignored")
	}
	man, _ := m.Load()
	for _, r := range man.Resources {
		if r.Path == "hostlens-diagnostics" && r.Owned {
			t.Fatal("unvisited preexisting identity owned")
		}
	}
	// A later unrelated file at an unvisited path must not be removed.
	path := m.path("/etc/hostlens/config.yaml")
	if err := os.WriteFile(path, []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	sys.failed = ""
	if err := m.Uninstall(context.Background()); err == nil {
		t.Fatal("unexpected content not reported")
	}
	if b, _ := os.ReadFile(path); string(b) != "external" {
		t.Fatal("unvisited file removed")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := m.Uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, call := range sys.calls {
		if strings.Contains(call, "del hostlens-diagnostics") || strings.HasPrefix(call, "systemctl disable") {
			t.Fatal("mutated uncreated resource", call)
		}
	}
}

func TestUncertainIdentityAndExistingServiceFailuresPreserved(t *testing.T) {
	m, source, sys := setup(t)
	sys.failed = "useradd"
	if err := m.Install(context.Background(), source, false); err == nil {
		t.Fatal("failure ignored")
	}
	m.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "getent" && args[0] == "passwd" && args[1] == "hostlens-gateway" {
			return []byte("exists"), nil
		}
		return sys.run(ctx, name, args...)
	}
	if err := m.Uninstall(context.Background()); err == nil || !strings.Contains(err.Error(), "uncertain identity") {
		t.Fatal(err)
	}
	m, source, sys = setup(t)
	if err := m.Install(context.Background(), source, false); err != nil {
		t.Fatal(err)
	}
	sys.failed = "systemctl disable"
	if err := m.Uninstall(context.Background()); err == nil {
		t.Fatal("stop failure hidden")
	}
	if _, err := os.Stat(m.path("/usr/local/bin/hostlens")); err != nil {
		t.Fatal("cleanup ran while service might remain active")
	}
}

func TestCreatedUserCannotImplicitlyDeletePreexistingGroup(t *testing.T) {
	m, source, sys := setup(t)
	created := false
	m.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "getent" && args[1] == "hostlens-gateway" {
			if args[0] == "group" {
				return []byte("hostlens-gateway:x:120:\n"), nil
			}
			if created {
				return []byte("hostlens-gateway:x:121:120::/nonexistent:/usr/sbin/nologin\n"), nil
			}
		}
		if name == "useradd" && args[len(args)-1] == "hostlens-gateway" {
			created = true
		}
		return sys.run(ctx, name, args...)
	}
	if err := m.Install(context.Background(), source, false); err != nil {
		t.Fatal(err)
	}
	if err := m.Uninstall(context.Background()); err == nil || !strings.Contains(err.Error(), "same-name group") {
		t.Fatal(err)
	}
	for _, call := range sys.calls {
		if call == "userdel hostlens-gateway" || call == "groupdel hostlens-gateway" {
			t.Fatal("pre-existing group endangered", call)
		}
	}
}

func TestServiceIdentityUIDSeparation(t *testing.T) {
	for _, uids := range [][2]string{{"120", "120"}, {"00", "121"}, {"invalid", "121"}, {"120", "121"}} {
		t.Run(strings.Join(uids[:], "-"), func(t *testing.T) {
			m, source, sys := setup(t)
			m.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "getent" && args[0] == "passwd" {
					i := 0
					if args[1] == "hostlens-diagnostics" {
						i = 1
					}
					return []byte(args[1] + ":x:" + uids[i] + ":120::/nonexistent:/usr/sbin/nologin\n"), nil
				}
				return sys.run(ctx, name, args...)
			}
			_, err := m.Plan(source)
			valid := uids == [2]string{"120", "121"}
			if (err == nil) != valid {
				t.Fatalf("UIDs %v: %v", uids, err)
			}
		})
	}
}

func TestIdentityCollisionAfterCreationStopsBeforeUnits(t *testing.T) {
	m, source, sys := setup(t)
	m.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		b, err := sys.run(ctx, name, args...)
		if name == "useradd" && err == nil {
			sys.users[args[len(args)-1]] = 200
		}
		return b, err
	}
	if err := m.Install(context.Background(), source, false); err == nil || !strings.Contains(err.Error(), "share UID") {
		t.Fatalf("collision accepted: %v", err)
	}
	if _, err := os.Stat(m.path("/etc/systemd/system/hostlens-gateway.service")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("created runnable units before verifying identities", err)
	}
}

func TestUpgradeStopFailureRestoresRunningServices(t *testing.T) {
	m, source, sys := setup(t)
	if err := m.Install(context.Background(), source, false); err != nil {
		t.Fatal(err)
	}
	active := map[string]bool{"hostlens-gateway.service": true, "hostlens-diagnostics.service": true}
	stops := []string{}
	m.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "systemctl" {
			svc := args[len(args)-1]
			switch args[0] {
			case "show":
				if active[svc] {
					return []byte("active\n"), nil
				}
				return []byte("inactive\n"), nil
			case "is-active":
				if active[svc] {
					return nil, nil
				}
				return nil, errors.New("inactive")
			case "stop":
				stops = append(stops, svc)
				if svc == "hostlens-diagnostics.service" {
					return nil, errors.New("stop failed")
				}
				active[svc] = false
				return nil, nil
			case "restart", "start":
				active[svc] = true
				return nil, nil
			}
		}
		return sys.run(ctx, name, args...)
	}
	bundle := archive(t, map[string][]byte{"hostlens": []byte("new"), "hostlens-diagnostics": []byte("new backend")}, nil)
	if err := m.Upgrade(context.Background(), bundle); err == nil {
		t.Fatal("stop failure hidden")
	}
	if strings.Join(stops, ",") != "hostlens-gateway.service,hostlens-diagnostics.service" {
		t.Fatal(stops)
	}
	for svc, running := range active {
		if !running {
			t.Fatal("left stopped", svc)
		}
	}
	man, err := m.Load()
	if err != nil || man.State != "installed" {
		t.Fatal("recovery state", man.State, err)
	}
	b, err := os.ReadFile(m.path("/usr/local/bin/hostlens"))
	if err != nil || string(b) != "old executable" {
		t.Fatal("binary changed", err)
	}
}

func TestArchiveRejectsDamagedGzipTrailer(t *testing.T) {
	for _, missing := range []int{1, 8} {
		t.Run(strconv.Itoa(missing), func(t *testing.T) {
			p := archive(t, map[string][]byte{"hostlens": []byte("binary"), "hostlens-diagnostics": []byte("backend")}, nil)
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(p, b[:len(b)-missing], 0600); err != nil {
				t.Fatal(err)
			}
			if _, err = Extract(p, t.TempDir(), runtime.GOARCH); err == nil {
				t.Fatal("truncated gzip accepted")
			}
		})
	}
	p := archive(t, map[string][]byte{"hostlens": []byte("binary"), "hostlens-diagnostics": []byte("backend")}, nil)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	b[len(b)-8] ^= 0xff
	if err = os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Extract(p, t.TempDir(), runtime.GOARCH); err == nil {
		t.Fatal("bad gzip checksum accepted")
	}
}

func TestUpgradeUnknownStatePreservesInstallation(t *testing.T) {
	for _, state := range []string{"query-error", "", "activating", "garbage"} {
		t.Run(state, func(t *testing.T) {
			m, source, sys := setup(t)
			if err := m.Install(context.Background(), source, false); err != nil {
				t.Fatal(err)
			}
			m.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "systemctl" && args[0] == "show" && args[1] == "--property=ActiveState" {
					if state == "query-error" {
						return nil, errors.New("Failed to connect to bus")
					}
					return []byte(state), nil
				}
				return sys.run(ctx, name, args...)
			}
			bundle := archive(t, map[string][]byte{"hostlens": []byte("new"), "hostlens-diagnostics": []byte("new backend")}, nil)
			if err := m.Upgrade(context.Background(), bundle); err == nil {
				t.Fatal("unknown state accepted")
			}
			b, err := os.ReadFile(m.path("/usr/local/bin/hostlens"))
			if err != nil || string(b) != "old executable" {
				t.Fatal("binary replaced", err)
			}
			man, err := m.Load()
			if err != nil || man.State != "installed" {
				t.Fatal(man.State, err)
			}
			for _, r := range man.Resources {
				if strings.Contains(r.Path, "previous-") {
					t.Fatal("backup created before state validation")
				}
			}
		})
	}
}

func TestUninstallRetainsManifestForUnexpectedStateFile(t *testing.T) {
	m, source, _ := setup(t)
	if err := m.Install(context.Background(), source, false); err != nil {
		t.Fatal(err)
	}
	unexpected := m.path("/var/lib/hostlens/operator-note")
	if err := os.WriteFile(unexpected, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.Uninstall(context.Background()); err == nil {
		t.Fatal("unexpected state content hidden")
	}
	if _, err := m.Load(); err != nil {
		t.Fatal("retry manifest lost", err)
	}
	if err := os.Remove(unexpected); err != nil {
		t.Fatal(err)
	}
	if err := m.Uninstall(context.Background()); err != nil {
		t.Fatal("retry failed", err)
	}
}

func TestArchiveRejectsUnsafeHeaders(t *testing.T) {
	for _, header := range []tar.Header{
		{Name: "hostlens", Mode: 0644},
		{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"},
		{Name: "large", Size: (64 << 20) + 1},
		{Name: "/absolute"},
		{Name: "duplicate"},
	} {
		t.Run(header.Name, func(t *testing.T) {
			var buf bytes.Buffer
			gz := gzip.NewWriter(&buf)
			tw := tar.NewWriter(gz)
			if err := tw.WriteHeader(&header); err != nil {
				t.Fatal(err)
			}
			if header.Name == "duplicate" {
				if err := tw.WriteHeader(&header); err != nil {
					t.Fatal(err)
				}
			}
			// An oversized member is rejected from its header before its body is read.
			if header.Size == 0 {
				if err := tw.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := gz.Close(); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "unsafe.tar.gz")
			if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			want := "unsafe or duplicate"
			if header.Name == "hostlens" {
				want = "permissions"
			}
			if header.Name == "link" || header.Name == "large" {
				want = "bounded regular"
			}
			if _, err := Extract(path, t.TempDir(), runtime.GOARCH); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("expected %q, got %v", want, err)
			}
		})
	}
}
