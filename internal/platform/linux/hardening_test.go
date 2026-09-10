package linux

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
	"golang.org/x/sys/unix"
)

func TestConfigurationReadBoundsAndTypes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	fixtureOK(t, os.WriteFile(path, []byte(strings.Repeat(" ", 1<<20)+"x"), 0600))
	if _, _, err := Load(path, false); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatal("oversized configuration accepted", err)
	}
	fixtureOK(t, os.Remove(path))
	fixtureOK(t, unix.Mkfifo(path, 0600))
	if _, _, err := Load(path, false); err == nil {
		t.Fatal("FIFO configuration accepted")
	}
	fixtureOK(t, os.Remove(path))
	profiles := filepath.Join(dir, "profiles")
	fixtureOK(t, os.Mkdir(profiles, 0700))
	fixtureOK(t, os.WriteFile(path, fmt.Appendf(nil, "profile_dirs: [%s]\n", profiles), 0600))
	for i := range 1025 {
		fixtureOK(t, os.WriteFile(filepath.Join(profiles, fmt.Sprint(i)), nil, 0600))
	}
	if _, _, err := Load(path, false); err == nil || !strings.Contains(err.Error(), "1024 entries") {
		t.Fatal("oversized directory accepted", err)
	}
	fixtureOK(t, os.Remove(filepath.Join(profiles, "1024")))
	if _, _, err := Load(path, false); err != nil {
		t.Fatal("directory at ceiling rejected", err)
	}
}

func TestRawTailRecordBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		limit         int
		want          []string
		truncated     bool
	}{
		{"empty", "", 8, []string{}, false},
		{"multibyte", "éé\nok\n", 5, []string{"ok"}, true},
		{"line-boundary", "old\nok\n", 3, []string{"ok"}, true},
		{"partial-only", "abcdefgh", 3, []string{}, true},
		{"empty-line", "\n", 3, []string{""}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, dir := fixture(t)
			c.Config.Limits.InspectionBytes = tc.limit
			path := filepath.Join(dir, "log")
			fixtureOK(t, os.WriteFile(path, []byte(tc.content), 0600))
			r := c.Collect(context.Background(), "query_logs", contract.Args{Path: path, RawTail: true})
			if r.Error || !reflect.DeepEqual(r.Data["lines"], tc.want) || r.Truncated != tc.truncated {
				t.Fatalf("unexpected tail: %+v", r)
			}
		})
	}
}

func TestHardlinkedSourcesRemainDeniedWithoutVisibleSecret(t *testing.T) {
	c, root := fixture(t)
	c.Root = root
	fixtureOK(t, os.Mkdir(filepath.Join(root, "etc"), 0700))
	secret := filepath.Join(root, "hidden")
	alias := filepath.Join(root, "etc/os-release")
	fixtureOK(t, os.WriteFile(secret, []byte("ID=SECRET\n"), 0600))
	fixtureOK(t, os.Link(secret, alias))
	// The protected pathname may refer to a mask rather than the linked inode.
	c.Config.TokenStore = filepath.Join(root, "mask")
	fixtureOK(t, os.WriteFile(c.Config.TokenStore, nil, 0600))
	var err error
	c.Policy, err = policy.CompileLinux(c.Config, "/configuration.yaml", nil)
	fixtureOK(t, err)
	if f, err := OpenRegular(alias, c.Policy); err == nil {
		f.Close()
		t.Fatal("hardlinked general source accepted")
	}
	if f, err := openObservation(root, "/etc/os-release", c.Policy, true); err == nil {
		f.Close()
		t.Fatal("hardlinked builtin source accepted")
	}
}

func TestJournalUnsupportedFieldsAreExplicitAndBounded(t *testing.T) {
	c, _ := fixture(t)
	c.Config.Allow.Journal = []string{"app.service"}
	var err error
	c.Policy, err = policy.CompileLinux(c.Config, "/configuration.yaml", nil)
	fixtureOK(t, err)
	timestamp := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	valid := fmt.Sprintf(`{"__REALTIME_TIMESTAMP":"%d","MESSAGE":"ok","PRIORITY":"3","_SYSTEMD_UNIT":"app.service"}`, timestamp.UnixMicro())
	invalid := strings.Replace(valid, `"MESSAGE":"ok"`, `"MESSAGE":[65,66]`, 1)
	c.Runner = &fakeRunner{out: strings.Repeat(invalid+"\n", 1000) + valid + "\n"}
	r := c.Collect(context.Background(), "query_logs", contract.Args{Unit: "app.service", Since: timestamp.Add(-time.Minute).Format(time.RFC3339), Until: timestamp.Add(time.Minute).Format(time.RFC3339)})
	entries, ok := r.Data["entries"].([]map[string]any)
	if !ok || len(entries) != 1 || entries[0]["message"] != "ok" || len(r.Issues) != 2 || r.Issues[0].Code != "invalid_journal_record" || !strings.Contains(r.Issues[0].Message, "1000 records") {
		t.Fatalf("unsupported journal fields hidden or unbounded: %+v", r)
	}
}

