package lifecycle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type archiveMember struct {
	name string
	body string
	mode int64
}

func buildArchive(t *testing.T, members []archiveMember) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, m := range members {
		if e := tw.WriteHeader(&tar.Header{Name: m.name, Size: int64(len(m.body)), Mode: m.mode, Typeflag: tar.TypeReg}); e != nil {
			t.Fatal(e)
		}
		if _, e := tw.Write([]byte(m.body)); e != nil {
			t.Fatal(e)
		}
	}
	if e := tw.Close(); e != nil {
		t.Fatal(e)
	}
	if e := gz.Close(); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "release.tgz")
	if e := os.WriteFile(path, buf.Bytes(), 0644); e != nil {
		t.Fatal(e)
	}
	return path
}

func checksum(body string) string {
	h := sha256.Sum256([]byte(body))
	return hex.EncodeToString(h[:])
}

func validMembers() []archiveMember {
	binaries := []archiveMember{
		{name: "hostlens", body: "bin-main", mode: 0755},
		{name: "hostlens-diagnostics", body: "bin-diag", mode: 0755},
		{name: "hostlens-docker-observer", body: "bin-obs", mode: 0755},
	}
	manifest := `{"version":"0.1.1","schema":1,"architecture":"amd64","checksums":{`
	parts := []string{}
	for _, m := range binaries {
		parts = append(parts, `"`+m.name+`":"`+checksum(m.body)+`"`)
	}
	manifest += strings.Join(parts, ",") + "}}"
	return append(binaries, archiveMember{name: "release.json", body: manifest, mode: 0644})
}

func TestExtractAcceptsValidArchive(t *testing.T) {
	dest := t.TempDir()
	release, e := Extract(buildArchive(t, validMembers()), dest, "amd64")
	if e != nil {
		t.Fatal(e)
	}
	if release.Version != "0.1.1" || release.Schema != 1 || release.Architecture != "amd64" {
		t.Fatalf("release metadata lost: %+v", release)
	}
	for _, name := range []string{"hostlens", "hostlens-diagnostics", "hostlens-docker-observer"} {
		st, e := os.Stat(filepath.Join(dest, name))
		if e != nil || st.Mode().Perm() != 0755 {
			t.Fatalf("executable %s not installed executable: %v", name, e)
		}
	}
}

func TestExtractRefusesUnsafePaths(t *testing.T) {
	for _, tc := range []struct {
		name   string
		member archiveMember
	}{
		{"absolute", archiveMember{name: "/etc/passwd", body: "x", mode: 0644}},
		{"traversal", archiveMember{name: "../escape", body: "x", mode: 0644}},
		{"duplicate", archiveMember{name: "release.json/release.json", body: "x", mode: 0644}},
	} {
		members := validMembers()
		members = append(members, tc.member)
		_, e := Extract(buildArchive(t, members), t.TempDir(), "amd64")
		if e == nil {
			t.Fatalf("unsafe member %s accepted", tc.name)
		}
	}
}

func TestExtractRefusesDirectoryAndWrongModes(t *testing.T) {
	members := validMembers()
	members[0].mode = 0644
	if _, e := Extract(buildArchive(t, members), t.TempDir(), "amd64"); e == nil || !strings.Contains(e.Error(), "0755") {
		t.Fatalf("wrong executable mode accepted: %v", e)
	}
}

func TestExtractRefusesCorruptAndIncompleteArchives(t *testing.T) {
	notGzip := filepath.Join(t.TempDir(), "release.tgz")
	if e := os.WriteFile(notGzip, []byte("definitely not gzip"), 0644); e != nil {
		t.Fatal(e)
	}
	if _, e := Extract(notGzip, t.TempDir(), "amd64"); e == nil {
		t.Fatal("non-gzip archive accepted")
	}
	if _, e := Extract(filepath.Join(t.TempDir(), "absent.tgz"), t.TempDir(), "amd64"); e == nil {
		t.Fatal("absent archive accepted")
	}
	// A release-less archive must refuse before any manifest math.
	members := []archiveMember{{name: "only", body: "x", mode: 0644}}
	if _, e := Extract(buildArchive(t, members), t.TempDir(), "amd64"); e == nil {
		t.Fatal("archive without release.json accepted")
	}
}

func TestExtractRefusesBadManifests(t *testing.T) {
	for _, tc := range []struct {
		name     string
		released string
	}{
		{"wrong schema", `{"version":"0.1.1","schema":2,"architecture":"amd64","checksums":{}}`},
		{"wrong architecture", `{"version":"0.1.1","schema":1,"architecture":"arm64","checksums":{}}`},
		{"missing version", `{"version":"","schema":1,"architecture":"amd64","checksums":{}}`},
		{"malformed", "not json"},
	} {
		members := validMembers()
		members[len(members)-1].body = tc.released
		if _, e := Extract(buildArchive(t, members), t.TempDir(), "amd64"); e == nil {
			t.Fatalf("bad manifest %s accepted", tc.name)
		}
	}
	// A checksum that does not match the stored bytes must refuse.
	members := validMembers()
	members[0].body = "tampered"
	if _, e := Extract(buildArchive(t, members), t.TempDir(), "amd64"); e == nil {
		t.Fatal("tampered binary accepted")
	}
	// A manifest that misses one member must refuse.
	members = validMembers()
	released := members[len(members)-1]
	released.body = `{"version":"0.1.1","schema":1,"architecture":"amd64","checksums":{"hostlens":"` + checksum("bin-main") + `"}}`
	members[len(members)-1] = released
	if _, e := Extract(buildArchive(t, members), t.TempDir(), "amd64"); e == nil {
		t.Fatal("incomplete manifest accepted")
	}
}
