//go:build linux

package lifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/core"
	"go.yaml.in/yaml/v3"
)

func releaseFixture(t *testing.T) (Manager, string) {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	source := filepath.Join(t.TempDir(), "release")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(source, "hostlens")
	build := exec.Command("go", "build", "-trimpath", "-ldflags", "-X github.com/Monska85/hostlens/internal/app.Version=0.6.0", "-o", binary, "./cmd/hostlens")
	build.Dir = repo
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building release fixture: %v: %s", err, output)
	}
	files := map[string]string{"config.example.yaml": "packaging/config.yaml"}
	for _, name := range profileNames {
		files["profiles/"+name] = "packaging/profiles/" + name
	}
	for _, name := range unitNames {
		files["systemd/"+name] = "internal/uninstall/systemd/" + name
	}
	for member, original := range files {
		destination := filepath.Join(source, member)
		if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(filepath.Join(repo, original))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	checksums := map[string]string{}
	members := []string{"hostlens"}
	for name := range files {
		members = append(members, name)
	}
	for _, member := range members {
		content, err := os.ReadFile(filepath.Join(source, member))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(content)
		checksums[member] = hex.EncodeToString(digest[:])
	}
	manifestData, err := json.Marshal(manifest{Version: "0.6.0", Schema: 2, Architecture: runtime.GOARCH, Checksums: checksums})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "release.json"), manifestData, 0600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "root")
	for _, target := range []string{"usr/local/bin", "etc/systemd/system"} {
		if err := os.MkdirAll(filepath.Join(root, target), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return Manager{Source: source, Root: root}, root
}

func TestPreviewRejectsChangedReleaseAndTarget(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("root-owned paths require the disposable root check container")
	}
	manager, root := releaseFixture(t)
	plan, err := manager.Preview("install", true)
	if err != nil || plan.Digest == "" || len(plan.Create) == 0 || len(plan.Start) != 1 {
		t.Fatalf("fresh preview: %+v, %v", plan, err)
	}
	if _, err := os.Stat(filepath.Join(root, "etc/hostlens")); !os.IsNotExist(err) {
		t.Fatal("preview changed the target")
	}
	profile := filepath.Join(manager.Source, "profiles/host.yaml")
	original, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, append(original, byte('\n')), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Preview("install", true); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("changed release accepted: %v", err)
	}
	if err := os.WriteFile(profile, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "etc/hostlens"), 0750); err != nil {
		t.Fatal(err)
	}
	changed, err := manager.Preview("install", true)
	if err != nil || changed.Digest == plan.Digest {
		t.Fatalf("changed plan accepted: %+v, %v", changed, err)
	}
	if err := os.Symlink(manager.Source, filepath.Join(root, "etc/hostlens/profiles")); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Preview("install", true); err == nil {
		t.Fatal("symlinked destination accepted")
	}
}

func TestUnitProvenanceAcceptsPriorReleaseAndRejectsModifiedUnit(t *testing.T) {
	old := map[string][]byte{}
	for _, name := range unitNames {
		old["systemd/"+name] = []byte("old packaged " + name)
	}
	record := packagedUnitProvenance(old)
	for _, name := range unitNames {
		if !record.matches(name, old["systemd/"+name]) {
			t.Fatalf("pristine previous unit refused: %s", name)
		}
	}
	if record.matches(unitNames[0], []byte("new packaged "+unitNames[0])) {
		t.Fatal("different release unit accepted as pristine previous unit")
	}
	if record.matches(unitNames[0], append(old["systemd/"+unitNames[0]], '\n')) {
		t.Fatal("administrator-modified unit accepted")
	}
}

func TestConfiguredTemplateUsesAssignedIdentities(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	template, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../packaging/config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	template = []byte(strings.Replace(string(template), "gateway_uid: 1001", "gateway_uid: 99999", 1))
	output, err := configuredTemplate(template, 2201, 2202, 2301, 2302)
	if err != nil {
		t.Fatal(err)
	}
	var config core.Config
	if err := yaml.Unmarshal(output, &config); err != nil {
		t.Fatal(err)
	}
	if config.GatewayUID != 2201 || config.ObserverUID != 2202 || config.SharedGID != 2301 || config.TokenGID != 2302 {
		t.Fatalf("installed service identities differ from created accounts: %+v", config)
	}
	if !strings.Contains(string(output), "active_profiles: [host]") || !strings.Contains(string(output), "# Set these numeric IDs") {
		t.Fatal("installer changed the documented template layout")
	}
	if _, err := configuredTemplate(append(template, []byte("\nunknown: value\n")...), 2201, 2202, 2301, 2302); err == nil {
		t.Fatal("unknown release configuration field accepted")
	}
}

func TestUnitStateQueryFailsClosed(t *testing.T) {
	bin := t.TempDir()
	command := filepath.Join(bin, "systemctl")
	if err := os.WriteFile(command, []byte("#!/bin/sh\ncase \"$HOSTLENS_UNIT_STATE\" in error) exit 1;; *) printf '%s\\n' \"$HOSTLENS_UNIT_STATE\";; esac\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	for _, test := range []struct {
		state  string
		active bool
		failed bool
	}{{"active", true, false}, {"inactive", false, false}, {"error", false, true}, {"unexpected", false, true}} {
		t.Setenv("HOSTLENS_UNIT_STATE", test.state)
		active, err := unitActive(context.Background(), "hostlens-gateway.service")
		if active != test.active || (err != nil) != test.failed {
			t.Fatalf("state %s: active=%t err=%v", test.state, active, err)
		}
	}
}

func TestPreviewIncludesAdditionalPackagedProfile(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("root-owned paths require the disposable root check container")
	}
	manager, _ := releaseFixture(t)
	name := "profiles/future-diagnostics.yaml"
	data := []byte("allow: []\n")
	if err := os.WriteFile(filepath.Join(manager.Source, name), data, 0600); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(manager.Source, "release.json")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var release manifest
	if err := json.Unmarshal(content, &release); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	release.Checksums[name] = hex.EncodeToString(digest[:])
	content, err = json.Marshal(release)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, content, 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := manager.Preview("install", false)
	if err != nil || !strings.Contains(strings.Join(plan.Create, " "), "future-diagnostics.yaml") {
		t.Fatalf("additional packaged profile omitted: %+v, %v", plan, err)
	}
}

func TestInstallPreviewAcceptsProvenanceRetainedAfterUninstall(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("root-owned paths require the disposable root check container")
	}
	manager, root := releaseFixture(t)
	directory := filepath.Join(root, "etc/hostlens")
	if err := os.Mkdir(directory, 0750); err != nil {
		t.Fatal(err)
	}
	content, err := json.Marshal(unitProvenance{Checksums: map[string]string{
		unitNames[0]: strings.Repeat("a", 64), unitNames[1]: strings.Repeat("b", 64), unitNames[2]: strings.Repeat("c", 64),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "unit-provenance.json"), content, 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := manager.Preview("install", false)
	if err != nil || !strings.Contains(strings.Join(plan.Replace, " "), provenancePath) {
		t.Fatalf("reinstall preview: %+v, %v", plan, err)
	}
}
