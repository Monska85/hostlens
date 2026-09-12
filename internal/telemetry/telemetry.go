// Package telemetry owns bounded service observations, never inspected host data.
package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"runtime/metrics"
	"slices"
	"sync/atomic"
	"time"

	"github.com/Monska85/hostlens/internal/contract"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

const MaxBytes = 1 << 20
const ScrapeTimeout = 5 * time.Second
const BackendTimeout = 2 * time.Second
const ContentType = "text/plain; version=0.0.4; charset=utf-8"

var processStarted = time.Now()

var Buckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2, 5, 10, 30, 60}
var outcomes = []string{"success", "partial", "unavailable", "error", "timeout", "cancelled", "overload", "denied"}
var gaps = []string{"unavailable", "denied", "limit", "invalid", "coverage", "other"}

type Registry struct {
	registry     *prometheus.Registry
	requests     *prometheus.CounterVec
	duration     *prometheus.HistogramVec
	active       *prometheus.GaugeVec
	tools        *prometheus.CounterVec
	toolDuration *prometheus.HistogramVec
	toolActive   prometheus.Gauge
	gaps         *prometheus.CounterVec
	failures     *prometheus.CounterVec
}

func New(component string) *Registry {
	r := &Registry{registry: prometheus.NewRegistry()}
	reg := prometheus.WrapRegistererWith(prometheus.Labels{"component": component}, r.registry)
	r.requests = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hostlens_http_requests_total", Help: "Completed HTTP requests by route and status class."}, []string{"route", "status"})
	r.duration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "hostlens_http_request_duration_seconds", Help: "HTTP request duration in seconds.", Buckets: Buckets}, []string{"route"})
	r.active = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "hostlens_http_requests_active", Help: "Active admitted HTTP work."}, []string{"route"})
	r.failures = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hostlens_http_rejections_total", Help: "HTTP authentication, authorization and admission rejections."}, []string{"route", "reason"})
	if component == "gateway" {
		reg.MustRegister(r.requests, r.duration, r.active, r.failures)
	}
	r.tools = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hostlens_tool_calls_total", Help: "Completed diagnostic calls at this component boundary."}, []string{"tool", "outcome"})
	r.toolDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "hostlens_tool_duration_seconds", Help: "Diagnostic call duration in seconds.", Buckets: Buckets}, []string{"tool"})
	r.toolActive = prometheus.NewGauge(prometheus.GaugeOpts{Name: "hostlens_tool_work_active", Help: "Unfinished diagnostic work owned by this component."})
	r.gaps = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hostlens_collection_gaps_total", Help: "Calls reporting each evidence gap category, counted once per call."}, []string{"tool", "reason"})
	reg.MustRegister(r.tools, r.toolDuration, r.toolActive, r.gaps)
	reg.MustRegister(runtimeCollector{start: float64(processStarted.UnixNano()) / 1e9})
	return r
}
func toolName(name string) string {
	if slices.Contains(contract.Tools, name) {
		return name
	}
	return "unknown"
}
func (r *Registry) StartTool() { r.toolActive.Inc() }
func (r *Registry) EndTool()   { r.toolActive.Dec() }
func (r *Registry) Tool(name string, result contract.Result, elapsed time.Duration) {
	name = toolName(name)
	outcome := "success"
	if result.Error {
		outcome = "error"
	} else if len(result.Issues) > 0 || result.Truncated {
		outcome = "partial"
	}
	seen := map[string]bool{}
	for _, issue := range result.Issues {
		reason := "other"
		switch issue.Code {
		case "timeout", "cancelled_or_timeout":
			outcome = "timeout"
		case "cancelled":
			outcome = "cancelled"
		case "overload":
			outcome = "overload"
		case "policy_denied":
			reason = "denied"
			if result.Error {
				outcome = "denied"
			}
		case "source_denied_or_unavailable", "missing_measurement", "not_found", "backend_unavailable", "generation_mismatch":
			reason = "unavailable"
			if result.Error {
				outcome = "unavailable"
			}
		case "inspection_limit", "response_limit", "size_or_read_failure":
			reason = "limit"
		case "invalid_bounds", "invalid_filter", "invalid_journal_record", "invalid_service_record", "invalid_unit", "invalid_window", "unsupported_encoding", "unsupported_filter", "unsupported_parser", "unsupported_operation":
			reason = "invalid"
		case "retention_unverified", "unverified_time_coverage":
			reason = "coverage"
		}
		seen[reason] = true
	}
	if result.Truncated {
		seen["limit"] = true
	}
	for reason := range seen {
		r.gaps.WithLabelValues(name, reason).Inc()
	}
	r.tools.WithLabelValues(name, outcome).Inc()
	r.toolDuration.WithLabelValues(name).Observe(elapsed.Seconds())
}
func (r *Registry) Reject(route, reason string) { r.failures.WithLabelValues(route, reason).Inc() }
func (r *Registry) StartHTTP(route string)      { r.active.WithLabelValues(route).Inc() }
func (r *Registry) EndHTTP(route string)        { r.active.WithLabelValues(route).Dec() }
func (r *Registry) HTTP(route string, status int, elapsed time.Duration) {
	class := "5xx"
	switch {
	case status < 300:
		class = "2xx"
	case status < 400:
		class = "3xx"
	case status < 500:
		class = "4xx"
	}
	r.requests.WithLabelValues(route, class).Inc()
	r.duration.WithLabelValues(route).Observe(elapsed.Seconds())
	reason := ""
	switch status {
	case 401:
		reason = "authentication"
	case 403:
		reason = "authorization"
	}
	if reason != "" {
		r.failures.WithLabelValues(route, reason).Inc()
	}
}
func (r *Registry) Gather() ([]*dto.MetricFamily, error) { return r.registry.Gather() }

