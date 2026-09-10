package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidationScriptPrerequisites(t *testing.T) {
	bin := t.TempDir()
	calls := filepath.Join(bin, "docker-calls")
	stub := "#!/bin/sh\nif [ \"${1:-}\" = info ]; then printf 'linux/amd64\\n'; exit 0; fi\nprintf '%s\\n' \"$*\" >>\"$HOSTLENS_DOCKER_CALLS\"\n[ \"${HOSTLENS_FAKE_IMAGE_MISSING:-0}\" = 0 ]\n"
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(stub), 0755); err != nil {
		t.Fatal(err)
	}
	run := func(script string, extra ...string) ([]byte, error) {
		args := []string{filepath.Join("../../scripts", script)}
		if script == "test-platforms.sh" {
			args = append(args, "debian:stable-slim", "arm64")
		}
		if script == "test-systemd-container.sh" {
			args = append(args, "restricted", "selected-image")
		}
		cmd := exec.Command("sh", args...)
		cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "HOSTLENS_DOCKER_CALLS="+calls, "HOSTLENS_MOD_CACHE=")
		cmd.Env = append(cmd.Env, extra...)
		return cmd.CombinedOutput()
	}
	// The container supplies an already populated, read-only GOMODCACHE.
	if out, err := run("test-container.sh"); err != nil {
		t.Fatal(string(out), err)
	}
	out, _ := os.ReadFile(calls)
	want := "src=" + os.Getenv("GOMODCACHE") + ",dst=/modules,readonly"
	if !strings.Contains(string(out), want) {
		t.Fatal("did not use go env GOMODCACHE", string(out))
	}
	if out, err := run("test-container.sh", "HOSTLENS_MOD_CACHE="+filepath.Join(bin, "absent")); err == nil || !strings.Contains(string(out), "go mod download") {
		t.Fatal(string(out), err)
	}
	os.Remove(calls)
	out, err := run("test-platforms.sh", "HOSTLENS_QEMU_AARCH64="+filepath.Join(bin, "absent"))
	if err == nil || !strings.Contains(string(out), "HOSTLENS_QEMU_AARCH64") {
		t.Fatal(string(out), err)
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatal("launched containers before QEMU preflight")
	}
	out, err = run("test-systemd-container.sh", "HOSTLENS_FAKE_IMAGE_MISSING=1")
	if err == nil || !strings.Contains(string(out), "docker build") {
		t.Fatal(string(out), err)
	}
	out, _ = os.ReadFile(calls)
	if strings.Contains(string(out), "run ") {
		t.Fatal("ran missing systemd image", string(out))
	}
}
