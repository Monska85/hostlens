package policy

import (
	"testing"

	"github.com/Monska85/hostlens/internal/config"
)

func TestAuditDomainsPreserveExplicitAuthorityAndDenial(t *testing.T) {
	c := config.DefaultsLinux(false)
	c.Allow.Files = []string{"/**"}
	c.Allow.Journal = []string{"*"}
	p, e := CompileLinux(c, "/config.yaml", nil)
	if e != nil {
		t.Fatal(e)
	}
	before := p.Fingerprint()
	if p.Allowed("audit", "network", false) {
		t.Fatal("file/journal grant authorized audit")
	}
	c.Profiles = []string{"audit"}
	defs := map[string]Definition{"audit": {Profile: config.Profile{Allow: config.Rules{Audit: []string{"*"}}}, Source: "/profiles/audit.yaml"}}
	p, e = CompileLinux(c, "/config.yaml", defs)
	if e != nil {
		t.Fatal(e)
	}
	if !p.Allowed("audit", "network", false) || p.Fingerprint() == before {
		t.Fatal("audit grant missing from evaluation or fingerprint")
	}
	c.Deny.Audit = []string{"network"}
	p, e = CompileLinux(c, "/config.yaml", defs)
	if e != nil {
		t.Fatal(e)
	}
	if p.Allowed("audit", "network", false) || !p.Allowed("audit", "accounts", false) {
		t.Fatal("domain denial incorrect")
	}
	c.Allow.Audit = []string{"netwrok"}
	if _, e = CompileLinux(c, "/config.yaml", defs); e == nil {
		t.Fatal("unknown domain accepted")
	}
}
