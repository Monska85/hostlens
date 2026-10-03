//go:build linux

package status

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
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

var systemdInventorySlot = make(chan struct{}, 1)

func New() Reader { return Linux{} }

func (Linux) Host(ctx context.Context, permitted func(string) bool) Host {
	result := collectHost(ctx, hostSources{
		info: host.InfoWithContext, cpu: cpu.CountsWithContext, load: load.AvgWithContext,
		memory: mem.VirtualMemoryWithContext, swap: mem.SwapMemoryWithContext, disk: disk.UsageWithContext,
	})
	if permitted == nil {
		return result
	}
	result.FailedLimit = 50
	select {
	case systemdInventorySlot <- struct{}{}:
		defer func() { <-systemdInventorySlot }()
	default:
		result.Issues = append(result.Issues, Issue{Source: "failed_services", Kind: "busy"})
		return result
	}
	failedCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	units, capped, err := systemctlRows(failedCtx, "--state=failed", permitted, result.FailedLimit)
	if err != nil {
		result.Issues = append(result.Issues, Issue{Source: "failed_services", Kind: "unavailable"})
		return result
	}
	if capped {
		result.Issues = append(result.Issues, Issue{Source: "failed_services", Kind: "result_limit"})
	}
	for _, unit := range units {
		if unit.State != "failed" {
			continue
		}
		result.Failed = append(result.Failed, unit.Name)
	}
	return result
}

type hostSources struct {
	info   func(context.Context) (*host.InfoStat, error)
	cpu    func(context.Context, bool) (int, error)
	load   func(context.Context) (*load.AvgStat, error)
	memory func(context.Context) (*mem.VirtualMemoryStat, error)
	swap   func(context.Context) (*mem.SwapMemoryStat, error)
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
		result.LoadFive, result.LoadFifteen = &avg.Load5, &avg.Load15
	} else {
		result.Issues = append(result.Issues, Issue{Source: "load", Kind: "unavailable"})
	}
	if m, err := sources.memory(ctx); err == nil && m != nil {
		result.MemoryTotal, result.MemoryUsed = &m.Total, &m.Used
	} else {
		result.Issues = append(result.Issues, Issue{Source: "memory", Kind: "failed"})
	}
	if sources.swap != nil {
		if swap, err := sources.swap(ctx); err == nil && swap != nil {
			result.SwapTotal, result.SwapUsed = &swap.Total, &swap.Used
		} else {
			result.Issues = append(result.Issues, Issue{Source: "swap", Kind: "failed"})
		}
	}
	if d, err := sources.disk(ctx, "/"); err == nil && d != nil {
		result.DiskTotal, result.DiskUsed = &d.Total, &d.Used
		result.InodesTotal, result.InodesUsed = &d.InodesTotal, &d.InodesUsed
	} else {
		result.Issues = append(result.Issues, Issue{Source: "root_disk", Kind: "failed"})
	}
	return result
}

func validateUnit(name string) error {
	if name == "" || len(name) > 256 || !strings.HasSuffix(name, ".service") || strings.HasPrefix(name, "-") {
		return errors.New("invalid service name")
	}
	for _, char := range name {
		if char != ':' && char != '_' && char != '.' && char != '@' && char != '-' && (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') {
			return errors.New("invalid service name")
		}
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
	result := Service{ObservedAt: time.Now().UTC(), Name: name, Availability: property("LoadState"), State: property("ActiveState"), Detail: property("SubState")}
	if serviceProperties, err := conn.GetUnitTypePropertiesContext(ctx, name, "Service"); err == nil {
		result.Result, _ = serviceProperties["Result"].(string)
		if count, ok := serviceProperties["NRestarts"].(uint32); ok {
			value := uint64(count)
			result.Restarts = &value
		} else {
			result.Issues = append(result.Issues, Issue{Source: "service_restarts", Kind: "unavailable"})
		}
	} else {
		result.Issues = append(result.Issues, Issue{Source: "service_properties", Kind: "unavailable"})
	}
	return result, nil
}

func (Linux) Services(ctx context.Context, permitted func(string) bool) (ServiceInventory, error) {
	result := ServiceInventory{ObservedAt: time.Now().UTC(), Services: []Service{}, Limit: 500}
	if permitted == nil {
		return result, nil
	}
	select {
	case systemdInventorySlot <- struct{}{}:
		defer func() { <-systemdInventorySlot }()
	default:
		return ServiceInventory{}, errors.New("systemd inventory busy")
	}
	serviceCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	units, capped, err := systemctlRows(serviceCtx, "--all", permitted, result.Limit)
	if err != nil {
		return ServiceInventory{}, fmt.Errorf("systemd inventory unavailable: %w", err)
	}
	if capped {
		result.Issues = append(result.Issues, Issue{Source: "services", Kind: "result_limit"})
	}
	for _, unit := range units {
		unit.ObservedAt = result.ObservedAt
		result.Services = append(result.Services, unit)
	}
	sort.Slice(result.Services, func(i, j int) bool { return result.Services[i].Name < result.Services[j].Name })
	return result, nil
}

func systemctlRows(ctx context.Context, selection string, permitted func(string) bool, limit int) ([]Service, bool, error) {
	// The utility applies an address-space limit before systemctl requests the
	// complete D-Bus inventory; the gateway only streams permitted rows.
	command := exec.CommandContext(ctx, "/usr/bin/prlimit", "--as=134217728", "--", "/usr/bin/systemctl", "list-units", "--type=service", selection, "--no-legend", "--plain", "--full", "--no-pager")
	command.Env = []string{"PATH=/usr/bin:/bin", "LC_ALL=C"}
	command.WaitDelay = time.Second
	command.Stderr = io.Discard
	pipe, err := command.StdoutPipe()
	if err != nil {
		return nil, false, err
	}
	if err := command.Start(); err != nil {
		return nil, false, err
	}
	units, capped, scanErr := collectUnitRows(pipe, permitted, limit)
	if capped {
		_ = command.Process.Kill()
	}
	waitErr := command.Wait()
	if scanErr != nil {
		return nil, false, scanErr
	}
	if waitErr != nil && !capped {
		return nil, false, waitErr
	}
	return units, capped, nil
}

func collectUnitRows(reader io.Reader, permitted func(string) bool, limit int) ([]Service, bool, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 8<<10)
	units := make([]Service, 0, limit)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || validateUnit(fields[0]) != nil {
			continue
		}
		if !permitted(fields[0]) {
			continue
		}
		if len(units) == limit {
			return units, true, nil
		}
		units = append(units, Service{Name: fields[0], Availability: fields[1], State: fields[2], Detail: fields[3]})
	}
	return units, false, scanner.Err()
}

