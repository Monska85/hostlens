//go:build linux

package migrate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Monska85/hostlens/internal/auth"
	"github.com/Monska85/hostlens/internal/core"
	"github.com/Monska85/hostlens/internal/server"
)

func TestPreparePreservesLegacyCredentialWithoutGrantingRepair(t *testing.T) {
	directory := t.TempDir()
	oldStore := filepath.Join(directory, "old-tokens.json")
	secret := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	digest := sha256.Sum256([]byte(secret))
	records := []oldToken{{ID: "old-id", Name: "client", Roles: []string{"diagnostics"}, Status: "active", Hash: hex.EncodeToString(digest[:])}}
	data, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldStore, data, 0600); err != nil {
		t.Fatal(err)
	}
	oldConfig := filepath.Join(directory, "old.yaml")
	profileDir := filepath.Join(directory, "profiles")
	if err := os.Mkdir(profileDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "web.yaml"), []byte("profiles: [database]\nallow:\n  audit: [services]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "database.yaml"), []byte("allow:\n  journal: [postgresql.service]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	data = []byte(fmt.Sprintf("version: 1\nmode: system\nserver:\n  bind: [127.0.0.1]\n  port: 8080\ntoken_store: %s\nprofile_dirs: [%s]\nprofiles: [web]\n", oldStore, profileDir))
	if err := os.WriteFile(oldConfig, data, 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(directory, "candidate")
	report, err := Prepare(Options{From: oldConfig, Output: output, GatewayUID: 1001, ObserverUID: 1002, RepairUID: 1003, SharedGID: 1001, TokenGID: 1004})
	if err != nil {
		t.Fatal(err)
	}
	if report.TokensPreserved != 1 || len(report.ProfilesUnmapped) != 2 || report.ProfilesUnmapped[0] != "database" || report.ProfilesUnmapped[1] != "web" {
		t.Fatalf("unexpected migration report: %+v", report)
	}
	config, policy, err := core.Load(filepath.Join(output, "config.candidate.yaml"), server.ToolEffects())
	if err != nil {
		t.Fatal(err)
	}
	if !config.ReadOnly || policy.Allows(core.EffectRepair, "restart_service", "web.service") {
		t.Fatal("migration enabled repair")
	}
	token, err := (auth.Store{Path: config.TokenStore}).Verify(secret)
	if err != nil {
		t.Fatal(err)
	}
	if len(token.Roles) != 1 || token.Roles[0] != "observe" {
		t.Fatalf("unexpected migrated roles: %+v", token.Roles)
	}
}

func TestPrepareRejectsMultipleOldListenersWithoutPublishingCandidate(t *testing.T) {
	directory := t.TempDir()
	oldConfig := filepath.Join(directory, "old.yaml")
	if err := os.WriteFile(oldConfig, []byte("version: 1\nmode: system\nserver:\n  bind: [127.0.0.1, ::1]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(directory, "candidate")
	if _, err := Prepare(Options{From: oldConfig, Output: output, GatewayUID: 1001, ObserverUID: 1002, RepairUID: 1003, SharedGID: 1001, TokenGID: 1004}); err == nil {
		t.Fatal("expected migration rejection")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("candidate unexpectedly created: %v", err)
	}
}

func TestPrepareRejectsUnsupportedSourceBeforeCandidate(t *testing.T) {
	for name, source := range map[string]string{
		"invalid mode":           "version: 1\nmode: invalid\ntoken_store: /tmp/old-tokens.json\n",
		"unknown setting":        "version: 1\nmode: system\nunrecognized: true\n",
		"unknown server setting": "version: 1\nmode: system\nserver:\n  mystery: true\n",
		"unknown nested setting": "version: 1\nmode: system\nlimits:\n  mystery: true\n",
		"multiple documents":     "version: 1\nmode: system\n---\nversion: 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "old.yaml")
			if err := os.WriteFile(path, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(directory, "candidate")
			if _, err := Prepare(Options{From: path, Output: output}); err == nil {
				t.Fatal("unsupported source was accepted")
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("candidate unexpectedly created: %v", err)
			}
		})
	}
}

func TestPrepareRejectsInvalidLegacyRulesBeforeCandidate(t *testing.T) {
	for name, rules := range map[string]string{
		"invalid audit category": "allow:\n  audit: [not_a_real_audit]\n",
		"invalid file pattern":   "allow:\n  files: [relative/path]\n",
		"invalid Docker target":  "deny:\n  docker: [container/../other]\n",
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			profiles := filepath.Join(directory, "profiles")
			if err := os.Mkdir(profiles, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(profiles, "active.yaml"), []byte(rules), 0600); err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(directory, "old.yaml")
			data := fmt.Sprintf("version: 1\nmode: system\nprofile_dirs: [%s]\nprofiles: [active]\n", profiles)
			if err := os.WriteFile(config, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(directory, "candidate")
			if _, err := Prepare(Options{From: config, Output: output}); err == nil {
				t.Fatal("invalid legacy rules accepted")
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("candidate unexpectedly created: %v", err)
			}
		})
	}
}