type runtimeCollector struct{ start float64 }

func (r runtimeCollector) Describe(ch chan<- *prometheus.Desc) { prometheus.DescribeByCollect(r, ch) }
func (r runtimeCollector) Collect(ch chan<- prometheus.Metric) {
	emit := func(name, help string, kind prometheus.ValueType, value float64) {
		ch <- prometheus.MustNewConstMetric(prometheus.NewDesc(name, help, nil, nil), kind, value)
	}
	samples := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}, {Name: "/memory/classes/total:bytes"}, {Name: "/sched/goroutines:goroutines"}, {Name: "/gc/cycles/total:gc-cycles"}}
	metrics.Read(samples)
	names := []string{"hostlens_go_heap_objects_bytes", "hostlens_go_memory_bytes", "hostlens_go_goroutines", "hostlens_go_gc_cycles_total"}
	helps := []string{"Bytes occupied by live or unswept heap objects.", "Bytes mapped by the Go runtime.", "Current Go goroutines.", "Completed garbage collection cycles."}
	for i, s := range samples {
		if s.Value.Kind() == metrics.KindUint64 {
			kind := prometheus.GaugeValue
			if i == 3 {
				kind = prometheus.CounterValue
			}
			emit(names[i], helps[i], kind, float64(s.Value.Uint64()))
		}
	}
	emit("hostlens_process_start_time_seconds", "Process telemetry initialization time in Unix seconds.", prometheus.GaugeValue, r.start)
	processMetrics(emit)
}

// Flight reserves capacity until both underlying work and its caller finish.
// Rejected requests never start a worker. A stuck collector retains its slot.
type Flight struct{ busy atomic.Bool }

func (f *Flight) Acquire() bool { return f.busy.CompareAndSwap(false, true) }
func (f *Flight) Release()      { f.busy.Store(false) }

type Reply struct {
	Body   []byte
	Status int
}

func (f *Flight) Run(ctx context.Context, work func() Reply) (Reply, error) {
	done := make(chan Reply, 1)
	go func() { defer f.Release(); done <- work(); <-ctx.Done() }()
	select {
	case result := <-done:
		return result, nil
	case <-ctx.Done():
		return Reply{}, ctx.Err()
	}
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > MaxBytes {
		return 0, errors.New("telemetry size limit")
	}
	return b.Buffer.Write(p)
}
func Encode(families []*dto.MetricFamily) ([]byte, error) {
	b := &limitedBuffer{}
	for _, family := range families {
		if _, err := expfmt.MetricFamilyToText(b, family); err != nil {
			return nil, err
		}
	}
	return b.Bytes(), nil
}
func Wire(families []*dto.MetricFamily) ([]byte, error) {
	b := &limitedBuffer{}
	err := json.NewEncoder(b).Encode(families)
	return b.Bytes(), err
}
func BackendUp(up bool) *dto.MetricFamily {
	value := 0.
	if up {
		value = 1
	}
	name := "hostlens_backend_up"
	help := "Backend telemetry reachable; not diagnostic readiness."
	kind := dto.MetricType_GAUGE
	return &dto.MetricFamily{Name: &name, Help: &help, Type: &kind, Metric: []*dto.Metric{{Gauge: &dto.Gauge{Value: &value}}}}
}

