// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package prometheus

import (
	"github.com/canonical/identity-saml-provider/internal/monitoring"
	"github.com/prometheus/client_golang/prometheus"
)

// Monitor implements monitoring.MonitorInterface backed by Prometheus
// metrics. All metrics carry "service" as a constant label set at
// construction time.
type Monitor struct {
	requestDuration *prometheus.HistogramVec
	requestsTotal   *prometheus.CounterVec
	bridgeOpsTotal  *prometheus.CounterVec
}

var _ monitoring.MonitorInterface = (*Monitor)(nil)

// NewMonitor creates a Monitor and registers Prometheus metrics on the
// provided registerer. Pass prometheus.DefaultRegisterer for production;
// pass a prometheus.NewRegistry() in tests to avoid global state leaks.
func NewMonitor(service string, registerer prometheus.Registerer) *Monitor {
	constLabels := prometheus.Labels{"service": service}

	m := &Monitor{
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "http_server_request_duration_seconds",
				Help:        "HTTP request duration in seconds.",
				ConstLabels: constLabels,
			},
			[]string{"method", "route", "status"},
		),
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "http_server_requests_total",
				Help:        "Total number of HTTP requests.",
				ConstLabels: constLabels,
			},
			[]string{"method", "route", "status"},
		),
		bridgeOpsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "bridge_operations_total",
				Help:        "Total bridge operations by operation and result.",
				ConstLabels: constLabels,
			},
			[]string{"operation", "result"},
		),
	}

	registerer.MustRegister(
		m.requestDuration,
		m.requestsTotal,
		m.bridgeOpsTotal,
	)

	return m
}

func (m *Monitor) ObserveHTTPRequestDuration(method, route, status string, durationSeconds float64) {
	m.requestDuration.WithLabelValues(method, route, status).Observe(durationSeconds)
}

func (m *Monitor) IncrementHTTPRequestsTotal(method, route, status string) {
	m.requestsTotal.WithLabelValues(method, route, status).Inc()
}

func (m *Monitor) IncrementBridgeOperation(operation, result string) {
	m.bridgeOpsTotal.WithLabelValues(operation, result).Inc()
}
