package linux

import (
	"context"
	"encoding/json"
	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixtureOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) (*Collector, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.DefaultsLinux(false)
	cfg.Allow.Files = []string{dir + "/**"}
	p, e := policy.CompileLinux(cfg, "/configuration.yaml", nil)
	if e != nil {
		t.Fatal(e)
	}
	return &Collector{Config: cfg, Policy: p}, dir
}
func TestFileBoundaryAcrossTools(t *testing.T) {
	t.Parallel()

	c, dir := fixture(t)
	secret := filepath.Join(dir, "secret")
	fixtureOK(t, os.WriteFile(secret, []byte("secret"), 0600))
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: secret, Deny: true})
	link := filepath.Join(dir, "link")
	fixtureOK(t, os.Symlink(secret, link))
	fifo := filepath.Join(dir, "fifo")
	fixtureOK(t, unix.Mkfifo(fifo, 0600))
	for _, p := range []string{secret, link, fifo, "/dev/null", "/proc/self/environ"} {
		for _, tool := range []string{"read_config", "query_logs"} {
			var args any = contract.PathArgs{Path: p}
			if tool == "query_logs" {
				args = contract.LogsArgs{Path: p, RawTail: true}
			}
			r := c.Collect(context.Background(), tool, args)
			if len(r.Issues) == 0 {
				t.Errorf("%s exposed %s", tool, p)
			}
			b, _ := json.Marshal(r.Data)
			if strings.Contains(string(b), "secret") {
				t.Errorf("secret returned %s", b)
			}
		}
	}
}
func TestConfigSizeUTF8AndRealZeros(t *testing.T) {
	t.Parallel()

	c, dir := fixture(t)
	p := filepath.Join(dir, "config")
	fixtureOK(t, os.WriteFile(p, []byte("zero: 0"), 0600))
	r := c.Collect(context.Background(), "read_config", contract.PathArgs{Path: p})
	if cf, ok := r.Data.(contract.ConfigFile); !ok || cf.Content != "zero: 0" {
		t.Fatal(r)
	}
	c.Config.Limits.ConfigBytes = 3
	r = c.Collect(context.Background(), "read_config", contract.PathArgs{Path: p})
	if r.Data != nil || len(r.Issues) == 0 {
		t.Fatal("partial config")
	}
	fixtureOK(t, os.WriteFile(p, []byte{0xff}, 0600))
	r = c.Collect(context.Background(), "read_config", contract.PathArgs{Path: p})
	if r.Data != nil {
		t.Fatal("invalid encoding")
	}
}
func TestLogSemantics(t *testing.T) {
	t.Parallel()

	c, dir := fixture(t)
	p := filepath.Join(dir, "log")
	fixtureOK(t, os.WriteFile(p, []byte(`{"timestamp":"2026-09-10T12:00:00Z","message":"warning is untrusted text","priority":3}`+"\n"+`{"message":"missing time"}`+"\n"), 0600))
	priority := 4
	a := contract.LogsArgs{Path: p, Format: "jsonl", Since: "2026-09-10T11:59:00Z", Until: "2026-09-10T12:01:00Z", Priority: &priority}
	r := c.Collect(context.Background(), "query_logs", a)
	lp, ok := r.Data.(contract.LogPage)
	if !ok || len(lp.Entries) != 1 || lp.CoverageComplete || len(r.Issues) < 2 {
		t.Fatal(r)
	}
	a.RawTail = true
	a.Format = "raw"
	r = c.Collect(context.Background(), "query_logs", a)
	if len(r.Issues) == 0 {
		t.Fatal("raw severity/time accepted")
	}
	a = contract.LogsArgs{Path: p, RawTail: true, Limit: 1}
	r = c.Collect(context.Background(), "query_logs", a)
	lp, ok = r.Data.(contract.LogPage)
	if !r.Truncated || !ok || lp.CoverageComplete {
		t.Fatal(r)
	}
	a = contract.LogsArgs{Path: p}
	r = c.Collect(context.Background(), "query_logs", a)
	if r.Issues[0].Code != "unsupported_parser" {
		t.Fatal(r)
	}
	// JSONL entries keep only explicit messages: absent, null, and invalid
	// UTF-8 messages surface as issues instead of fabricated values.
	path := filepath.Join(dir, "log")
	content := `{"timestamp":"2026-09-10T12:00:00Z"}` + "\n" +
		`{"timestamp":"2026-09-10T12:00:00Z","message":null}` + "\n" +
		`{"timestamp":"2026-09-10T12:00:00Z","message":"` + string([]byte{0xff}) + `"}` + "\n" +
		`{"timestamp":"2026-09-10T12:00:00Z","message":""}` + "\n"
	fixtureOK(t, os.WriteFile(path, []byte(content), 0600))
	r = c.Collect(context.Background(), "query_logs", contract.LogsArgs{Path: path, Format: "jsonl", Since: "2026-09-10T11:59:00Z", Until: "2026-09-10T12:01:00Z"})
	lp, ok = r.Data.(contract.LogPage)
	if !ok || len(lp.Entries) != 1 || lp.Entries[0].Message != "" || len(r.Issues) != 2 || !strings.Contains(r.Issues[0].Message, "3 records") {
		t.Fatalf("missing messages fabricated or explicit empty lost: %+v", r)
	}
	// Hardlinked sources stay denied without exposing the linked secret:
	// protected pathnames may point at a mask rather than the same inode.
	_, root := fixture(t)
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
func TestDistributionCapabilitiesWithoutVersionGate(t *testing.T) {
	t.Parallel()

	for _, osRelease := range []string{"ID=debian\nVERSION_ID=99\n", "ID=ubuntu\nVERSION_ID=99.99\n", "ID=arch\n"} {
		c, dir := fixture(t)
		c.Root = dir
		fixtureOK(t, os.MkdirAll(filepath.Join(dir, "etc"), 0755))
		fixtureOK(t, os.MkdirAll(filepath.Join(dir, "proc/sys/kernel"), 0755))
		fixtureOK(t, os.WriteFile(filepath.Join(dir, "etc/os-release"), []byte(osRelease), 0644))
		fixtureOK(t, os.WriteFile(filepath.Join(dir, "proc/sys/kernel/osrelease"), []byte("6.18.0"), 0644))
		r := c.Collect(context.Background(), "get_os_info", contract.NoArgs{})
		os, ok := r.Data.(contract.OSInfo)
		if !ok || os.Kernel != "6.18.0" {
			t.Fatal(r)
		}
		if strings.Contains(osRelease, "arch") && os.Version != "" {
			t.Fatal("invented Arch version")
		}
		caps := c.Capabilities(context.Background())
		if caps["list_services"] || caps["list_packages"] {
			t.Fatal("invented capabilities")
		}
	}
}
func TestTrustAndProfileLoading(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := config.DefaultsLinux(false)
	cfg.ProfileDirs = []string{filepath.Join(dir, "profiles")}
	fixtureOK(t, os.Mkdir(cfg.ProfileDirs[0], 0755))
	p := filepath.Join(dir, "config.yaml")
	fixtureOK(t, os.WriteFile(p, []byte("profile_dirs: ["+cfg.ProfileDirs[0]+"]\nprofiles: [base]\n"), 0644))
	fixtureOK(t, os.WriteFile(filepath.Join(cfg.ProfileDirs[0], "base.yaml"), []byte("allow:\n  files: [/etc/**]\n"), 0644))
	_, pol, e := Load(p, false)
	if e != nil {
		t.Fatal(e)
	}
	if !pol.Allowed("files", "/etc/example", false) {
		t.Fatal("profile inactive")
	}
	fixtureOK(t, os.Chmod(cfg.ProfileDirs[0], 0777))
	if _, _, e = Load(p, false); e == nil {
		t.Fatal("unsafe source accepted")
	}
}
func TestReplacementRaceFailsClosed(t *testing.T) {
	// Serial on purpose: the covered fail-closed branch depends on a
	// microscopic swap window between openat2 and the fd recheck; parallel
	// load makes it vanish and would silently lose the assertion.
	c, dir := fixture(t)
	good := filepath.Join(dir, "good")
	secret := filepath.Join(dir, "secret")
	fixtureOK(t, os.WriteFile(good, []byte("public"), 0600))
	fixtureOK(t, os.WriteFile(secret, []byte("SECRET"), 0600))
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: secret, Deny: true})
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if err := os.Remove(good); err != nil {
				t.Error(err)
				return
			}
			if err := os.Symlink(secret, good); err != nil {
				t.Error(err)
				return
			}
			if err := os.Remove(good); err != nil {
				t.Error(err)
				return
			}
			if err := os.WriteFile(good, []byte("public"), 0600); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	for range 200 {
		r := c.Collect(context.Background(), "read_config", contract.PathArgs{Path: good})
		if cf, _ := r.Data.(contract.ConfigFile); cf.Content == "SECRET" {
			close(stop)
			<-done
			t.Fatal("race bypass")
		}
	}
	close(stop)
	<-done
}

