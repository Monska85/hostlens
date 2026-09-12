package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Monska85/hostlens/internal/contract"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"strings"
	"testing"
	"time"
)

func TestCatalogBoundedAndPrivate(t *testing.T) {
	for range 2 {
		r := New("backend")
		for i := 0; i < 1000; i++ {
			r.Tool(fmt.Sprintf("secret-name-%d", i), contract.Result{Issues: []contract.Issue{{Code: "secret-error", Source: "/secret", Message: "secret-content"}}}, time.Millisecond)
		}
		f, err := r.Gather()
		if err != nil {
			t.Fatal(err)
		}
		wire, err := Wire(f)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeBackend(bytes.NewReader(wire))
		if err != nil {
			t.Fatal(err)
		}
		text, err := Encode(decoded)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(text), "secret") || len(text) > 10000 {
			t.Fatal("unbounded or sensitive exposition")
		}
		parser := expfmt.NewTextParser(model.LegacyValidation)
		parsed, err := parser.TextToMetricFamilies(bytes.NewReader(text))
		if err != nil {
			t.Fatal(err)
		}
		if len(parsed) != 11 || parsed["hostlens_tool_calls_total"].GetType() != dto.MetricType_COUNTER || parsed["hostlens_tool_calls_total"].Metric[0].Counter.GetValue() != 1000 {
			t.Fatalf("catalog: %d %v", len(parsed), parsed)
		}
	}
}
func TestToolOutcomesAndGapDeduplication(t *testing.T) {
	for _, tc := range []struct {
		result contract.Result
		want   string
	}{
		{contract.Result{}, "success"}, {contract.Result{Issues: []contract.Issue{{Code: "missing_measurement"}}}, "partial"},
		{contract.Failure("source_denied_or_unavailable"), "unavailable"}, {contract.Failure("timeout"), "timeout"},
		{contract.Failure("cancelled"), "cancelled"}, {contract.Failure("overload"), "overload"}, {contract.Failure("policy_denied"), "denied"}, {contract.Failure("unknown"), "error"},
	} {
		r := New("backend")
		r.Tool("get_os_info", tc.result, time.Millisecond)
		f, _ := r.Gather()
		for _, family := range f {
			if family.GetName() == "hostlens_tool_calls_total" {
				for _, l := range family.Metric[0].Label {
					if l.GetName() == "outcome" && l.GetValue() != tc.want {
						t.Fatal(tc.want, l.GetValue())
					}
				}
			}
		}
	}
	r := New("backend")
	r.Tool("get_os_info", contract.Result{Issues: []contract.Issue{{Code: "missing_measurement"}, {Code: "not_found"}}}, 0)
	f, _ := r.Gather()
	for _, family := range f {
		if family.GetName() == "hostlens_collection_gaps_total" && (len(family.Metric) != 1 || family.Metric[0].Counter.GetValue() != 1) {
			t.Fatal("gap counted twice")
		}
	}
}
func TestRejectBackendPoisonAndOverflow(t *testing.T) {
	r := New("backend")
	r.Tool("get_os_info", contract.Result{}, time.Second)
	fresh := func() []*dto.MetricFamily { f, _ := r.Gather(); return f }
	for _, mutate := range []func([]*dto.MetricFamily) []*dto.MetricFamily{
		func(f []*dto.MetricFamily) []*dto.MetricFamily {
			for _, family := range f {
				if family.GetType() == dto.MetricType_GAUGE {
					family.Metric[0].Gauge.Value = nil
					break
				}
			}
			return f
		},
		func(f []*dto.MetricFamily) []*dto.MetricFamily {
			for _, family := range f {
				if family.GetType() == dto.MetricType_COUNTER {
					family.Metric[0].Counter.Value = nil
					break
				}
			}
			return f
		},
		func(f []*dto.MetricFamily) []*dto.MetricFamily {
			for _, family := range f {
				if family.GetType() == dto.MetricType_HISTOGRAM {
					family.Metric[0].Histogram.SampleCount = nil
					break
				}
			}
			return f
		},
		func(f []*dto.MetricFamily) []*dto.MetricFamily {
			for _, family := range f {
				if family.GetType() == dto.MetricType_HISTOGRAM {
					family.Metric[0].Histogram.SampleSum = nil
					break
				}
			}
			return f
		},
		func(f []*dto.MetricFamily) []*dto.MetricFamily {
			for _, family := range f {
				if family.GetType() == dto.MetricType_HISTOGRAM {
					family.Metric[0].Histogram.Bucket[0].CumulativeCount = nil
					break
				}
			}
			return f
		},
		func(f []*dto.MetricFamily) []*dto.MetricFamily {
			for _, family := range f {
				if family.GetType() == dto.MetricType_HISTOGRAM {
					value := 123.
					family.Metric[0].Histogram.Bucket[0].CumulativeCountFloat = &value
					break
				}
			}
			return f
		},

		func(f []*dto.MetricFamily) []*dto.MetricFamily { return append(f, f[0]) },
		func(f []*dto.MetricFamily) []*dto.MetricFamily { x := "secret"; f[0].Name = &x; return f },
		func(f []*dto.MetricFamily) []*dto.MetricFamily {
			x := "secret"
			f[0].Metric[0].Label[0].Value = &x
			return f
		},
		func(f []*dto.MetricFamily) []*dto.MetricFamily {
			f[0].Metric = append(f[0].Metric, f[0].Metric[0])
			return f
		},
		func(f []*dto.MetricFamily) []*dto.MetricFamily {
			x := int64(123)
			f[0].Metric[0].TimestampMs = &x
			return f
		},
		func(f []*dto.MetricFamily) []*dto.MetricFamily {
			for _, v := range f {
				if v.GetType() == dto.MetricType_HISTOGRAM {
					x := 999.
					v.Metric[0].Histogram.Bucket[0].UpperBound = &x
				}
			}
			return f
		},
	} {
		b, _ := json.Marshal(mutate(fresh()))
		if _, err := DecodeBackend(bytes.NewReader(b)); err == nil {
			t.Fatal("poison accepted")
		}
	}
	for _, s := range []string{"null", "[]", "{}", "[{}]", "[] []", strings.Repeat(" ", MaxBytes+1)} {
		if _, err := DecodeBackend(strings.NewReader(s)); err == nil {
			t.Fatal("bad payload accepted")
		}
	}
	f := fresh()
	large := strings.Repeat("x", MaxBytes)
	f[0].Help = &large
	if _, err := Encode(f); err == nil {
		t.Fatal("exposition overflow succeeded")
	}
	if _, err := Wire(f); err == nil {
		t.Fatal("wire overflow succeeded")
	}
}
func TestFlightRetainsStalledCapacity(t *testing.T) {
	var flight Flight
	if !flight.Acquire() {
		t.Fatal("first admission")
	}
	ctx, cancel := context.WithCancel(context.Background())
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := flight.Run(ctx, func() Reply { close(entered); <-release; return Reply{Status: 200} })
		done <- err
	}()
	<-entered
	cancel()
	if <-done == nil {
		t.Fatal("cancellation missing")
	}
	for range 100 {
		if flight.Acquire() {
			t.Fatal("stalled worker lost admission")
		}
	}
	close(release)
	until := time.Now().Add(time.Second)
	for !flight.Acquire() {
		if time.Now().After(until) {
			t.Fatal("slot never released")
		}
		time.Sleep(time.Millisecond)
	}
	flight.Release()
}

