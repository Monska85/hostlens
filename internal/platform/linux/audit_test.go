package linux

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
)

func auditFixture(t *testing.T) (*Collector, string) {
	t.Helper()
	c, root := fixture(t)
	c.Root = root
	c.Config.Allow.Audit = []string{"*"}
	c.Config.Allow.Files = []string{"/**"}
	var err error
	c.Policy, err = policy.CompileLinux(c.Config, "/etc/hostlens/config.yaml", nil)
	if err != nil {
		t.Fatal(err)
	}
	return c, root
}

func TestAuditRequiresExplicitDomainAndValidSelectors(t *testing.T) {
	c, _ := fixture(t)
	for _, tool := range contract.Tools {
		if contract.AuditDomain(tool) == "" {
			continue
		}
		r := c.Collect(context.Background(), tool, contract.Args{})
		if !r.Error || len(r.Issues) != 1 || r.Issues[0].Code != "policy_denied" {
			t.Fatalf("%s: %+v", tool, r)
		}
	}
	c, _ = auditFixture(t)
	for _, tc := range []struct {
		tool string
		args contract.Args
	}{
		{"get_process_info", contract.Args{PID: -1}}, {"inspect_service", contract.Args{Unit: "--all"}}, {"inspect_path", contract.Args{Path: "/etc/../shadow"}}, {"get_hostlens_info", contract.Args{Path: "/etc/shadow"}}, {"list_processes", contract.Args{Offset: -1}},
	} {
		r := c.Collect(context.Background(), tc.tool, tc.args)
		if !r.Error || r.Issues[0].Code != "invalid_arguments" {
			t.Fatalf("%s: %+v", tc.tool, r)
		}
	}
}

func TestAuditPathMetadataRespectsObjectsAndAliases(t *testing.T) {
	c, root := auditFixture(t)
	fixtureOK(t, os.MkdirAll(filepath.Join(root, "etc"), 0755))
	fixtureOK(t, os.WriteFile(filepath.Join(root, "etc/public"), []byte("DO NOT RETURN CONTENT"), 0640))
	fixtureOK(t, os.Symlink("public", filepath.Join(root, "etc/link")))
	fixtureOK(t, os.WriteFile(filepath.Join(root, "etc/protected"), []byte("secret"), 0600))
	c.Config.TokenStore = "/etc/protected"
	var err error
	c.Policy, err = policy.CompileLinux(c.Config, "/etc/hostlens/config.yaml", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/etc/public", "/etc", "/etc/link", "/etc/protected"} {
		r := c.Collect(context.Background(), "inspect_path", contract.Args{Path: path})
		want := path == "/etc/public" || path == "/etc"
		if r.Error == want {
			t.Fatalf("%s: %+v", path, r)
		}
		b, _ := json.Marshal(r)
		if strings.Contains(string(b), "DO NOT RETURN CONTENT") {
			t.Fatal("metadata exposed content")
		}
	}
	fixtureOK(t, os.Link(filepath.Join(root, "etc/protected"), filepath.Join(root, "etc/alias")))
	r := c.Collect(context.Background(), "inspect_path", contract.Args{Path: "/etc/alias"})
	if !r.Error {
		t.Fatal("hardlinked protected metadata exposed", r)
	}
}

func TestHostlensAuditDoesNotReturnSecretConfiguration(t *testing.T) {
	c, _ := auditFixture(t)
	c.Config.Server.TLS.KeyFile = "/never-expose-key-location"
	c.Config.TokenStore = "/never-expose-token-location"
	r := c.Collect(context.Background(), "get_hostlens_info", contract.Args{})
	b, err := json.Marshal(r)
	if err != nil || r.Error || strings.Contains(string(b), "never-expose") || r.Data["privilege"] != c.Config.Privilege {
		t.Fatalf("unsafe self inspection: %+v %v", r, err)
	}
}

type auditRunnerFunc func(context.Context, string, ...string) ([]byte, error)

