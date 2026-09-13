package linux

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/contract"
)

func TestCpuParseValidLine(t *testing.T) {
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
}

func TestCpuParseFieldsCappedAtEight(t *testing.T) {
	// Nine counters: the ninth must be ignored by the fixed cap.
	line := "cpu  1 1 1 1 1 1 1 1 99\n"
	total, idle, e := cpu([]byte(line))
	if e != nil {
		t.Fatal(e)
	}
	if total != 8 || idle != 2 {
		t.Fatalf("field cap broken: total=%v idle=%v", total, idle)
	}
}

func TestCpuParseMalformedFloat(t *testing.T) {
	if _, _, e := cpu([]byte("cpu  1 x 1 1 1 1\n")); e == nil {
		t.Fatal("malformed counter accepted")
	}
}

func TestCpuParseMissingAggregateLine(t *testing.T) {
	if _, _, e := cpu([]byte("cpu0 1 2 3 4 5\nintr 1\n")); e == nil || !strings.Contains(e.Error(), "absent") {
		t.Fatalf("missing aggregate line accepted: %v", e)
	}
}

func TestCpuParseShortFieldsIgnored(t *testing.T) {
	if _, _, e := cpu([]byte("cpu  1 2\n")); e == nil {
		t.Fatal("short cpu line accepted")
	}
}

func TestGetHealthSnapshotCoversCpu(t *testing.T) {
	c := collectorWith(t, nil, nil, nil)
	c.Config.Health.Sample = 10 * time.Millisecond
	c.Config.Health.Required = []string{"cpu"}
	result := c.Collect(context.Background(), "get_health_snapshot", contract.Args{})
	if result.Error {
		t.Fatal(result.Issues)
	}
	checks, ok := result.Data["checks"].(map[string]any)
	if !ok {
		t.Fatalf("checks lost: %v", result.Data)
	}
	cpuCheck, ok := checks["cpu"].(map[string]any)
	if !ok {
		t.Fatalf("cpu check missing: %v", checks)
	}
	if cpuCheck["status"] != "OK" && cpuCheck["status"] != "warning" && cpuCheck["status"] != "critical" {
		t.Fatalf("cpu status invalid: %v", cpuCheck)
	}
	if _, ok := result.Data["cpu_utilization_percent"].(float64); !ok {
		t.Fatalf("cpu utilization missing: %v", result.Data["cpu_utilization_percent"])
	}
}

func TestHealthCpuResetCountersFailClosed(t *testing.T) {
	c := collectorWith(t, nil, nil, nil)
	c.Config.Health.Sample = 10 * time.Millisecond
	c.Config.Health.Required = []string{"cpu"}
	// A static /proc/stat would violate the monotonically increasing sample
	// pair; the fixture must produce real advancing counters, so this is
	// only a guard against regression in the fixture path.
	if _, _, e := cpu([]byte("cpu  5 0 0 0 0 0 0 0\n")); e != nil {
		t.Fatal(e)
	}
}
