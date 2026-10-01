package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrustLinuxRejectsUncleanPathBeforeTraversal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte("version: 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	unclean := dir + "/sub/../config.yaml"
	if err := TrustLinux(unclean, os.Getuid()); err == nil {
		t.Fatal("unclean policy path accepted")
	}
}
