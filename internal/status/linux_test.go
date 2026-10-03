//go:build linux

package status

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

func TestServiceRejectsInvalidNameAndCanceledCollection(t *testing.T) {
	reader := Linux{}
	if _, err := reader.Service(context.Background(), "../../unsafe.service"); err == nil {
		t.Fatal("invalid native unit reached systemd")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader.Service(ctx, "example.service"); err == nil {
		t.Fatal("canceled service collection returned a fabricated observation")
	}
}

func TestHostPreservesSuccessfulObservationsWhenMemoryFails(t *testing.T) {
	result := collectHost(context.Background(), hostSources{
		info: func(context.Context) (*host.InfoStat, error) {
			return &host.InfoStat{OS: "linux", Uptime: 42}, nil
		},
		cpu: func(context.Context, bool) (int, error) { return 4, nil },
		load: func(context.Context) (*load.AvgStat, error) {
			return &load.AvgStat{Load1: 1.5}, nil
		},
		memory: func(context.Context) (*mem.VirtualMemoryStat, error) {
			return nil, errors.New("source unavailable")
		},
		disk: func(context.Context, string) (*disk.UsageStat, error) {
			return &disk.UsageStat{Total: 100, Used: 25}, nil
		},
	})
	if result.OS != "linux" || result.Uptime == nil || *result.Uptime != 42 ||
		result.LogicalCPUs == nil || *result.LogicalCPUs != 4 ||
		result.LoadOne == nil || *result.LoadOne != 1.5 ||
		result.DiskTotal == nil || *result.DiskTotal != 100 ||
		result.MemoryTotal != nil || result.MemoryUsed != nil ||
		len(result.Issues) != 1 || result.Issues[0] != (Issue{Source: "memory", Kind: "failed"}) {
		t.Fatalf("incorrect partial observation: %+v", result)
	}
}

func TestServiceNameCannotExpandJournalScope(t *testing.T) {
	for _, name := range []string{"*.service", "ssh@*.service", "foo[0].service", "foo?.service", "--system.service", "a/b.service", "a b.service"} {
		if err := validateUnit(name); err == nil {
			t.Fatalf("unsafe unit accepted: %q", name)
		}
	}
	for _, name := range []string{"nginx.service", "ssh@worker_1.service", "db:replica-2.service"} {
		if err := validateUnit(name); err != nil {
			t.Fatalf("valid unit rejected: %q: %v", name, err)
		}
	}
}

func TestJournalPreservesValidEntriesAndReportsUnsupportedFields(t *testing.T) {
	data := strings.Join([]string{
		`{"MESSAGE":"old","__REALTIME_TIMESTAMP":"1000000"}`,
		`{"MESSAGE":[1,2],"__REALTIME_TIMESTAMP":"2000000"}`,
		`{"MESSAGE":null,"__REALTIME_TIMESTAMP":"3000000"}`,
		`{"MESSAGE":"new","__REALTIME_TIMESTAMP":"4000000"}`,
	}, "\n")
	result, err := decodeJournal([]byte(data), ServiceLog{Entries: []LogEntry{}, Limit: 1})
	if err != nil || !result.Truncated || len(result.Entries) != 1 || result.Entries[0].Message != "new" || len(result.Issues) != 1 || result.Issues[0].Kind != "unsupported" {
		t.Fatalf("journal evidence: %+v, %v", result, err)
	}
}

func TestHostSkipsFailedServiceCollectionWhenNotGranted(t *testing.T) {
	result := Linux{}.Host(context.Background(), nil)
	if result.FailedLimit != 0 || len(result.Failed) != 0 {
		t.Fatalf("ungranted service evidence collected: %+v", result)
	}
	for _, issue := range result.Issues {
		if issue.Source == "failed_services" {
			t.Fatal("unrequested systemd collection was attempted")
		}
	}
}

func TestServiceInventoryAppliesPolicyBeforeResultLimit(t *testing.T) {
	var source strings.Builder
	for i := 0; i < 600; i++ {
		source.WriteString("denied-" + strings.Repeat("x", i%5) + ".service loaded active running Denied\n")
	}
	source.WriteString("allowed-late.service loaded active running Allowed\n")
	units, capped, err := collectUnitRows(strings.NewReader(source.String()), func(name string) bool { return name == "allowed-late.service" }, 500)
	if err != nil || capped || len(units) != 1 || units[0].Name != "allowed-late.service" {
		t.Fatalf("permitted service after denied units omitted: %+v, %t, %v", units, capped, err)
	}
}

func TestUnitInventoryParserIgnoresIncompleteAndInvalidRows(t *testing.T) {
	units, capped, err := collectUnitRows(strings.NewReader("alpha.service loaded active running Alpha service\n../invalid.service loaded active running Bad\nbeta.service loaded failed failed Beta\ngamma.service loaded active running Gamma\n"), func(string) bool { return true }, 2)
	if err != nil || !capped || len(units) != 2 || units[0].Name != "alpha.service" || units[1].Name != "beta.service" || units[1].State != "failed" {
		t.Fatalf("parsed incomplete or invalid unit row: %+v, %t, %v", units, capped, err)
	}
	_, capped, err = collectUnitRows(strings.NewReader("alpha.service loaded active running Alpha\nbeta.service loaded failed failed Beta\n"), func(string) bool { return true }, 2)
	if err != nil || capped {
		t.Fatalf("exact result limit was reported as truncated: %t, %v", capped, err)
	}
}

func TestSystemdInventoryAdmissionIsBounded(t *testing.T) {
	systemdInventorySlot <- struct{}{}
	defer func() { <-systemdInventorySlot }()
	if _, err := (Linux{}).Services(context.Background(), func(string) bool { return true }); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("concurrent inventory was admitted: %v", err)
	}
	host := Linux{}.Host(context.Background(), func(string) bool { return true })
	found := false
	for _, issue := range host.Issues {
		if issue.Source == "failed_services" && issue.Kind == "busy" {
			found = true
		}
	}
	if !found {
		t.Fatalf("busy source omitted from host status: %+v", host)
	}
}
