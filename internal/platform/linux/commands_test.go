package linux

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// stubRoot builds a Root tree with one executable stub script per name.
func stubRoot(t *testing.T, scripts map[string]string) string {
	t.Helper()
	root := t.TempDir()
	bin := root + "/usr/bin"
	if e := os.MkdirAll(bin, 0755); e != nil {
		t.Fatal(e)
	}
	for name, body := range scripts {
		path := bin + "/" + name
		if e := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0755); e != nil {
			t.Fatal(e)
		}
	}
	return root
}

func TestCommandsRunExecutesAllowedTools(t *testing.T) {
	root := stubRoot(t, map[string]string{
		"systemctl":  `echo unit-ok`,
		"journalctl": `echo -n "$@"`,
	})
	r := Commands{Limit: 4096, Root: root}
	out, e := r.Run(context.Background(), "systemctl")
	if e != nil || strings.TrimSpace(string(out)) != "unit-ok" {
		t.Fatalf("stub execution failed: %q %v", out, e)
	}
	out, e = r.Run(context.Background(), "journalctl", "arg1", "arg2")
	if e != nil || strings.TrimSpace(string(out)) != "arg1 arg2" {
		t.Fatalf("argument passing lost: %q %v", out, e)
	}
}

func TestCommandsRunRejectsUnsupportedAndMissing(t *testing.T) {
	root := stubRoot(t, map[string]string{"systemctl": "true"})
	r := Commands{Limit: 4096, Root: root}
	if _, e := r.Run(context.Background(), "rm"); e == nil || !strings.Contains(e.Error(), "unsupported executable") {
		t.Fatalf("disallowed executable accepted: %v", e)
	}
	if _, e := r.Run(context.Background(), "dpkg-query"); e == nil || !strings.Contains(e.Error(), "collector unavailable") {
		t.Fatalf("missing executable accepted: %v", e)
	}
}

func TestCommandsRunReportsExitFailures(t *testing.T) {
	root := stubRoot(t, map[string]string{"systemctl": "echo to-stderr >&2; exit 3"})
	r := Commands{Limit: 4096, Root: root}
	if _, e := r.Run(context.Background(), "systemctl"); e == nil {
		t.Fatal("non-zero exit accepted")
	}
}

func TestCommandsRunReportsInspectionCeiling(t *testing.T) {
	root := stubRoot(t, map[string]string{"systemctl": "printf 'aaaaaaaaaaaaaaaaaaaa'"})
	r := Commands{Limit: 4, Root: root}
	if _, e := r.Run(context.Background(), "systemctl"); e == nil || !strings.Contains(e.Error(), "inspection limit exceeded") {
		t.Fatalf("oversized output accepted: %v", e)
	}
}

func TestCommandsRunBoundsHungProcess(t *testing.T) {
	root := stubRoot(t, map[string]string{"systemctl": "sleep 30"})
	r := Commands{Limit: 4096, Root: root}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, e := r.Run(ctx, "systemctl")
	if e == nil {
		t.Fatal("hung process returned success")
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("hang not bounded: %v", elapsed)
	}
}

func TestCommandPathRejectsDirectoryAndEmptyRoot(t *testing.T) {
	root := t.TempDir()
	if e := os.MkdirAll(root+"/usr/bin/systemctl", 0755); e != nil {
		t.Fatal(e)
	}
	if _, e := commandPath(root, "systemctl"); e == nil {
		t.Fatal("directory accepted as executable")
	}
	if _, e := commandPath(root, "nonexistent-tool"); e == nil {
		t.Fatal("unknown name accepted")
	}
}

func TestLimitedBufferClampsOversizedWrites(t *testing.T) {
	b := &limitedBuffer{n: 8}
	if _, e := b.Write([]byte("12345678")); e != nil {
		t.Fatal(e)
	}
	if b.exceeded {
		t.Fatal("exact fit must not report excess")
	}
	if _, e := b.Write([]byte("more")); e != nil {
		t.Fatal(e)
	}
	if !b.exceeded || string(b.buf) != "12345678" {
		t.Fatalf("ceiling not enforced: %q exceeded=%v", b.buf, b.exceeded)
	}
	empty := &limitedBuffer{n: 4}
	if _, e := empty.Write([]byte("1234")); e != nil || empty.exceeded {
		t.Fatalf("exact fit rejected: %v", empty.exceeded)
	}
}