type fakeRunner struct {
	out  string
	err  error
	args []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.args = append([]string{name}, args...)
	return []byte(f.out), f.err
}
func TestServicesExcludeBodiesAndPaginate(t *testing.T) {
	t.Parallel()

	c, _ := fixture(t)
	f := &fakeRunner{out: "a.service loaded active running description\nb.service loaded failed failed desc\n"}
	c.Runner = f
	r := c.Collect(context.Background(), "list_services", contract.PageArgs{Limit: 1})
	if r.NextOffset == nil || *r.NextOffset != 1 {
		t.Fatal(r)
	}
	f.out = "Id=a.service\nLoadState=loaded\nActiveState=active\n"
	r = c.Collect(context.Background(), "get_service_status", contract.UnitArgs{Unit: "a.service"})
	if strings.Contains(strings.Join(f.args, " "), "status") || !strings.Contains(strings.Join(f.args, " "), "--property=") {
		t.Fatal(f.args)
	}
	r = c.Collect(context.Background(), "get_service_status", contract.UnitArgs{Unit: "--root=/etc"})
	if len(r.Issues) == 0 {
		t.Fatal("argument injection")
	}
}
func TestTypedFailurePathsAndGuards(t *testing.T) {
	t.Parallel()

	c, dir := fixture(t)
	c.Root = dir
	for _, tc := range []struct {
		tool string
		args any
	}{
		{"get_service_status", contract.PageArgs{}},
		{"list_services", contract.UnitArgs{Unit: "a.service"}},
		{"list_packages", contract.UnitArgs{Unit: "a.service"}},
		{"read_config", contract.UnitArgs{Unit: "a.service"}},
		{"query_logs", contract.PathArgs{Path: dir}},
	} {
		r := c.Collect(context.Background(), tc.tool, tc.args)
		if !r.Error || r.Issues[0].Code != "invalid_arguments" {
			t.Fatalf("%s accepted wrong argument type: %+v", tc.tool, r)
		}
	}
	c.Runner = &fakeRunner{out: "a.service loaded active running desc\n"}
	r := c.Collect(context.Background(), "list_services", contract.PageArgs{Offset: -1})
	if !r.Error || r.Issues[0].Code != "invalid_bounds" {
		t.Fatalf("negative offset reached a page: %+v", r)
	}
	if r.Data != nil {
		t.Fatalf("failed page carried data: %T", r.Data)
	}
	c.Runner = &fakeRunner{out: "Id=a.service\nLoadState=loaded\nActiveState=active\nSubState=running\nUnitFileState=enabled\nMainPID=42\nResult=success\n"}
	r = c.Collect(context.Background(), "get_service_status", contract.UnitArgs{Unit: "a.service"})
	st, ok := r.Data.(contract.ServiceStatus)
	if !ok || st.ID != "a.service" || st.SubState != "running" || st.UnitFileState != "enabled" || st.MainPID != "42" || st.Result != "success" {
		t.Fatalf("service status fields lost: %+v", r)
	}
	c.Runner = &fakeRunner{out: "Id=a.service\nLoadState=not-found\n"}
	r = c.Collect(context.Background(), "get_service_status", contract.UnitArgs{Unit: "a.service"})
	if !r.Error || r.Issues[0].Code != "not_found" {
		t.Fatalf("missing unit stayed successful: %+v", r)
	}
	c.Runner = &fakeRunner{err: os.ErrPermission}
	r = c.Collect(context.Background(), "get_service_status", contract.UnitArgs{Unit: "a.service"})
	if !r.Error || r.Issues[0].Code != "collection_failed" {
		t.Fatalf("%+v", r)
	}
	r = c.Collect(context.Background(), "list_services", contract.PageArgs{})
	if !r.Error {
		t.Fatalf("failed service list stayed successful: %+v", r)
	}
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/run/systemd/system", Deny: true})
	for _, tool := range []string{"get_service_status", "list_services"} {
		r = c.Collect(context.Background(), tool, map[string]any{"unit": "a.service", "offset": 0, "limit": 5})
		args := contract.PageArgs{}
		if tool == "get_service_status" {
			r = c.Collect(context.Background(), tool, contract.UnitArgs{Unit: "a.service"})
			if !r.Error || r.Issues[0].Code != "policy_denied" {
				t.Fatalf("%s ignored the systemd source denial: %+v", tool, r)
			}
			continue
		}
		_ = args
		r = c.Collect(context.Background(), tool, contract.PageArgs{})
		if !r.Error || r.Issues[0].Code != "policy_denied" {
			t.Fatalf("%s ignored the systemd source denial: %+v", tool, r)
		}
	}
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "journal", Pattern: "a.service", Deny: true})
	r = c.Collect(context.Background(), "get_service_status", contract.UnitArgs{Unit: "a.service"})
	if !r.Error || r.Issues[0].Code != "policy_denied" {
		t.Fatalf("explicit unit denial ignored: %+v", r)
	}
}

