package lifecycle

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Release struct {
	Version      string            `json:"version"`
	Schema       int               `json:"schema"`
	Architecture string            `json:"architecture"`
	Checksums    map[string]string `json:"checksums"`
}

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// Extract validates an archive into a fresh private directory. Callers must discard
// the directory on error and supply the architecture they intend to execute.
func Extract(archive, dest, architecture string) (Release, error) {
	var release Release
	f, e := os.Open(archive)
	if e != nil {
		return release, e
	}
	defer f.Close()
	gz, e := gzip.NewReader(f)
	if e != nil {
		return release, e
	}
	defer gz.Close()
	expanded := &io.LimitedReader{R: gz, N: (256 << 20) + 1}
	tr := tar.NewReader(expanded)
	seen := map[string]bool{}
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return release, e
		}
		name := h.Name
		if name == "" || filepath.IsAbs(name) || filepath.Clean(name) != name || strings.HasPrefix(name, "../") || seen[name] {
			return release, errors.New("unsafe or duplicate archive path")
		}
		seen[name] = true
		if h.Typeflag != tar.TypeReg || h.Size < 0 || h.Size > 64<<20 {
			return release, errors.New("archive supports bounded regular files only")
		}
		if (name == "hostlens" || name == "hostlens-diagnostics") && h.Mode != 0755 {
			return release, errors.New("archive executable permissions must be 0755")
		}
		p := filepath.Join(dest, name)
		if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			return release, e
		}
		out, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return release, e
		}
		_, e = io.CopyN(out, tr, h.Size)
		ce := out.Close()
		if e != nil {
			return release, e
		}
		if ce != nil {
			return release, ce
		}
	}
	// Tar EOF does not prove gzip completion: drain padding and verify its trailer.
	if _, e = io.Copy(io.Discard, expanded); e != nil {
		return release, fmt.Errorf("invalid compressed archive: %w", e)
	}
	if expanded.N == 0 {
		return release, errors.New("archive exceeds 256 MiB expanded limit")
	}

	b, e := os.ReadFile(filepath.Join(dest, "release.json"))
	if e != nil {
		return release, e
	}
	if e = json.Unmarshal(b, &release); e != nil {
		return release, e
	}
	if release.Schema != 1 || release.Architecture != architecture || release.Version == "" {
		return release, errors.New("incompatible release schema or architecture")
	}
	delete(seen, "release.json")
	if len(seen) != len(release.Checksums) {
		return release, errors.New("archive manifest does not cover every member")
	}
	for name, want := range release.Checksums {
		if !seen[name] {
			return release, errors.New("missing manifest member")
		}
		b, e = os.ReadFile(filepath.Join(dest, name))
		if e != nil || digest(b) != want {
			return release, errors.New("archive checksum mismatch")
		}
	}
	for _, name := range []string{"hostlens", "hostlens-diagnostics", "hostlens-docker-observer"} {
		if !seen[name] {
			return release, errors.New("archive executable missing")
		}
		if e = os.Chmod(filepath.Join(dest, name), 0755); e != nil {
			return release, e
		}
	}
	return release, nil
}
