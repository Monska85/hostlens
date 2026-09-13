package linux

import (
	"os"
	"strings"
	"testing"
)

// inaccessibleFixture renders one synthetic mountinfo line. The mount path
// points at a real temporary resource so the stat guards behave like the
// live check without touching system mounts.
func inaccessibleFixture(mountPath, options string) []byte {
	root := `/systemd/inaccessible/reg`
	if mountPath != "" && strings.HasSuffix(mountPath, "/") || mountPath == "" {
		root = `/systemd/inaccessible/dir`
	}
	return []byte("42 1 0:42 / " + mountPath + " " + root + " " + optionsWithRo(options) + " - tmpfs tmpfs rw,seclabel\n")
}

func optionsWithRo(options string) string {
	if options == "" {
		return "rw,seclabel"
	}
	return options
}

func TestSecretIsMaskedGuards(t *testing.T) {
	dir := t.TempDir()
	masked := dir + "/mask"
	if e := os.Mkdir(masked, 0000); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = os.Chmod(masked, 0700) })

	// No inaccessible mount in the table: refusal.
	info := []byte("42 1 0:42 / / tmpfs rw - tmpfs tmpfs rw\n")
	if e := secretIsMasked(masked, info); e == nil {
		t.Fatal("absent mount accepted")
	}

	// Prefix match without the read-only option: refused.
	noRO := []byte("42 1 0:42 / " + masked + " /systemd/inaccessible/dir rw,seclabel - tmpfs tmpfs rw,seclabel\n")
	if e := secretIsMasked(masked, noRO); e == nil {
		t.Fatal("writable mask accepted")
	}

	// Matching mount with ro but a missing mount resource: refused.
	gone := dir + "/absent-mask"
	table := []byte("42 1 0:42 / " + gone + " /systemd/inaccessible/dir ro,seclabel - tmpfs tmpfs rw,seclabel\n")
	if e := secretIsMasked(gone, table); e == nil {
		t.Fatal("unstatable mask accepted")
	}

	// Real file mask, readable: ownership or permission guards refuse it.
	fileMask := dir + "/file"
	if e := os.WriteFile(fileMask, []byte("x"), 0644); e != nil {
		t.Fatal(e)
	}
	roFile := []byte("42 1 0:42 / " + fileMask + " /systemd/inaccessible/reg ro,seclabel - tmpfs tmpfs rw,seclabel\n")
	if os.Geteuid() != 0 {
		if e := secretIsMasked(fileMask, roFile); e == nil {
			t.Fatal("non-root-owned mask accepted")
		}
	}
	// Non-zero permissions refuse even a root-owned mask.
	if e := os.Chmod(fileMask, 0640); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = os.Chmod(fileMask, 0700) })
	if e := secretIsMasked(fileMask, roFile); e == nil {
		t.Fatal("mask with non-zero permissions accepted")
	}
	if e := os.Chmod(fileMask, 0000); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = os.Chmod(fileMask, 0700) })
	if e := secretIsMasked(fileMask, roFile); e == nil {
		t.Fatal("mask without the accessible reference accepted")
	}
}

func TestSecretIsMaskedReadFailureSurfaces(t *testing.T) {
	if _, e := os.ReadFile("/proc/self/mountinfo"); e != nil {
		t.Skip("mountinfo unavailable in this environment")
	}
	if e := SecretIsMasked("/definitely/not/masked/path"); e == nil {
		t.Fatal("unmasked path accepted")
	}
}

func TestSecretIsMaskedMatchesPrefixOnlyBelowMask(t *testing.T) {
	dir := t.TempDir()
	sibling := dir + "/mask-not"
	if e := os.WriteFile(sibling, []byte("x"), 0644); e != nil {
		t.Fatal(e)
	}
	// A sibling sharing only the text prefix must not pass the prefix check.
	table := []byte("42 1 0:42 / " + dir + "/mask" + " /systemd/inaccessible/dir ro,seclabel - tmpfs tmpfs rw,seclabel\n")
	if e := secretIsMasked(sibling, table); e == nil {
		t.Fatal("prefix-collision path accepted")
	}
}

func TestSecretIsMaskedMalformedLinesIgnored(t *testing.T) {
	table := []byte("not enough fields\n\n42 1 0:42 / /run/inaccessible /systemd/inaccessible/dir ro - tmpfs tmpfs rw\n")
	if e := secretIsMasked("/run/inaccessible", table); e == nil {
		t.Fatal("malformed table with unstatable mask accepted")
	}
}
