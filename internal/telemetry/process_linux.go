package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sys/unix"
)

func processMetrics(emit func(string, string, prometheus.ValueType, float64)) {
	var usage unix.Rusage
	if unix.Getrusage(unix.RUSAGE_SELF, &usage) != nil {
		return
	}
	cpu := float64(usage.Utime.Sec+usage.Stime.Sec) + float64(usage.Utime.Usec+usage.Stime.Usec)/1e6
	emit("hostlens_process_cpu_seconds_total", "Process user and system CPU seconds.", prometheus.CounterValue, cpu)
	emit("hostlens_process_max_resident_bytes", "Process lifetime peak resident memory in bytes.", prometheus.GaugeValue, float64(usage.Maxrss)*1024)
}
