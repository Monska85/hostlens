package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Monska85/hostlens/internal/lifecycle"
)

func TestReleaseMaterials(t *testing.T) {
	for _, missing := range []string{"", "NOTICE.txt", "licenses/dependency_LICENSE", "version"} {
		t.Run(missing, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "release.tar.gz")
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			gz := gzip.NewWriter(f)
			tw := tar.NewWriter(gz)
			release := lifecycle.Release{Schema: 1, Version: "1.2.3", Architecture: "arm64", Checksums: map[string]string{}}
			for _, name := range []string{"hostlens", "hostlens-diagnostics", "hostlens-docker-observer", "LICENSE", "NOTICE.txt", "config.example.yaml", "OPERATIONS.md", "INSTALL.md", "VALIDATION.md", "profiles/nginx.yaml", "profiles/allow-all.yaml", "profiles/docker-readonly.yaml", "licenses/dependency_LICENSE", "release.json"} {
				if name == missing {
					continue
				}
				data := []byte("fixture")
				mode := int64(0644)
				if name == "hostlens" || name == "hostlens-diagnostics" {
					mode = 0755
				}
				if name == "release.json" {
					data, err = json.Marshal(release)
					if err != nil {
						t.Fatal(err)
					}
				} else {
					release.Checksums[name] = fmt.Sprintf("%x", sha256.Sum256(data))
				}
				if err = tw.WriteHeader(&tar.Header{Name: name, Size: int64(len(data)), Mode: mode}); err != nil {
					t.Fatal(err)
				}
				if _, err = tw.Write(data); err != nil {
					t.Fatal(err)
				}
			}
			for _, close := range []func() error{tw.Close, gz.Close, f.Close} {
				if err = close(); err != nil {
					t.Fatal(err)
				}
			}
			version := "1.2.3"
			if missing == "version" {
				version = "1.2.4"
			}
			if err = verify(path, "", "arm64", version); (err == nil) != (missing == "") {
				t.Fatalf("verify = %v", err)
			}
		})
	}
}
