package linux

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigurationAggregateByteBudget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "profiles")
	fixtureOK(t, os.Mkdir(dir, 0700))
	path := filepath.Join(root, "config.yaml")
	main := fmt.Sprintf("profile_dirs: [%q]\n", dir)
	fixtureOK(t, os.WriteFile(path, []byte(main), 0600))
	const documentBytes = 1 << 20
	var last string
	var lastData []byte
	for i := range maxConfigurationInputBytes / documentBytes {
		n := documentBytes
		if i == maxConfigurationInputBytes/documentBytes-1 {
			n -= len(main)
		}
		prefix := "profiles: []\n#"
		data := []byte(prefix + strings.Repeat("x", n-len(prefix)))
		last = filepath.Join(dir, fmt.Sprintf("p-%d.yaml", i))
		fixtureOK(t, os.WriteFile(last, data, 0600))
		lastData = data
	}
	// Main configuration counts toward the same budget as inactive definitions.
	if _, _, err := Load(path, false); err != nil {
		t.Fatal("exact aggregate boundary rejected", err)
	}
	fixtureOK(t, os.WriteFile(last, append(lastData, '\n'), 0600))
	if _, _, err := Load(path, false); err == nil || !strings.Contains(err.Error(), "8 MiB aggregate") {
		t.Fatal("aggregate overflow accepted or misclassified", err)
	}
}

func TestConfigurationDefinitionBudgetAcrossDirectories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dirs := []string{filepath.Join(root, "first"), filepath.Join(root, "second")}
	for _, dir := range dirs {
		fixtureOK(t, os.Mkdir(dir, 0700))
	}
	path := filepath.Join(root, "config.yaml")
	fixtureOK(t, os.WriteFile(path, fmt.Appendf(nil, "profile_dirs: [%q, %q]\n", dirs[0], dirs[1]), 0600))
	for i := range maxProfileDefinitions {
		file := filepath.Join(dirs[i%len(dirs)], fmt.Sprintf("p-%04d.yaml", i))
		fixtureOK(t, os.WriteFile(file, []byte("profiles: []\n"), 0600))
	}
	if _, _, err := Load(path, false); err != nil {
		t.Fatal("definition boundary rejected", err)
	}
	fixtureOK(t, os.WriteFile(filepath.Join(dirs[1], "overflow.yaml"), []byte("profiles: []\n"), 0600))
	if _, _, err := Load(path, false); err == nil || !strings.Contains(err.Error(), "1024 profile definitions") {
		t.Fatal("aggregate definition overflow accepted", err)
	}
}

func TestConfigurationDirectoryBudget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "empty")
	fixtureOK(t, os.Mkdir(dir, 0700))
	path := filepath.Join(root, "config.yaml")
	for _, count := range []int{maxProfileDirectories, maxProfileDirectories + 1} {
		data := "profile_dirs:\n" + strings.Repeat(fmt.Sprintf("  - %q\n", dir), count)
		fixtureOK(t, os.WriteFile(path, []byte(data), 0600))
		_, _, err := Load(path, false)
		if count == maxProfileDirectories {
			if err != nil {
				t.Fatal("directory boundary rejected", err)
			}
		} else if err == nil || !strings.Contains(err.Error(), "1024 profile directories") {
			t.Fatal("directory overflow accepted", err)
		}
	}
}
