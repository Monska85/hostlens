//go:build linux

package uninstall

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

func fixture(t *testing.T) (Manager, string) {
	t.Helper()
	root := t.TempDir()
	unitDir := filepath.Join(root, "units")
	if err := os.Mkdir(unitDir, 0700); err != nil {
		t.Fatal(err)
	}
	name := names[0]
	data, err := units.ReadFile("systemd/" + name)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(unitDir, name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return Manager{UnitDir: unitDir, Binary: filepath.Join(root, "missing-binary")}, path
}

func TestPreviewAndRetry(t *testing.T) {
	m, path := fixture(t)
	var calls []string
	m.Run = func(_ context.Context, name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return nil
	}
	var out, progress bytes.Buffer
	if err := m.Execute(context.Background(), false, &out, &progress); err != nil {
		t.Fatal(err)
	}
	var preview Report
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Action != "preview" || !reflect.DeepEqual(preview.Remove, []string{path}) || len(preview.Retain) != 4 || len(calls) != 0 {
		t.Fatalf("unexpected preview: %#v, calls=%v", preview, calls)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := m.Execute(context.Background(), true, &out, &progress); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil || preview.Action != "removed" {
		t.Fatalf("unexpected apply: %s, %v", out.String(), err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unit remains: %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"systemctl disable --now " + names[0], "systemctl daemon-reload"}) {
		t.Fatalf("unexpected systemctl calls: %v", calls)
	}
	m.Run = func(context.Context, string, ...string) error {
		t.Fatal("completed retry contacted systemd")
		return nil
	}
	out.Reset()
	if err := m.Execute(context.Background(), true, &out, &progress); err != nil {
		t.Fatal(err)
	}
}

func TestReloadFailureRetainsBinaryForRetry(t *testing.T) {
	m, unitPath := fixture(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	build, err := buildinfo.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	m.ExpectedMain = build.Path
	source, err := os.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	target, err := os.OpenFile(m.Binary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(target, source); err != nil {
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	reloads := 0
	m.Run = func(_ context.Context, _ string, args ...string) error {
		if len(args) == 1 && args[0] == "daemon-reload" {
			reloads++
			if reloads == 1 {
				return errors.New("temporary bus failure")
			}
		}
		return nil
	}
	var out bytes.Buffer
	if err := m.Execute(context.Background(), true, &out, &out); err == nil || !strings.Contains(err.Error(), "reloading systemd") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(unitPath); err != nil {
		t.Fatalf("unit was not restored for retry: %v", err)
	}
	if _, err := os.Stat(m.Binary); err != nil {
		t.Fatalf("binary removed before reload: %v", err)
	}
	out.Reset()
	if err := m.Execute(context.Background(), true, &out, &out); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if reloads != 2 {
		t.Fatalf("retry did not reload systemd: %d", reloads)
	}
	if _, err := os.Stat(m.Binary); !os.IsNotExist(err) {
		t.Fatalf("binary remains after retry: %v", err)
	}
}

func TestModifiedUnitBlocksAllMutation(t *testing.T) {
	m, path := fixture(t)
	if err := os.WriteFile(filepath.Join(m.UnitDir, names[1]), []byte("[Service]\nExecStart=/bin/true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m.Run = func(context.Context, string, ...string) error { t.Fatal("systemctl called"); return nil }
	var out bytes.Buffer
	if err := m.Execute(context.Background(), true, &out, &out); err == nil || !strings.Contains(err.Error(), "refusing modified unit") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("verified unit was changed: %v", err)
	}
}

func TestSymlinkAndUnknownBinaryBlockMutation(t *testing.T) {
	m, _ := fixture(t)
	if err := os.Symlink("/bin/true", filepath.Join(m.UnitDir, names[1])); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := m.Execute(context.Background(), false, &out, &out); err == nil || !strings.Contains(err.Error(), "unexpected file type") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := os.Remove(filepath.Join(m.UnitDir, names[1])); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.Binary, []byte("not hostlens"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.Execute(context.Background(), false, &out, &out); err == nil || !strings.Contains(err.Error(), "unrecognized executable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUntrustedUnitModeBlocksMutation(t *testing.T) {
	m, path := fixture(t)
	if err := os.Chmod(path, 0666); err != nil {
		t.Fatal(err)
	}
	m.Run = func(context.Context, string, ...string) error { t.Fatal("systemctl called"); return nil }
	var out bytes.Buffer
	if err := m.Execute(context.Background(), true, &out, &out); err == nil || !strings.Contains(err.Error(), "untrusted ownership or mode") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(path, 12345, 12345); errors.Is(err, syscall.EPERM) {
		t.Log("container cannot change file ownership; mode rejection verified")
		return
	} else if err != nil {
		t.Fatal(err)
	}
	if err := m.Execute(context.Background(), true, &out, &out); err == nil || !strings.Contains(err.Error(), "untrusted ownership or mode") {
		t.Fatalf("unexpected owner error: %v", err)
	}
}