// DecodeBackend accepts only the fixed backend catalog, with no metadata or
// dimensions supplied by an inspected application. Re-encoding discards wire bytes.
func DecodeBackend(reader io.Reader) ([]*dto.MetricFamily, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxBytes+1))
	if err != nil || len(data) > MaxBytes {
		return nil, errors.New("invalid telemetry size")
	}
	var families []*dto.MetricFamily
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(&families); err != nil {
		return nil, err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, errors.New("trailing telemetry")
	}
	if len(families) == 0 || len(families) > 11 {
		return nil, errors.New("invalid telemetry catalog")
	}
	seen := map[string]bool{}
	for _, f := range families {
		if f == nil || seen[f.GetName()] || f.Unit != nil {
			return nil, errors.New("duplicate or invalid family")
		}
		seen[f.GetName()] = true
		labels := []string{"component"}
		kind := dto.MetricType_GAUGE
		switch f.GetName() {
		case "hostlens_tool_calls_total":
			kind = dto.MetricType_COUNTER
			labels = append(labels, "tool", "outcome")
		case "hostlens_tool_duration_seconds":
			kind = dto.MetricType_HISTOGRAM
			labels = append(labels, "tool")
		case "hostlens_collection_gaps_total":
			kind = dto.MetricType_COUNTER
			labels = append(labels, "tool", "reason")
		case "hostlens_go_gc_cycles_total", "hostlens_process_cpu_seconds_total":
			kind = dto.MetricType_COUNTER
		case "hostlens_tool_work_active", "hostlens_go_heap_objects_bytes", "hostlens_go_memory_bytes", "hostlens_go_goroutines", "hostlens_process_start_time_seconds", "hostlens_process_max_resident_bytes":
		default:
			return nil, errors.New("unexpected telemetry family")
		}
		if f.Type == nil || f.GetType() != kind || len(f.Metric) == 0 || len(f.Metric) > 19*len(outcomes) || len(f.GetHelp()) > 128 {
			return nil, errors.New("invalid telemetry family")
		}
		series := map[string]bool{}
		for _, m := range f.Metric {
			if m == nil || len(m.Label) != len(labels) || m.TimestampMs != nil || m.Summary != nil || m.Untyped != nil {
				return nil, errors.New("invalid telemetry sample")
			}
			values := map[string]string{}
			for _, l := range m.Label {
				if l == nil || !slices.Contains(labels, l.GetName()) || values[l.GetName()] != "" {
					return nil, errors.New("unexpected label")
				}
				v := l.GetValue()
				ok := false
				switch l.GetName() {
				case "component":
					ok = v == "backend"
				case "tool":
					ok = v == toolName(v)
				case "outcome":
					ok = slices.Contains(outcomes, v)
				case "reason":
					ok = slices.Contains(gaps, v)
				}
				if !ok {
					return nil, errors.New("unexpected label value")
				}
				values[l.GetName()] = v
			}
			key := ""
			for _, l := range labels {
				key += values[l] + "/"
			}
			if series[key] {
				return nil, errors.New("duplicate sample")
			}
			series[key] = true
			valid := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 }
			switch kind {
			case dto.MetricType_GAUGE:
				if m.Gauge == nil || m.Gauge.Value == nil || m.Counter != nil || m.Histogram != nil || !valid(m.Gauge.GetValue()) {
					return nil, errors.New("invalid gauge")
				}
			case dto.MetricType_COUNTER:
				if m.Counter == nil || m.Counter.Value == nil || m.Gauge != nil || m.Histogram != nil || m.Counter.Exemplar != nil || !valid(m.Counter.GetValue()) {
					return nil, errors.New("invalid counter")
				}
				m.Counter.CreatedTimestamp = nil
			case dto.MetricType_HISTOGRAM:
				h := m.Histogram
				if h == nil || h.SampleCount == nil || h.SampleSum == nil || h.SampleCountFloat != nil || h.ZeroThreshold != nil || h.ZeroCount != nil || h.ZeroCountFloat != nil || len(h.PositiveDelta) > 0 || len(h.PositiveCount) > 0 || len(h.NegativeDelta) > 0 || len(h.NegativeCount) > 0 || m.Gauge != nil || m.Counter != nil || len(h.Bucket) != len(Buckets) || !valid(h.GetSampleSum()) || h.Schema != nil || len(h.PositiveSpan) > 0 || len(h.NegativeSpan) > 0 || len(h.Exemplars) > 0 {
					return nil, errors.New("invalid histogram")
				}
				var previous uint64
				for i, b := range h.Bucket {
					if b == nil || b.CumulativeCount == nil || b.CumulativeCountFloat != nil || b.UpperBound == nil || b.GetUpperBound() != Buckets[i] || b.Exemplar != nil || b.GetCumulativeCount() < previous || b.GetCumulativeCount() > h.GetSampleCount() {
						return nil, errors.New("invalid bucket")
					}
					previous = b.GetCumulativeCount()
				}
				// Rebuild to exclude optional native-histogram fields and timestamps.
				m.Histogram = &dto.Histogram{SampleCount: h.SampleCount, SampleSum: h.SampleSum, Bucket: h.Bucket}
			}
		}
		// Help is implementation metadata, never forward peer-controlled prose.
		help := "HostLens backend service observation."
		f.Help = &help
	}
	if !seen["hostlens_process_start_time_seconds"] {
		return nil, errors.New("missing backend runtime")
	}
	return families, nil
}