func TestMaximumCatalogSeriesAndSize(t *testing.T) {
	totalSeries, totalBytes := 1, 0
	for _, component := range []string{"gateway", "backend"} {
		r := New(component)
		cases := []contract.Result{{}, {Issues: []contract.Issue{{Code: "missing_measurement"}}}, contract.Failure("backend_unavailable"), contract.Failure("unknown"), contract.Failure("timeout"), contract.Failure("cancelled"), contract.Failure("overload"), contract.Failure("policy_denied")}
		for _, tool := range append(append([]string{}, contract.Tools...), "unknown") {
			for _, result := range cases {
				r.Tool(tool, result, time.Millisecond)
			}
			r.Tool(tool, contract.Result{Issues: []contract.Issue{{Code: "missing_measurement"}, {Code: "policy_denied"}, {Code: "response_limit"}, {Code: "invalid_unit"}, {Code: "retention_unverified"}, {Code: "unknown"}}}, time.Millisecond)
		}
		if component == "gateway" {
			for _, route := range []string{"mcp", "metrics"} {
				r.StartHTTP(route)
				r.EndHTTP(route)
				for _, status := range []int{200, 302, 401, 403, 500} {
					r.HTTP(route, status, time.Millisecond)
				}
				r.Reject(route, "overload")
			}
		}
		f, err := r.Gather()
		if err != nil {
			t.Fatal(err)
		}
		body, err := Encode(f)
		if err != nil {
			t.Fatal(err)
		}
		totalBytes += len(body)
		for _, family := range f {
			for _, m := range family.Metric {
				if m.Histogram != nil {
					totalSeries += len(m.Histogram.Bucket) + 3
				} else {
					totalSeries++
				}
			}
		}
		if component == "backend" {
			wire, err := Wire(f)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = DecodeBackend(bytes.NewReader(wire)); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Logf("maximum catalog: %d series, %d bytes before shared HELP/TYPE deduplication and backend-up", totalSeries, totalBytes)
	if totalSeries != 1205 || totalBytes > MaxBytes {
		t.Fatal("catalog ceiling changed", totalSeries, totalBytes)
	}
}