type cappedWriter struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (writer *cappedWriter) Write(data []byte) (int, error) {
	remaining := writer.limit - writer.buffer.Len()
	if len(data) > remaining {
		writer.buffer.Write(data[:max(remaining, 0)])
		writer.truncated = true
		return max(remaining, 0), errors.New("journal byte ceiling reached")
	}
	return writer.buffer.Write(data)
}

func (Linux) ServiceLogs(ctx context.Context, name string, limit int) (ServiceLog, error) {
	if err := validateUnit(name); err != nil {
		return ServiceLog{}, err
	}
	if limit < 1 || limit > 100 {
		return ServiceLog{}, errors.New("log limit must be between 1 and 100")
	}
	result := ServiceLog{ObservedAt: time.Now().UTC(), Name: name, Entries: []LogEntry{}, Limit: limit}
	command := exec.CommandContext(ctx, "/usr/bin/journalctl", "--no-pager", "--output=json", "--unit", name, "--since=-15min", "--lines", strconv.Itoa(limit+1))
	command.Env = []string{"PATH=/usr/bin:/bin", "LC_ALL=C"}
	command.WaitDelay = time.Second
	output := &cappedWriter{limit: 128 << 10}
	command.Stdout = output
	warnings := &cappedWriter{limit: 4096}
	command.Stderr = warnings
	if err := command.Run(); err != nil && !output.truncated {
		return ServiceLog{}, fmt.Errorf("service journal unavailable: %w", err)
	}
	result.Truncated = output.truncated
	if strings.Contains(warnings.buffer.String(), "not seeing messages") || strings.Contains(warnings.buffer.String(), "insufficient permissions") {
		result.Issues = append(result.Issues, Issue{Source: "service_journal", Kind: "access_limited"})
	}
	return decodeJournal(output.buffer.Bytes(), result)
}

func decodeJournal(data []byte, result ServiceLog) (ServiceLog, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	unsupportedEntry := false
	for {
		var row struct {
			Message json.RawMessage `json:"MESSAGE"`
			Time    string          `json:"__REALTIME_TIMESTAMP"`
		}
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if result.Truncated {
				break
			}
			return ServiceLog{}, fmt.Errorf("invalid journal response: %w", err)
		}
		micros, err := strconv.ParseInt(row.Time, 10, 64)
		if err != nil {
			unsupportedEntry = true
			continue
		}
		if len(row.Message) == 0 || string(row.Message) == "null" {
			unsupportedEntry = true
			continue
		}
		var message string
		if err := json.Unmarshal(row.Message, &message); err != nil {
			unsupportedEntry = true
			continue
		}
		if len(message) > 4096 {
			message = message[:4096]
			result.Truncated = true
		}
		result.Entries = append(result.Entries, LogEntry{Time: time.UnixMicro(micros).UTC(), Message: message})
	}
	if unsupportedEntry {
		result.Issues = append(result.Issues, Issue{Source: "service_journal_entry", Kind: "unsupported"})
	}
	if len(result.Entries) > result.Limit {
		result.Entries = result.Entries[len(result.Entries)-result.Limit:]
		result.Truncated = true
	}
	return result, nil
}
