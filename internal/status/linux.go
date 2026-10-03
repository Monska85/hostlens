//go:build linux

package status

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

type Linux struct{}

func New() Reader { return Linux{} }

func (Linux) Host(ctx context.Context) Host {
	return collectHost(ctx, hostSources{
		info: host.InfoWithContext, cpu: cpu.CountsWithContext, load: load.AvgWithContext,
		memory: mem.VirtualMemoryWithContext, disk: disk.UsageWithContext,
	})
}

type hostSources struct {
	info   func(context.Context) (*host.InfoStat, error)
	cpu    func(context.Context, bool) (int, error)
	load   func(context.Context) (*load.AvgStat, error)
	memory func(context.Context) (*mem.VirtualMemoryStat, error)
	disk   func(context.Context, string) (*disk.UsageStat, error)
}

func collectHost(ctx context.Context, sources hostSources) Host {
	result := Host{ObservedAt: time.Now().UTC(), Architecture: runtime.GOARCH}
	if h, err := sources.info(ctx); err == nil && h != nil {
		result.OS, result.Platform, result.Kernel = h.OS, h.Platform, h.KernelVersion
		result.Uptime = &h.Uptime
	} else {
		result.Issues = append(result.Issues, Issue{Source: "host", Kind: "failed"})
	}
	if n, err := sources.cpu(ctx, true); err == nil && n > 0 {
		result.LogicalCPUs = &n
	} else {
		result.Issues = append(result.Issues, Issue{Source: "cpu", Kind: "failed"})
	}
	if avg, err := sources.load(ctx); err == nil && avg != nil {
		result.LoadOne = &avg.Load1
	} else {
		result.Issues = append(result.Issues, Issue{Source: "load", Kind: "unavailable"})
	}
	if m, err := sources.memory(ctx); err == nil && m != nil {
		result.MemoryTotal, result.MemoryUsed = &m.Total, &m.Used
	} else {
		result.Issues = append(result.Issues, Issue{Source: "memory", Kind: "failed"})
	}
	if d, err := sources.disk(ctx, "/"); err == nil && d != nil {
		result.DiskTotal, result.DiskUsed = &d.Total, &d.Used
	} else {
		result.Issues = append(result.Issues, Issue{Source: "root_disk", Kind: "failed"})
	}
	return result
}

func validateUnit(name string) error {
	if name == "" || len(name) > 256 || !strings.HasSuffix(name, ".service") || strings.ContainsAny(name, "/\\\x00\n\r") || strings.HasPrefix(name, "-") {
		return errors.New("invalid service name")
	}
	return nil
}

func (Linux) Service(ctx context.Context, name string) (Service, error) {
	if err := validateUnit(name); err != nil {
		return Service{}, err
	}
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		return Service{}, fmt.Errorf("systemd unavailable: %w", err)
	}
	defer conn.Close()
	properties, err := conn.GetUnitPropertiesContext(ctx, name)
	if err != nil {
		return Service{}, fmt.Errorf("unit status unavailable: %w", err)
	}
	property := func(key string) string {
		value, _ := properties[key].(string)
		return value
	}
	return Service{ObservedAt: time.Now().UTC(), Name: name, Availability: property("LoadState"), State: property("ActiveState"), Detail: property("SubState")}, nil
}
