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
			r := c.Collect(context.Background(), tool, contract.Args{Path: p, RawTail: tool == "query_logs"})
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
	r := c.Collect(context.Background(), "read_config", contract.Args{Path: p})
	if r.Data["content"] != "zero: 0" {
		t.Fatal(r)
	}
	c.Config.Limits.ConfigBytes = 3
	r = c.Collect(context.Background(), "read_config", contract.Args{Path: p})
	if r.Data["content"] != nil || len(r.Issues) == 0 {
		t.Fatal("partial config")
	}
	fixtureOK(t, os.WriteFile(p, []byte{0xff}, 0600))
	r = c.Collect(context.Background(), "read_config", contract.Args{Path: p})
	if r.Data["content"] != nil {
		t.Fatal("invalid encoding")
	}
}
func TestLogSemantics(t *testing.T) {
	t.Parallel()

	c, dir := fixture(t)
	p := filepath.Join(dir, "log")
	fixtureOK(t, os.WriteFile(p, []byte(`{"timestamp":"2026-09-10T12:00:00Z","message":"warning is untrusted text","priority":3}`+"\n"+`{"message":"missing time"}`+"\n"), 0600))
	priority := 4
	a := contract.Args{Path: p, Format: "jsonl", Since: "2026-09-10T11:59:00Z", Until: "2026-09-10T12:01:00Z", Priority: &priority}
	r := c.Collect(context.Background(), "query_logs", a)
	entries, ok := r.Data["entries"].([]map[string]any)
	if !ok || len(entries) != 1 || r.Data["coverage_complete"] != false || len(r.Issues) < 2 {
		t.Fatal(r)
	}
	a.RawTail = true
	a.Format = "raw"
	r = c.Collect(context.Background(), "query_logs", a)
	if len(r.Issues) == 0 {
		t.Fatal("raw severity/time accepted")
	}
	a = contract.Args{Path: p, RawTail: true, Limit: 1}
	r = c.Collect(context.Background(), "query_logs", a)
	if !r.Truncated || r.Data["coverage_complete"] != false {
		t.Fatal(r)
	}
	a = contract.Args{Path: p}
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
	r = c.Collect(context.Background(), "query_logs", contract.Args{Path: path, Format: "jsonl", Since: "2026-09-10T11:59:00Z", Until: "2026-09-10T12:01:00Z"})
	entries, ok = r.Data["entries"].([]map[string]any)
	if !ok || len(entries) != 1 || entries[0]["message"] != "" || len(r.Issues) != 2 || !strings.Contains(r.Issues[0].Message, "3 records") {
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
		r := c.Collect(context.Background(), "get_os_info", contract.Args{})
		if r.Data["kernel"] != "6.18.0" {
			t.Fatal(r)
		}
		if strings.Contains(osRelease, "arch") && r.Data["version"] != nil {
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
		r := c.Collect(context.Background(), "read_config", contract.Args{Path: good})
		if r.Data["content"] == "SECRET" {
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
	r := c.Collect(context.Background(), "list_services", contract.Args{Limit: 1})
	if r.NextOffset == nil || *r.NextOffset != 1 {
		t.Fatal(r)
	}
	f.out = "Id=a.service\nLoadState=loaded\nActiveState=active\n"
	r = c.Collect(context.Background(), "get_service_status", contract.Args{Unit: "a.service"})
	if strings.Contains(strings.Join(f.args, " "), "status") || !strings.Contains(strings.Join(f.args, " "), "--property=") {
		t.Fatal(f.args)
	}
	r = c.Collect(context.Background(), "get_service_status", contract.Args{Unit: "--root=/etc"})
	if len(r.Issues) == 0 {
		t.Fatal("argument injection")
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
	r := c.Collect(context.Background(), "get_health_snapshot", contract.Args{})
	if r.Data["severity"] != "warning" || r.Data["complete"] != false || r.Data["swap_total_bytes"] != float64(0) {
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
						r := c.Collect(context.Background(), tool, contract.Args{Path: path, RawTail: tool == "query_logs"})
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
	if r := c.Collect(context.Background(), "get_os_info", contract.Args{}); r.Data["distribution"] != "public" {
		t.Fatal("safe OS symlink failed", r)
	}
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/etc/secret", Deny: true})
	fixtureOK(t, os.Remove(link))
	fixtureOK(t, os.Symlink("/etc/secret", link))
	if r := c.Collect(context.Background(), "get_os_info", contract.Args{}); r.Data["distribution"] != nil {
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
		r := c.Collect(context.Background(), "get_inventory", contract.Args{})
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
	r := c.Collect(context.Background(), "get_os_info", contract.Args{})
	if r.Data["distribution"] != nil || time.Since(start) > time.Second {
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
	r := c.Collect(context.Background(), "query_logs", contract.Args{Path: path, Format: "jsonl"})
	b, err := json.Marshal(r)
	if err != nil || len(b) > 4096 || len(r.Issues) != 2 || !strings.Contains(r.Issues[0].Message, "2048 records") || r.Data["coverage_complete"] != false {
		t.Fatal("unbounded or dishonest parsing result", len(b), r.Issues)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	r = c.Collect(ctx, "query_logs", contract.Args{Path: path, Format: "jsonl"})
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
	if r := c.Collect(context.Background(), "get_os_info", contract.Args{}); r.Data["distribution"] != nil {
		t.Fatal("builtin mandatory inode leaked", r)
	}
	// Absolute fixture symlinks remain scoped to the fixture, never to the host.
	fixtureOK(t, os.Remove(filepath.Join(root, "etc/os-release")))
	outside := filepath.Join(t.TempDir(), "outside")
	fixtureOK(t, os.WriteFile(outside, []byte("ID=OUTSIDE\n"), 0600))
	fixtureOK(t, os.Symlink(outside, filepath.Join(root, "etc/os-release")))
	if r := c.Collect(context.Background(), "get_os_info", contract.Args{}); r.Data["distribution"] != nil {
		t.Fatal("fixture root escaped", r)
	}
}
