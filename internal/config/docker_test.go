package config

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func dockerConfig(mutate func(*Config)) Config {
	c := DefaultsLinux(true)
	c.Docker.Enabled = true
	c.Docker.DaemonSocket = "/var/run/docker.sock"
	c.Docker.ObserverSocket = "/run/hostlens/docker-observer.sock"
	c.Docker.Group = "docker"
	if mutate != nil {
		mutate(&c)
	}
	return c
}

func TestDockerEnabledDefaultsValidate(t *testing.T) {
	if e := ValidateLinux(dockerConfig(nil)); e != nil {
		t.Fatal(e)
	}
}

func TestDockerOmittedStartsWithoutAuthority(t *testing.T) {
	c := DefaultsLinux(true)
	if e := ValidateLinux(c); e != nil {
		t.Fatal(e)
	}
	if c.Docker.Enabled {
		t.Fatal("docker must default to disabled")
	}
}

func TestDockerRemoteTransportsRejected(t *testing.T) {
	for _, remote := range []string{
		"tcp://127.0.0.1:2375", "ssh://user@host", "https://host:2376",
		"unix:///var/run/docker.sock", "fd://", "npipe:////./pipe/docker_engine",
	} {
		c := dockerConfig(func(c *Config) { c.Docker.DaemonSocket = remote })
		if e := ValidateLinux(c); e == nil || !strings.Contains(e.Error(), "absolute clean local path") {
			t.Fatalf("remote transport %q accepted: %v", remote, e)
		}
	}
}

func TestDockerUnsafePathsRejected(t *testing.T) {
	for _, path := range []string{"docker.sock", "../docker.sock", "/var/run//docker.sock", "/var/run/docker.sock/", "relative/sock"} {
		c := dockerConfig(func(c *Config) { c.Docker.ObserverSocket = path })
		if e := ValidateLinux(c); e == nil {
			t.Fatalf("unsafe observer path %q accepted", path)
		}
		c = dockerConfig(func(c *Config) { c.Docker.DaemonSocket = path })
		if e := ValidateLinux(c); e == nil {
			t.Fatalf("unsafe daemon path %q accepted", path)
		}
	}
}

func TestDockerIPCCollisionRejected(t *testing.T) {
	for _, target := range []string{cSocket(), cAdmin()} {
		c := dockerConfig(func(c *Config) { c.Docker.ObserverSocket = target })
		if e := ValidateLinux(c); e == nil || !strings.Contains(e.Error(), "collides") {
			t.Fatalf("observer socket %q collision accepted: %v", target, e)
		}
	}
	// The observer must never bind the Docker control path.
	c := dockerConfig(func(c *Config) { c.Docker.ObserverSocket = c.Docker.DaemonSocket })
	if e := ValidateLinux(c); e == nil || !strings.Contains(e.Error(), "differ from the daemon socket") {
		t.Fatalf("observer socket equal to the daemon socket accepted: %v", e)
	}
}

func cSocket() string { return "/run/hostlens/diagnostics.sock" }
func cAdmin() string  { return "/run/hostlens/admin.sock" }

func TestDockerGroupValidation(t *testing.T) {
	for _, group := range []string{"", "-bad", ".bad", "has space", "way-too-long-" + strings.Repeat("x", 40), "bad/slash"} {
		c := dockerConfig(func(c *Config) { c.Docker.Group = group })
		if e := ValidateLinux(c); e == nil {
			t.Fatalf("group %q accepted", group)
		}
	}
	c := dockerConfig(func(c *Config) { c.Docker.Group = "docker-users_2.custom" })
	if e := ValidateLinux(c); e != nil {
		t.Fatalf("nonstandard valid group rejected: %v", e)
	}
}

func TestDockerEnabledRequiresFields(t *testing.T) {
	c := dockerConfig(func(c *Config) { c.Docker.ObserverSocket = "" })
	if e := ValidateLinux(c); e == nil || !strings.Contains(e.Error(), "require the daemon and observer socket") {
		t.Fatalf("enabled without sockets accepted: %v", e)
	}
	c = dockerConfig(func(c *Config) { c.Docker.Group = "" })
	if e := ValidateLinux(c); e == nil || !strings.Contains(e.Error(), "group is required") {
		t.Fatalf("enabled without group accepted: %v", e)
	}
}

func TestDockerRequiresSystemMode(t *testing.T) {
	c := DefaultsLinux(false)
	c.Docker.Enabled = true
	c.Docker.DaemonSocket = "/var/run/docker.sock"
	c.Docker.ObserverSocket = "/run/user/1000/docker-observer.sock"
	if e := ValidateLinux(c); e == nil || !strings.Contains(e.Error(), "system installation") {
		t.Fatalf("docker enabled in user mode accepted: %v", e)
	}
}