func TestPackagesGuardAndRowParsing(t *testing.T) {
	t.Parallel()

	c, dir := fixture(t)
	c.Root = dir
	fixtureOK(t, os.MkdirAll(filepath.Join(dir, "var/lib/dpkg"), 0755))
	fixtureOK(t, os.WriteFile(filepath.Join(dir, "var/lib/dpkg/status"), nil, 0644))
	fixtureOK(t, os.MkdirAll(filepath.Join(dir, "usr/bin"), 0755))
	fixtureOK(t, os.WriteFile(filepath.Join(dir, "usr/bin/dpkg-query"), []byte("#!/bin/sh\n"), 0755))
	r := c.Collect(context.Background(), "list_packages", contract.UnitArgs{Unit: "a"})
	if !r.Error || r.Issues[0].Code != "invalid_arguments" {
		t.Fatalf("%+v", r)
	}
	c.Runner = &fakeRunner{out: "ii\ta.pkg\t1.0\nrc\tpurged.pkg\t2.0\nbadline\n"}
	r = c.Collect(context.Background(), "list_packages", contract.PageArgs{})
	rows, ok := r.Data.(contract.Page[contract.PackageRow])
	if !ok || len(rows.Items) != 1 || rows.Items[0].Name != "a.pkg" || rows.Items[0].Version != "1.0" {
		t.Fatalf("package rows lost: %+v", r)
	}
	c.Runner = &fakeRunner{err: os.ErrPermission}
	r = c.Collect(context.Background(), "list_packages", contract.PageArgs{})
	if !r.Error || r.Issues[0].Code != "collection_failed" {
		t.Fatalf("%+v", r)
	}
}

