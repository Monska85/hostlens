package linux

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/contract"
)

func TestCpuParseValidLine(t *testing.T) {
	t.Parallel()

	line := []byte("cpu  100 0 50 850 0 0 0 0 0 0\ncpu0 10 0 5 85 0 0 0 0 0 0\n")
	total, idle, e := cpu(line)
	if e != nil {
		t.Fatal(e)
	}
	if total != 1000 || idle != 850 {
		t.Fatalf("aggregate counters lost: total=%v idle=%v", total, idle)
	}
	// Idle and iowait both count toward the idle share.
	_, idle, e = cpu([]byte("cpu  100 0 50 800 50 0 0 0 0 0\n"))
	if e != nil {
		t.Fatal(e)
	}
	if idle != 850 {
		t.Fatalf("idle share lost: %v", idle)
	}
	// Nine counters: the ninth must be ignored by the fixed cap.
	total, idle, e = cpu([]byte("cpu  1 1 1 1 1 1 1 1 99\n"))
	if e != nil {
		t.Fatal(e)
	}
	if total != 8 || idle != 2 {
		t.Fatalf("field cap broken: total=%v idle=%v", total, idle)
	}
	// A static /proc/stat line must stay parseable: the health fixture is
	// guarded against regressing to non-advancing counters.
	if _, _, e := cpu([]byte("cpu  5 0 0 0 0 0 0 0\n")); e != nil {
		t.Fatal(e)
	}
}

func TestCpuParseMalformedFloat(t *testing.T) {
	t.Parallel()

	if _, _, e := cpu([]byte("cpu  1 x 1 1 1 1\n")); e == nil {
		t.Fatal("malformed counter accepted")
	}
	if _, _, e := cpu([]byte("cpu  1 2\n")); e == nil {
		t.Fatal("short cpu line accepted")
	}
}

func TestCpuParseMissingAggregateLine(t *testing.T) {
	t.Parallel()

	if _, _, e := cpu([]byte("cpu0 1 2 3 4 5\nintr 1\n")); e == nil || !strings.Contains(e.Error(), "absent") {
		t.Fatalf("missing aggregate line accepted: %v", e)
	}
}

func TestGetHealthSnapshotCoversCpu(t *testing.T) {
	t.Parallel()

	c := collectorWith(t, nil, nil, nil)
	c.Config.Health.Sample = 10 * time.Millisecond
	c.Config.Health.Required = []string{"cpu"}
	result := c.Collect(context.Background(), "get_health_snapshot", contract.NoArgs{})
	if result.Error {
		t.Fatal(result.Issues)
	}
	snap, ok := result.Data.(contract.HealthSnapshot)
	if !ok {
		t.Fatalf("checks lost: %v", result.Data)
	}
	cpuCheck, ok := snap.Checks["cpu"]
	if !ok {
		t.Fatalf("cpu check missing: %v", snap.Checks)
	}
	if cpuCheck.Status != "OK" && cpuCheck.Status != "warning" && cpuCheck.Status != "critical" {
		t.Fatalf("cpu status invalid: %v", cpuCheck)
	}
	if snap.CPUUtilization == nil {
		t.Fatalf("cpu utilization missing: %v", snap.CPUUtilization)
	}
}