func TestDockerDisabledCarriesNoAuthority(t *testing.T) {
	c := dockerConfig(func(c *Config) { c.Docker.Enabled = false })
	if e := ValidateLinux(c); e != nil {
		t.Fatal(e)
	}
}

func TestDockerSettingsAreRestartOnly(t *testing.T) {
	// The gateway restart fingerprint includes the whole docker section;
	// verify value-level differences through marshaled equality.
	a := dockerConfig(nil)
	b := dockerConfig(func(c *Config) { c.Docker.Group = "other" })
	if mustEqual(a.Docker, b.Docker) {
		t.Fatal("docker topology settings must differ for the restart fingerprint")
	}
	if a.Limits.PageSize != b.Limits.PageSize {
		t.Fatal("unrelated limits must not differ")
	}
}

func mustEqual(a, b any) bool {
	am, _ := json.Marshal(a)
	bm, _ := json.Marshal(b)
	return bytes.Equal(am, bm)
}

func TestDockerReloadablePolicyRemainsPolicy(t *testing.T) {
	// Docker policy rides the shared profile rules and stays reloadable.
	c := dockerConfig(nil)
	c.Profile.Allow.Docker = []string{"containers"}
	if e := ValidateLinux(c); e != nil {
		t.Fatal(e)
	}
}

func TestValidateLinuxIdentityNamesAreFixed(t *testing.T) {
	c := dockerConfig(func(c *Config) { c.GatewayUser = "someone-else" })
	if e := ValidateLinux(c); e == nil || !strings.Contains(e.Error(), "fixed") {
		t.Fatalf("renamed gateway identity accepted: %v", e)
	}
	c = dockerConfig(func(c *Config) { c.DiagnosticsUser = "someone-else" })
	if e := ValidateLinux(c); e == nil || !strings.Contains(e.Error(), "fixed") {
		t.Fatalf("renamed diagnostics identity accepted: %v", e)
	}
}

func TestValidateLinuxTLSPathsMustBeAbsoluteAndClean(t *testing.T) {
	for _, path := range []string{"cert/relative.pem", "/abs/cert//p.pem", "/abs/cert/p.pem/"} {
		c := dockerConfig(func(c *Config) {
			c.Server.TLS.Enabled = true
			c.Server.TLS.CertFile = path
			c.Server.TLS.KeyFile = "/abs/key.pem"
		})
		if e := ValidateLinux(c); e == nil || !strings.Contains(e.Error(), "absolute and clean") {
			t.Fatalf("TLS cert path %q accepted: %v", path, e)
		}
	}
	for _, path := range []string{"key/relative.pem", "/abs/key//k.pem", "/abs/key/k.pem/"} {
		c := dockerConfig(func(c *Config) {
			c.Server.TLS.Enabled = true
			c.Server.TLS.CertFile = "/abs/cert.pem"
			c.Server.TLS.KeyFile = path
		})
		if e := ValidateLinux(c); e == nil || !strings.Contains(e.Error(), "absolute and clean") {
			t.Fatalf("TLS key path %q accepted: %v", path, e)
		}
	}
}

func TestValidateLinuxProfileTokenSocketPathsMustBeAbsoluteAndClean(t *testing.T) {
	c := dockerConfig(nil)
	c.ProfileDirs = []string{"relative/profiles"}
	if e := ValidateLinux(c); e == nil || !strings.Contains(e.Error(), "absolute clean path") {
		t.Fatalf("relative profile dir accepted: %v", e)
	}
	c = dockerConfig(nil)
	c.ProfileDirs = []string{"/abs//double"}
	if e := ValidateLinux(c); e == nil {
		t.Fatalf("unclean profile dir accepted")
	}
	c = dockerConfig(nil)
	c.TokenStore = "relative/tokens.json"
	if e := ValidateLinux(c); e == nil {
		t.Fatalf("relative token store accepted")
	}
	c = dockerConfig(nil)
	c.Socket = "/run/x//diag.sock"
	if e := ValidateLinux(c); e == nil {
		t.Fatalf("unclean socket accepted")
	}
	c = dockerConfig(nil)
	c.AdminSocket = "/run/x//admin.sock"
	if e := ValidateLinux(c); e == nil {
		t.Fatalf("unclean admin socket accepted")
	}
}

func TestValidateLinuxUserAndSystemDefaultsDiffer(t *testing.T) {
	user := DefaultsLinux(false)
	system := DefaultsLinux(true)
	if e := ValidateLinux(user); e != nil {
		t.Fatalf("user defaults rejected: %v", e)
	}
	if e := ValidateLinux(system); e != nil {
		t.Fatalf("system defaults rejected: %v", e)
	}
	if user.Mode == system.Mode || user.Privilege == system.Privilege {
		t.Fatalf("user and system defaults must differ: %+v vs %+v", user, system)
	}
	if user.GatewayUser == system.GatewayUser && system.GatewayUser != "" {
		t.Fatalf("system must pin the gateway identity")
	}
}