func TestLogGuardBranchesStayTyped(t *testing.T) {
	t.Parallel()

	c, dir := fixture(t)
	c.Root = dir
	r := c.Collect(context.Background(), "query_logs", contract.LogsArgs{Path: dir + "/app.log", Unit: "a.service"})
	if !r.Error || r.Issues[0].Code != "invalid_bounds" {
		t.Fatalf("two selectors accepted: %+v", r)
	}
	eight := 8
	r = c.Collect(context.Background(), "query_logs", contract.LogsArgs{Path: dir + "/app.log", Priority: &eight})
	if !r.Error || r.Issues[0].Code != "unsupported_filter" {
		t.Fatalf("priority above 7 accepted: %+v", r)
	}
	r = c.Collect(context.Background(), "query_logs", contract.LogsArgs{Path: dir + "/app.log", Format: "jsonl", Since: "not-a-time"})
	if !r.Error || r.Issues[0].Code != "invalid_window" {
		t.Fatalf("%+v", r)
	}
	fixtureOK(t, os.WriteFile(filepath.Join(dir, "app.raw"), []byte("ok\n\xff\xfe broken"), 0600))
	r = c.Collect(context.Background(), "query_logs", contract.LogsArgs{Path: dir + "/app.raw", RawTail: true})
	if !r.Error || r.Issues[0].Code != "unsupported_encoding" {
		t.Fatalf("non-UTF-8 tail accepted: %+v", r)
	}
	fixtureOK(t, os.WriteFile(filepath.Join(dir, "app.log"), []byte("one\ntwo\nthree\n"), 0600))
	c.Config.Limits.InspectionBytes = 4
	r = c.Collect(context.Background(), "query_logs", contract.LogsArgs{Path: dir + "/app.log", Format: "jsonl"})
	if !r.Error || !r.Truncated || r.Issues[0].Code != "inspection_limit" {
		t.Fatalf("oversized source not bounded: %+v", r)
	}
}

