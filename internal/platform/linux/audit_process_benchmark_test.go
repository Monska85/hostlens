package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/policy"
)

func BenchmarkProcessLink(b *testing.B) {
	root := b.TempDir()
	dir := filepath.Join(root, "proc/42/fd")
	if err := os.MkdirAll(dir, 0700); err != nil {
		b.Fatal(err)
	}
	if err := os.Symlink("socket:[123]", filepath.Join(dir, "3")); err != nil {
		b.Fatal(err)
	}
	cfg := config.DefaultsLinux(false)
	cfg.Allow.Files = []string{root + "/**"}
	p, err := policy.CompileLinux(cfg, "/configuration.yaml", nil)
	if err != nil {
		b.Fatal(err)
	}
	c := &Collector{Config: cfg, Policy: p, Root: root}
	f, err := c.auditDirectory("/proc/42/fd")
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	scratch := make([]byte, 4097)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for range 200 {
			if _, err := c.auditLink(f, "3", "/proc/42/fd/3", false, scratch); err != nil {
				b.Fatal(err)
			}
		}
	}
}
