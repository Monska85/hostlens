// Command archive validates release material using the runtime upgrade reader.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Monska85/hostlens/internal/lifecycle"
)

func main() {
	archive := flag.String("archive", "", "release archive")
	dest := flag.String("dest", "", "fresh extraction directory (omitted: verify and discard)")
	arch := flag.String("arch", "", "expected release architecture")
	version := flag.String("version", "", "expected release version")
	flag.Parse()
	if err := verify(*archive, *dest, *arch, *version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func verify(archive, dest, arch, version string) error {
	if archive == "" || version == "" || (arch != "amd64" && arch != "arm64") {
		return fmt.Errorf("archive, version and supported architecture are required")
	}
	if dest == "" {
		var err error
		dest, err = os.MkdirTemp("", "hostlens-archive-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dest)
	} else if err := os.Mkdir(dest, 0700); err != nil {
		return err
	}
	release, err := lifecycle.Extract(archive, dest, arch)
	if err != nil {
		return err
	}
	if release.Version != version {
		return fmt.Errorf("release version mismatch")
	}
	for _, name := range []string{"LICENSE", "NOTICE.txt", "config.example.yaml", "OPERATIONS.md", "INSTALL.md", "VALIDATION.md", "tools.json", "profiles/nginx.yaml", "profiles/allow-all.yaml", "profiles/docker-readonly.yaml", "hostlens-docker-observer"} {
		if _, ok := release.Checksums[name]; !ok {
			return fmt.Errorf("missing release material: %s", name)
		}
	}
	licenses, err := filepath.Glob(filepath.Join(dest, "licenses", "*"))
	if err != nil || len(licenses) == 0 {
		return fmt.Errorf("dependency notices missing")
	}
	fmt.Printf("verified %s\n", filepath.Base(archive))
	return nil
}
