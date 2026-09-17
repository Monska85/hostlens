package linux

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
)

func TestHealthUsesAllServiceObservationsAndPreservesPartialFailures(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, suffix string
		denied       bool
		complete     bool
	}{
		{name: "beyond page", complete: true},
		{name: "denied peer", suffix: "private.service loaded active running\n", denied: true},
		{name: "malformed peer", suffix: strings.Repeat("malformed\n", 1000)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, root := fixture(t)
			c.Root = root
			c.Config.Health.Required = []string{"services"}
			c.Config.Health.Services = config.Threshold{Warning: 1, Critical: 2}
			c.Config.Limits.PageSize = 1
			if tc.denied {
				c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "journal", Pattern: "private.service", Deny: true})
			}
			c.Runner = &fakeRunner{out: "good.service loaded active running\nfailed.service loaded failed failed\n" + tc.suffix}
			listed := c.Collect(context.Background(), "list_services", contract.PageArgs{})
			if listed.NextOffset == nil {
				t.Fatal("public pagination was removed")
			}
			r := c.Collect(context.Background(), "get_health_snapshot", contract.NoArgs{})
			snap, ok := r.Data.(contract.HealthSnapshot)
			if !ok || snap.Severity != "warning" || snap.Complete != tc.complete || !reflect.DeepEqual(snap.FailedServices, []string{"failed.service"}) {
				t.Fatalf("lost observed failure: %+v", r)
			}
			if snap.Checks["services"].Status != "warning" {
				t.Fatalf("partial coverage erased observed check: %+v", snap.Checks)
			}
			if len(r.Issues) > 10 {
				t.Fatal("malformed service issues are unbounded")
			}
		})
	}
}

func TestHealthMountDecodingAndControlSkips(t *testing.T) {
	t.Parallel()

	// Escaped octal mounts decode back to native paths.
	c, root := fixture(t)
	c.Root = root
	c.Config.Health.Required = []string{"filesystem"}
	mount := filepath.Join(root, "line\nbreak")
	fixtureOK(t, os.Mkdir(mount, 0755))
	fixtureOK(t, os.MkdirAll(filepath.Join(root, "proc/self"), 0755))
	escaped := strings.ReplaceAll(mount, "\n", `\012`)
	fixtureOK(t, os.WriteFile(filepath.Join(root, "proc/self/mounts"), []byte("device "+escaped+" ext4 rw 0 0\n"), 0644))
	c.Runner = &fakeRunner{}
	r := c.Collect(context.Background(), "get_health_snapshot", contract.NoArgs{})
	snap, ok := r.Data.(contract.HealthSnapshot)
	if !ok || len(snap.Filesystems) != 1 || snap.Filesystems[0].Mount != mount || !snap.Complete {
		t.Fatalf("escaped native mount was not observed: %+v", r)
	}
	// Control mounts are skipped without hiding real storage.
	for _, missingStorage := range []bool{false, true} {
		t.Run(fmt.Sprint("missing_storage=", missingStorage), func(t *testing.T) {
			c, root := fixture(t)
			c.Root = root
			c.Config.Health.Required = []string{"filesystem"}
			c.Runner = &fakeRunner{}
			fixtureOK(t, os.MkdirAll(filepath.Join(root, "proc/self"), 0755))
			mounts := "control /nonexistent-hostlens-automount autofs rw 0 0\n" +
				"control /nonexistent-hostlens-binfmt binfmt_misc rw 0 0\n" +
				"control " + root + " autofs rw 0 0\n" +
				"device " + root + " ext4 rw 0 0\n"
			if missingStorage {
				mounts += "device " + filepath.Join(root, "missing") + " ext4 rw 0 0\n"
			}
			fixtureOK(t, os.WriteFile(filepath.Join(root, "proc/self/mounts"), []byte(mounts), 0644))
			r := c.Collect(context.Background(), "get_health_snapshot", contract.NoArgs{})
			snap, ok := r.Data.(contract.HealthSnapshot)
			if !ok || len(snap.Filesystems) != 1 || snap.Filesystems[0].Mount != root || snap.Complete != !missingStorage {
				t.Fatalf("incorrect storage coverage: %+v", r)
			}
			for _, issue := range r.Issues {
				if strings.Contains(issue.Source, "nonexistent-hostlens") {
					t.Fatalf("control mount was accessed: %+v", issue)
				}
			}
		})
	}
}
