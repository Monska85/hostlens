package config

import (
	"errors"
	"fmt"
	"os"
	"path"
)

func DefaultsLinux(system bool) Config {
	c := defaults()
	home, _ := os.UserHomeDir()
	base := path.Join(home, ".config/hostlens")
	run := path.Join(base, "run")
	if system {
		c.Mode = "system"
		c.Privilege = "standard"
		base = "/etc/hostlens"
		run = "/run/hostlens"
		c.GatewayUser = "hostlens-gateway"
		c.DiagnosticsUser = "hostlens-diagnostics"
	}
	c.ProfileDirs = []string{path.Join(base, "profiles")}
	c.TokenStore = path.Join(base, "secrets/tokens.json")
	c.Socket = path.Join(run, "diagnostics.sock")
	c.AdminSocket = path.Join(run, "admin.sock")
	return c
}
func ValidateLinux(c Config) error {
	if c.Mode == "system" && (c.GatewayUser != "hostlens-gateway" || c.DiagnosticsUser != "hostlens-diagnostics") {
		return errors.New("v1 system identity names are fixed")
	}
	if c.Server.TLS.Enabled {
		for _, p := range []string{c.Server.TLS.KeyFile, c.Server.TLS.CertFile} {
			if !path.IsAbs(p) || path.Clean(p) != p {
				return errors.New("TLS paths must be absolute and clean")
			}
		}
	}
	for _, p := range append(append([]string{}, c.ProfileDirs...), c.TokenStore, c.Socket, c.AdminSocket) {
		if !path.IsAbs(p) || path.Clean(p) != p {
			return fmt.Errorf("absolute clean path required: %q", p)
		}
	}
	return validate(c)
}
