// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package prometheus

import "github.com/prometheus/client_golang/prometheus"

// PoolStats abstracts the subset of pgxpool.Stat data needed by the
// collector. *pgxpool.Stat satisfies this interface.
type PoolStats interface {
	AcquiredConns() int32
	IdleConns() int32
	TotalConns() int32
	MaxConns() int32
}

// PoolStatFunc is a function that returns pool statistics. It decouples
// the collector from the concrete pgxpool.Pool type. In production, pass
// a closure like:
//
//	func() prometheus.PoolStats { return pool.Stat() }
type PoolStatFunc func() PoolStats

// PoolCollector exports pgxpool connection pool metrics at scrape time.
// It implements prometheus.Collector.
type PoolCollector struct {
	statFn PoolStatFunc

	acquiredConns *prometheus.Desc
	idleConns     *prometheus.Desc
	totalConns    *prometheus.Desc
	maxConns      *prometheus.Desc
}

// NewPoolCollector creates a collector using the provided stat function.
// Register it on a prometheus.Registerer.
func NewPoolCollector(service string, statFn PoolStatFunc) *PoolCollector {
	constLabels := prometheus.Labels{"service": service}

	return &PoolCollector{
		statFn: statFn,
		acquiredConns: prometheus.NewDesc(
			"db_pool_acquired_connections",
			"Number of currently acquired connections.",
			nil, constLabels,
		),
		idleConns: prometheus.NewDesc(
			"db_pool_idle_connections",
			"Number of idle connections in the pool.",
			nil, constLabels,
		),
		totalConns: prometheus.NewDesc(
			"db_pool_total_connections",
			"Total number of connections in the pool.",
			nil, constLabels,
		),
		maxConns: prometheus.NewDesc(
			"db_pool_max_connections",
			"Maximum number of connections allowed.",
			nil, constLabels,
		),
	}
}

// Describe sends the descriptor set to the channel.
func (c *PoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.acquiredConns
	ch <- c.idleConns
	ch <- c.totalConns
	ch <- c.maxConns
}

// Collect reads the pool stats and sends gauge values.
func (c *PoolCollector) Collect(ch chan<- prometheus.Metric) {
	stat := c.statFn()

	ch <- prometheus.MustNewConstMetric(
		c.acquiredConns, prometheus.GaugeValue,
		float64(stat.AcquiredConns()),
	)
	ch <- prometheus.MustNewConstMetric(
		c.idleConns, prometheus.GaugeValue,
		float64(stat.IdleConns()),
	)
	ch <- prometheus.MustNewConstMetric(
		c.totalConns, prometheus.GaugeValue,
		float64(stat.TotalConns()),
	)
	ch <- prometheus.MustNewConstMetric(
		c.maxConns, prometheus.GaugeValue,
		float64(stat.MaxConns()),
	)
}