func TestHealthIncompletePreservesWarning(t *testing.T) {
	t.Parallel()

	c, dir := fixture(t)
	c.Root = dir
	c.Config.Health.Sample = time.Millisecond
	c.Config.Health.Required = []string{"memory", "services"}
	fixtureOK(t, os.MkdirAll(filepath.Join(dir, "proc"), 0755))
	fixtureOK(t, os.WriteFile(filepath.Join(dir, "proc/meminfo"), []byte("MemTotal: 100 kB\nMemAvailable: 10 kB\nSwapTotal: 0 kB\nSwapFree: 0 kB\n"), 0644))
	c.Runner = &fakeRunner{err: os.ErrPermission}
	r := c.Collect(context.Background(), "get_health_snapshot", contract.NoArgs{})
	snap, ok := r.Data.(contract.HealthSnapshot)
	if !ok || snap.Severity != "warning" || snap.Complete || snap.SwapTotalBytes == nil || *snap.SwapTotalBytes != 0 {
		t.Fatal(r)
	}
}

func TestMissingSourceIsNotContainment(t *testing.T) {
	t.Parallel()

	if e := SecretIsMasked(filepath.Join(t.TempDir(), "absent-secret")); e == nil {
		t.Fatal("arbitrary absent path accepted as credential isolation")
	}
}

func TestMandatoryLiteralSourcesAcrossReaders(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"config[1]", "token*", `key\literal`, "profile?"} {
		for _, kind := range []string{"config", "token", "key", "profile"} {
			t.Run(kind+"/"+name, func(t *testing.T) {
				c, dir := fixture(t)
				secret := filepath.Join(dir, name)
				if err := os.WriteFile(secret, []byte("SECRET"), 0600); err != nil {
					t.Fatal(err)
				}
				source := "/configuration.yaml"
				defs := map[string]policy.Definition{}
				switch kind {
				case "config":
					source = secret
				case "token":
					c.Config.TokenStore = secret
				case "key":
					c.Config.Server.TLS.KeyFile = secret
				case "profile":
					defs["base"] = policy.Definition{Source: secret}
				}
				var err error
				c.Policy, err = policy.CompileLinux(c.Config, source, defs)
				if err != nil {
					t.Fatal(err)
				}
				alias := filepath.Join(dir, "alias")
				if err := os.Link(secret, alias); err != nil {
					t.Fatal(err)
				}
				for _, path := range []string{secret, alias} {
					for _, tool := range []string{"read_config", "query_logs"} {
						var args any = contract.PathArgs{Path: path}
						if tool == "query_logs" {
							args = contract.LogsArgs{Path: path, RawTail: true}
						}
						r := c.Collect(context.Background(), tool, args)
						if !r.Error {
							t.Fatalf("%s allowed literal source or object alias: %+v", tool, r)
						}
					}
				}
			})
		}
	}
}

