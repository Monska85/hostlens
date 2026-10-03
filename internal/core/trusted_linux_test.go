//go:build linux

package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenTrustedRejectsSymlinkAndWritableParent(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.yaml")
	if err := os.WriteFile(path, []byte("version: 2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := OpenTrusted(path)
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	link := filepath.Join(directory, "linked.yaml")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if file, err := OpenTrusted(link); err == nil {
		file.Close()
		t.Fatal("symlink accepted")
	}
	if err := os.Chmod(directory, 0777); err != nil {
		t.Fatal(err)
	}
	if file, err := OpenTrusted(path); err == nil {
		file.Close()
		t.Fatal("writable parent accepted")
	}
}