func (f auditRunnerFunc) Run(ctx context.Context, n string, a ...string) ([]byte, error) {
	return f(ctx, n, a...)
}
func TestServiceAuditSelectsSafeProperties(t *testing.T) {
	c, _ := auditFixture(t)
	c.Runner = auditRunnerFunc(func(_ context.Context, name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name != "systemctl" || strings.Contains(joined, "Environment") || strings.Contains(joined, "ExecStart") {
			t.Fatal("unsafe service query", name, joined)
		}
		return []byte("Id=example.service\nLoadState=loaded\nMainPID=42\nNoNewPrivileges=yes\nEnvironment=PASSWORD=secret\nExecStart=secret\n"), nil
	})
	r := c.Collect(context.Background(), "inspect_service", contract.Args{Unit: "example.service"})
	b, _ := json.Marshal(r)
	if r.Error || strings.Contains(string(b), "secret") {
		t.Fatalf("unsafe service result: %+v", r)
	}
	c.Config.Deny = config.Rules{Journal: []string{"example.service"}}
	c.Policy, _ = policy.CompileLinux(c.Config, "/etc/hostlens/config.yaml", nil)
	r = c.Collect(context.Background(), "inspect_service", contract.Args{Unit: "example.service"})
	if !r.Error || r.Issues[0].Code != "policy_denied" {
		t.Fatal(r)
	}
}

func TestServiceAuditRejectsDeniedCanonicalAlias(t *testing.T) {
	c, _ := auditFixture(t)
	c.Config.Deny.Journal = []string{"ssh.service"}
	c.Policy, _ = policy.CompileLinux(c.Config, "/configuration.yaml", nil)
	c.Runner = auditRunnerFunc(func(context.Context, string, ...string) ([]byte, error) {
		return []byte("Id=ssh.service\nLoadState=loaded\nMainPID=42\n"), nil
	})
	r := c.Collect(context.Background(), "inspect_service", contract.Args{Unit: "sshd.service"})
	if !r.Error || r.Data["service"] != nil || r.Issues[0].Code != "policy_denied" {
		t.Fatalf("canonical denial bypassed: %+v", r)
	}
}

func TestAuditRootMetadataExactGrant(t *testing.T) {
	c, _ := auditFixture(t)
	c.Config.Allow.Files = []string{"/"}
	c.Policy, _ = policy.CompileLinux(c.Config, "/configuration.yaml", nil)
	r := c.Collect(context.Background(), "inspect_path", contract.Args{Path: "/"})
	if r.Error || r.Data["type"] != "directory" {
		t.Fatalf("exact root grant rejected: %+v", r)
	}
}

func TestAuditServiceDiscoveryRequiresRuntime(t *testing.T) {
	c, _ := auditFixture(t)
	if c.Capabilities(context.Background())["inspect_service"] {
		t.Fatal("advertised missing systemd service collector")
	}
}

func TestServiceAuditFiltersDeniedDependencies(t *testing.T) {
	c, _ := auditFixture(t)
	c.Config.Deny.Journal = []string{"secret.service", "-.mount"}
	c.Policy, _ = policy.CompileLinux(c.Config, "/configuration.yaml", nil)
	c.Runner = auditRunnerFunc(func(context.Context, string, ...string) ([]byte, error) {
		return []byte("Id=example.service\nLoadState=loaded\nRequires=secret.service -.mount allowed.service\n"), nil
	})
	r := c.Collect(context.Background(), "inspect_service", contract.Args{Unit: "example.service"})
	b, _ := json.Marshal(r)
	if r.Error || strings.Contains(string(b), "secret.service") || strings.Contains(string(b), "-.mount") || !strings.Contains(string(b), "allowed.service") {
		t.Fatalf("dependency denial failed: %+v", r)
	}
}

func TestAuditPaginationBoundsBeforeCollection(t *testing.T) {
	c, _ := auditFixture(t)
	for _, tool := range []string{"list_processes", "list_accounts"} {
		r := c.Collect(context.Background(), tool, contract.Args{Limit: c.Config.Limits.PageSize + 1})
		if !r.Error || r.Data["coverage_complete"] != false || len(r.Issues) != 1 || r.Issues[0].Code != "invalid_arguments" {
			t.Fatalf("invalid bounds reached collection: %+v", r)
		}
	}
}

func TestServiceAuditPreservesNativeStorageDependencies(t *testing.T) {
	c, _ := auditFixture(t)
	c.Runner = auditRunnerFunc(func(context.Context, string, ...string) ([]byte, error) {
		return []byte("Id=example.service\nLoadState=loaded\nRequires=dev-sda.device dev-sda2.swap -.mount\n"), nil
	})
	r := c.Collect(context.Background(), "inspect_service", contract.Args{Unit: "example.service"})
	b, _ := json.Marshal(r)
	if r.Error || !strings.Contains(string(b), "dev-sda.device dev-sda2.swap -.mount") {
		t.Fatalf("native dependencies lost: %+v", r)
	}
}