type cancellingServiceRunner struct{ cancel context.CancelFunc }

func (r cancellingServiceRunner) Run(context.Context, string, ...string) ([]byte, error) {
	r.cancel()
	return []byte("app.service loaded active running\n"), nil
}

func TestCancelledServiceParsingCannotReportHealthy(t *testing.T) {
	c, root := fixture(t)
	c.Root = root
	c.Config.Health.Required = []string{"services"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Runner = cancellingServiceRunner{cancel}
	r := c.Collect(ctx, "get_health_snapshot", contract.Args{})
	if !r.Error || r.Data["complete"] != false || !reflect.DeepEqual(r.Data["missing_required"], []string{"services"}) {
		t.Fatalf("cancelled parsing reported healthy: %+v", r)
	}
}

func TestRawTailUsesObservedEnd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log")
	fixtureOK(t, os.WriteFile(path, []byte("old\nlast\n"), 0600))
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	fixtureOK(t, err)
	defer f.Close()
	st, err := f.Stat()
	fixtureOK(t, err)
	_, err = f.WriteAt([]byte("new\n"), st.Size())
	fixtureOK(t, err)
	b, truncated, err := readTailWindow(f, st.Size(), 5)
	if err != nil || !truncated || string(b) != "\nlast\n" {
		t.Fatalf("appends changed observed window: %q %t %v", b, truncated, err)
	}
	fixtureOK(t, f.Truncate(3))
	if _, _, err := readTailWindow(f, st.Size(), 5); err == nil || !strings.Contains(err.Error(), "shrank") {
		t.Fatal("shrink not reported", err)
	}
}

func TestJSONLPreservesExplicitMessagesOnly(t *testing.T) {
	c, dir := fixture(t)
	path := filepath.Join(dir, "log")
	content := `{"timestamp":"2026-09-10T12:00:00Z"}` + "\n" +
		`{"timestamp":"2026-09-10T12:00:00Z","message":null}` + "\n" +
		`{"timestamp":"2026-09-10T12:00:00Z","message":"` + string([]byte{0xff}) + `"}` + "\n" +
		`{"timestamp":"2026-09-10T12:00:00Z","message":""}` + "\n"
	fixtureOK(t, os.WriteFile(path, []byte(content), 0600))
	r := c.Collect(context.Background(), "query_logs", contract.Args{Path: path, Format: "jsonl", Since: "2026-09-10T11:59:00Z", Until: "2026-09-10T12:01:00Z"})
	entries, ok := r.Data["entries"].([]map[string]any)
	if !ok || len(entries) != 1 || entries[0]["message"] != "" || len(r.Issues) != 2 || !strings.Contains(r.Issues[0].Message, "3 records") {
		t.Fatalf("missing messages fabricated or explicit empty lost: %+v", r)
	}
}

func TestCapabilitiesRequireMatchingExecutables(t *testing.T) {
	c, root := fixture(t)
	c.Root = root
	for _, dir := range []string{"run/systemd/system", "var/lib/dpkg", "var/lib/pacman/local", "usr/bin"} {
		fixtureOK(t, os.MkdirAll(filepath.Join(root, dir), 0700))
	}
	fixtureOK(t, os.WriteFile(filepath.Join(root, "var/lib/dpkg/status"), nil, 0600))
	for _, executable := range []string{"systemctl", "dpkg-query", "pacman"} {
		fixtureOK(t, os.WriteFile(filepath.Join(root, "usr/bin", executable), []byte("#!/bin/sh\nexit 0\n"), 0600))
	}
	caps := c.Capabilities(context.Background())
	if caps["list_services"] || caps["get_service_status"] || caps["list_packages"] {
		t.Fatal("nonexecutable fixture commands or host commands advertised", caps)
	}
	fixtureOK(t, os.Chmod(filepath.Join(root, "usr/bin/systemctl"), 0700))
	fixtureOK(t, os.Chmod(filepath.Join(root, "usr/bin/pacman"), 0700))
	caps = c.Capabilities(context.Background())
	if !caps["list_services"] || !caps["get_service_status"] || !caps["list_packages"] {
		t.Fatal("usable interfaces omitted", caps)
	}
	name, _, _, err := c.packageCommand()
	if err != nil || name != "pacman" {
		t.Fatal("execution does not match discovery fallback", name, err)
	}
	fixtureOK(t, os.Remove(filepath.Join(root, "usr/bin/pacman")))
	if c.Capabilities(context.Background())["list_packages"] {
		t.Fatal("missing executable advertised")
	}
}
