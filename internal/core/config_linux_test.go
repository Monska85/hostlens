//go:build linux

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigurationDefaultsReadOnlyAndRejectsUnsafeFiles(t *testing.T) {
	directory := t.TempDir()
	profiles := filepath.Join(directory, "profiles")
	if err := os.Mkdir(profiles, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "config.yaml")
	content := fmt.Sprintf("version: 2\nlisten: 127.0.0.1:8080\ntoken_store: %s\nprofile_dir: %s\ngateway_uid: 1001\nobserver_uid: 1002\nrepair_uid: 0\nshared_gid: 1001\ntoken_gid: 1003\n", filepath.Join(directory, "tokens.json"), profiles)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	config, _, err := Load(path, map[string]Effect{})
	if err != nil {
		t.Fatal(err)
	}
	if !config.ReadOnly {
		t.Fatal("omitted read_only enabled repair")
	}
	config.Listen = "127.0.0.1:0"
	if err := config.Validate(); err == nil {
		t.Fatal("zero listener port accepted")
	}
	config.Listen = "127.0.0.1:8080"
	config.TLSCert = "relative.pem"
	config.TLSKey = "relative.key"
	if err := config.Validate(); err == nil {
		t.Fatal("relative TLS material accepted")
	}
	if err := os.Chmod(path, 0660); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(path, map[string]Effect{}); err == nil {
		t.Fatal("group-writable configuration accepted")
	}
}
