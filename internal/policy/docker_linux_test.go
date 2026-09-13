package policy

import (
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/config"
)

func dockerPolicy(t *testing.T, allow, deny []string) *Policy {
	t.Helper()
	c := config.Config{Profile: config.Profile{Allow: config.Rules{Docker: allow}, Deny: config.Rules{Docker: deny}}}
	p, err := CompileLinux(c, "/etc/hostlens/config.yaml", nil)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return p
}

func TestDockerPatternGrammar(t *testing.T) {
	for _, valid := range []string{
		"*", "daemon", "containers", "images", "volumes", "networks", "disk_usage",
		"container/*", "container/web", "stats/*", "logs/*", "image/*", "volume/*", "network/*",
		"container/6f9c2f5f0f0e0c1d2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f",
		"volume/prod-*.data", "network/internal_*",
	} {
		if !validDockerPattern(valid) {
			t.Fatalf("pattern %q must be valid", valid)
		}
	}
	for _, invalid := range []string{
		"", "container", "stats", "logs", "image", "volume", "network",
		"container/", "/web", "unknown/*", "daemon/*", "containers/web",
		"container/a=b", "container/a b", "container/../x", "container/*extra/slash",
	} {
		if validDockerPattern(invalid) {
			t.Fatalf("pattern %q must be invalid", invalid)
		}
	}
}

func TestDockerDenialWinsAcrossIdentityForms(t *testing.T) {
	p := dockerPolicy(t, []string{"container/*", "logs/*"}, []string{"container/db"})
	if !p.DockerDecision("container", "web", id64(1)) {
		t.Fatal("allowed container must pass")
	}
	if p.DockerDecision("container", "db", id64(2)) {
		t.Fatal("denied name must fail even with broad allow")
	}
	if p.DockerDecision("logs", "db", id64(2)) {
		t.Fatal("log authority is independent of container authority but still denied here")
	}
	if !p.DockerDecision("logs", "web", id64(1)) {
		t.Fatal("granted logs must pass")
	}
	p = dockerPolicy(t, []string{"container/web"}, []string{"container/" + id64(3)})
	if p.DockerDecision("container", "web", id64(3)) {
		t.Fatal("denied stable identity must fail despite name allow")
	}
	if !p.DockerDecision("container", "web", id64(4)) {
		t.Fatal("name allow must authorize the resolved identity")
	}
}

func TestDockerDecisionRequiresGrant(t *testing.T) {
	p := dockerPolicy(t, []string{"containers"}, []string{})
	if p.DockerDecision("stats", "", id64(1)) {
		t.Fatal("container inventory grant must not grant stats")
	}
	if p.DockerDecision("container", "", id64(1)) {
		t.Fatal("collection grant must not grant item observation")
	}
}

func TestDockerInactiveProvenanceGrantsNothing(t *testing.T) {
	// An inactive profile's docker grants must not affect decisions.
	defs := map[string]Definition{"docker-wide": {Profile: config.Profile{Allow: config.Rules{Docker: []string{"*"}}}, Source: "/etc/hostlens/profiles/docker-wide.yaml"}}
	c := config.Config{Profile: config.Profile{}}
	p, err := CompileLinux(c, "/etc/hostlens/config.yaml", defs)
	if err != nil {
		t.Fatal(err)
	}
	if p.DockerDecision("daemon", "", id64(1)) {
		t.Fatal("inactive grant must not authorize")
	}
	for _, r := range p.Rules {
		if r.Category == "docker" && !r.Inactive {
			t.Fatal("docker provenance must be inactive")
		}
	}
	if kinds := p.ActiveKinds("docker"); len(kinds) != 0 {
		t.Fatal("inactive kinds leaked", kinds)
	}
}

func TestDockerActiveKinds(t *testing.T) {
	p := dockerPolicy(t, []string{"daemon", "containers", "logs/*", "container/web"}, []string{})
	got := strings.Join(p.ActiveKinds("docker"), ",")
	want := "container,containers,daemon,logs"
	if got != want {
		t.Fatalf("ActiveKinds = %q, want %q", got, want)
	}
	p = dockerPolicy(t, []string{"*"}, []string{})
	if len(p.ActiveKinds("docker")) != len(dockerCollections)+len(dockerItems) {
		t.Fatal("wildcard must cover every docker class")
	}
}

func TestDockerFingerprintReactsToPolicyChange(t *testing.T) {
	a := dockerPolicy(t, []string{"containers"}, []string{})
	b := dockerPolicy(t, []string{"containers", "logs/*"}, []string{})
	if a.Fingerprint() == b.Fingerprint() {
		t.Fatal("fingerprint must react to docker policy change")
	}
}

func TestDockerPolicyExplainFindsRules(t *testing.T) {
	p := dockerPolicy(t, []string{"container/*"}, []string{"container/db"})
	matched := p.Matches("docker", "container/db")
	if len(matched) != 2 {
		t.Fatalf("explanation must list every matching rule, got %d", len(matched))
	}
	denied := false
	for _, m := range matched {
		if m.Deny && m.Source == "/etc/hostlens/config.yaml" {
			denied = true
		}
	}
	if !denied {
		t.Fatal("deny rule must be visible with provenance")
	}
}

func id64(n int) string {
	const hex = "0123456789abcdef"
	s := make([]byte, 64)
	for i := range s {
		s[i] = hex[(n+i)%16]
	}
	return string(s)
}
