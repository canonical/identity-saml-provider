// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package prometheus_test

import (
	"testing"

	prommon "github.com/canonical/identity-saml-provider/internal/monitoring/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

type fakeStats struct {
	acquired int32
	idle     int32
	total    int32
	max      int32
}

func (f *fakeStats) AcquiredConns() int32 { return f.acquired }
func (f *fakeStats) IdleConns() int32     { return f.idle }
func (f *fakeStats) TotalConns() int32    { return f.total }
func (f *fakeStats) MaxConns() int32      { return f.max }

func TestPoolCollector_Collect(t *testing.T) {
	t.Parallel()

	stats := &fakeStats{acquired: 3, idle: 7, total: 10, max: 20}
	statFn := func() prommon.PoolStats { return stats }

	reg := prometheus.NewRegistry()
	collector := prommon.NewPoolCollector("test-svc", statFn)
	reg.MustRegister(collector)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	want := map[string]float64{
		"db_pool_acquired_connections": 3,
		"db_pool_idle_connections":     7,
		"db_pool_total_connections":    10,
		"db_pool_max_connections":      20,
	}

	found := 0
	for _, mf := range families {
		if expected, ok := want[mf.GetName()]; ok {
			found++
			if len(mf.GetMetric()) != 1 {
				t.Fatalf("%s: expected 1 metric, got %d", mf.GetName(), len(mf.GetMetric()))
			}
			got := mf.GetMetric()[0].GetGauge().GetValue()
			if got != expected {
				t.Errorf("%s = %f, want %f", mf.GetName(), got, expected)
			}
		}
	}

	if found != len(want) {
		t.Errorf("found %d metric families, want %d", found, len(want))
	}
}

func TestPoolCollector_Describe(t *testing.T) {
	t.Parallel()

	stats := &fakeStats{}
	statFn := func() prommon.PoolStats { return stats }
	collector := prommon.NewPoolCollector("test-svc", statFn)

	ch := make(chan *prometheus.Desc, 10)
	collector.Describe(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}

	if count != 4 {
		t.Errorf("descriptor count = %d, want 4", count)
	}
}
