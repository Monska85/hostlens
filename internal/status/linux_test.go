//go:build linux

package status

import (
	"context"
	"errors"
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
