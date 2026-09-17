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
	t.Parallel()

	c, root := fixture(t)
	c.Root = root
	fixtureOK(t, os.MkdirAll(filepath.Join(root, "var/lib/dpkg"), 0755))
	fixtureOK(t, os.WriteFile(filepath.Join(root, "var/lib/dpkg/status"), nil, 0644))
	fixtureOK(t, os.MkdirAll(filepath.Join(root, "usr/bin"), 0755))
	fixtureOK(t, os.WriteFile(filepath.Join(root, "usr/bin/dpkg-query"), nil, 0755))
	c.Runner = &fakeRunner{out: "ii \tinstalled\t1\nhi \theld\t2\nri \tselected-remove\t3\npi \tselected-purge\t4\niiR\treinstall-required\t5\nrc \tconfig-only\t6\npn \tremoved\t7\nun \tunknown\t8\niU \tunpacked\t9\niH \thalf-installed\t10\niF \thalf-configured\t11\n"}
	result := c.Collect(context.Background(), "list_packages", contract.PageArgs{})
	expected := []contract.PackageRow{
		{Name: "installed", Version: "1"},
		{Name: "held", Version: "2"},
		{Name: "selected-remove", Version: "3"},
		{Name: "selected-purge", Version: "4"},
		{Name: "reinstall-required", Version: "5"},
	}
	page, ok := result.Data.(contract.Page[contract.PackageRow])
	if result.Error || len(result.Issues) != 0 || !ok || !reflect.DeepEqual(page.Items, expected) {
		t.Fatalf("incorrect installed inventory: %+v", result)
	}
}