func TestBuiltinDescriptorBoundaryAndReplacement(t *testing.T) {
	t.Parallel()

	c, root := fixture(t)
	c.Root = root
	for _, dir := range []string{"etc", "usr/lib", "proc/sys/kernel"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	good := filepath.Join(root, "usr/lib/os-release")
	secret := filepath.Join(root, "etc/secret")
	fixtureOK(t, os.WriteFile(good, []byte("ID=public\n"), 0644))
	fixtureOK(t, os.WriteFile(secret, []byte("ID=SECRET\n"), 0644))
	link := filepath.Join(root, "etc/os-release")
	fixtureOK(t, os.Symlink("/usr/lib/os-release", link))
	r := c.Collect(context.Background(), "get_os_info", contract.NoArgs{})
	if os, _ := r.Data.(contract.OSInfo); os.Distribution != "public" {
		t.Fatal("safe OS symlink failed", r)
	}
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/etc/secret", Deny: true})
	fixtureOK(t, os.Remove(link))
	fixtureOK(t, os.Symlink("/etc/secret", link))
	r = c.Collect(context.Background(), "get_os_info", contract.NoArgs{})
	if os, _ := r.Data.(contract.OSInfo); os.Distribution != "" {
		t.Fatal("denied resolved source", r)
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if err := os.Remove(link); err != nil {
				t.Error(err)
				return
			}
			if err := os.Symlink("/usr/lib/os-release", link); err != nil {
				t.Error(err)
				return
			}
			if err := os.Remove(link); err != nil {
				t.Error(err)
				return
			}
			if err := os.Symlink("/etc/secret", link); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	for range 200 {
		r := c.Collect(context.Background(), "get_inventory", contract.NoArgs{})
		b, _ := json.Marshal(r.Data)
		if strings.Contains(string(b), "SECRET") {
			close(stop)
			<-done
			t.Fatal("builtin replacement leaked", string(b))
		}
	}
	close(stop)
	<-done
	fixtureOK(t, os.Remove(link))
	fixtureOK(t, unix.Mkfifo(link, 0600))
	start := time.Now()
	r = c.Collect(context.Background(), "get_os_info", contract.NoArgs{})
	if os, _ := r.Data.(contract.OSInfo); os.Distribution != "" || time.Since(start) > time.Second {
		t.Fatal("special source accepted or blocked")
	}
	fixtureOK(t, os.Remove(link))
	fixtureOK(t, os.Symlink("/proc/self/fd/0", link))
	if _, err := c.file("/etc/os-release", true); err == nil {
		t.Fatal("magic link accepted")
	}
}

func TestMalformedJSONLBoundedIssuesAndCancellation(t *testing.T) {
	t.Parallel()

	c, dir := fixture(t)
	c.Config.Limits.InspectionBytes = 4096
	path := filepath.Join(dir, "malformed.log")
	fixtureOK(t, os.WriteFile(path, []byte(strings.Repeat("x\n", c.Config.Limits.InspectionBytes/2)), 0600))
	r := c.Collect(context.Background(), "query_logs", contract.LogsArgs{Path: path, Format: "jsonl"})
	b, err := json.Marshal(r)
	lp, _ := r.Data.(contract.LogPage)
	if err != nil || len(b) > 4096 || len(r.Issues) != 2 || !strings.Contains(r.Issues[0].Message, "2048 records") || lp.CoverageComplete {
		t.Fatal("unbounded or dishonest parsing result", len(b), r.Issues)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	r = c.Collect(ctx, "query_logs", contract.LogsArgs{Path: path, Format: "jsonl"})
	if !r.Error || !r.Truncated || time.Since(start) > time.Second {
		t.Fatal("parsing ignored cancellation", r)
	}
}

func TestBuiltinProcAndMandatoryObjectProtection(t *testing.T) {
	t.Parallel()

	c, root := fixture(t)
	for _, path := range []string{"/proc/meminfo", "/proc/self/mounts", "/proc/sys/kernel/osrelease"} {
		b, err := c.file(path, true)
		if err != nil || len(b) == 0 {
			t.Fatalf("real proc observation %s: %v", path, err)
		}
	}
	if _, err := c.file("/proc/self/fd/0", true); err == nil {
		t.Fatal("real proc magic link accepted")
	}
	c.Root = root
	fixtureOK(t, os.MkdirAll(filepath.Join(root, "etc"), 0755))
	secret := filepath.Join(root, "etc/secret[1]")
	fixtureOK(t, os.WriteFile(secret, []byte("ID=SECRET\n"), 0600))
	fixtureOK(t, os.Link(secret, filepath.Join(root, "etc/os-release")))
	c.Policy, _ = policy.CompileLinux(c.Config, "/etc/secret[1]", nil)
	r := c.Collect(context.Background(), "get_os_info", contract.NoArgs{})
	if os, _ := r.Data.(contract.OSInfo); os.Distribution != "" {
		t.Fatal("builtin mandatory inode leaked", r)
	}
	// Absolute fixture symlinks remain scoped to the fixture, never to the host.
	fixtureOK(t, os.Remove(filepath.Join(root, "etc/os-release")))
	outside := filepath.Join(t.TempDir(), "outside")
	fixtureOK(t, os.WriteFile(outside, []byte("ID=OUTSIDE\n"), 0600))
	fixtureOK(t, os.Symlink(outside, filepath.Join(root, "etc/os-release")))
	r = c.Collect(context.Background(), "get_os_info", contract.NoArgs{})
	if os, _ := r.Data.(contract.OSInfo); os.Distribution != "" {
		t.Fatal("fixture root escaped", r)
	}
}
