//go:build !linux

package telemetry

import "github.com/prometheus/client_golang/prometheus"

// Native process measurements are omitted on unsupported platforms.
func processMetrics(func(string, string, prometheus.ValueType, float64)) {}
