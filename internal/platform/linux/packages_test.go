package linux

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Monska85/hostlens/internal/contract"
)

func TestPackagesUseInstalledStateRatherThanDesiredAction(t *testing.T) {
	c, root := fixture(t)
	c.Root = root
	fixtureOK(t, os.MkdirAll(filepath.Join(root, "var/lib/dpkg"), 0755))
	fixtureOK(t, os.WriteFile(filepath.Join(root, "var/lib/dpkg/status"), nil, 0644))
	fixtureOK(t, os.MkdirAll(filepath.Join(root, "usr/bin"), 0755))
	fixtureOK(t, os.WriteFile(filepath.Join(root, "usr/bin/dpkg-query"), nil, 0755))
	c.Runner = &fakeRunner{out: "ii \tinstalled\t1\nhi \theld\t2\nri \tselected-remove\t3\npi \tselected-purge\t4\niiR\treinstall-required\t5\nrc \tconfig-only\t6\npn \tremoved\t7\nun \tunknown\t8\niU \tunpacked\t9\niH \thalf-installed\t10\niF \thalf-configured\t11\n"}
	result := c.Collect(context.Background(), "list_packages", contract.Args{})
	expected := []map[string]any{
		{"name": "installed", "version": "1"},
		{"name": "held", "version": "2"},
		{"name": "selected-remove", "version": "3"},
		{"name": "selected-purge", "version": "4"},
		{"name": "reinstall-required", "version": "5"},
	}
	if result.Error || len(result.Issues) != 0 || !reflect.DeepEqual(result.Data["items"], expected) {
		t.Fatalf("incorrect installed inventory: %+v", result)
	}
}
