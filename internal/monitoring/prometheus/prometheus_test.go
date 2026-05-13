package prometheus_test

import (
	"testing"

	prommon "github.com/canonical/identity-saml-provider/internal/monitoring/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestNewMonitor_RegistersWithoutPanic(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	_ = prommon.NewMonitor("test-svc", reg)
}

func TestObserveHTTPRequestDuration(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	m := prommon.NewMonitor("test-svc", reg)

	m.ObserveHTTPRequestDuration("GET", "/saml/metadata", "200", 0.042)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	mf := findFamily(families, "http_server_request_duration_seconds")
	if mf == nil {
		t.Fatal("metric family not found")
	}
	if len(mf.GetMetric()) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(mf.GetMetric()))
	}

	metric := mf.GetMetric()[0]
	assertLabel(t, metric, "method", "GET")
	assertLabel(t, metric, "route", "/saml/metadata")
	assertLabel(t, metric, "status", "200")

	if got := metric.GetHistogram().GetSampleCount(); got != 1 {
		t.Errorf("sample count = %d, want 1", got)
	}
}

func TestIncrementHTTPRequestsTotal(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	m := prommon.NewMonitor("test-svc", reg)

	m.IncrementHTTPRequestsTotal("POST", "/admin/service-providers", "201")
	m.IncrementHTTPRequestsTotal("POST", "/admin/service-providers", "201")

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	mf := findFamily(families, "http_server_requests_total")
	if mf == nil {
		t.Fatal("metric family not found")
	}

	metric := mf.GetMetric()[0]
	assertLabel(t, metric, "method", "POST")
	assertLabel(t, metric, "status", "201")

	if got := metric.GetCounter().GetValue(); got != 2 {
		t.Errorf("counter = %f, want 2", got)
	}
}

func TestIncrementBridgeOperation(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	m := prommon.NewMonitor("test-svc", reg)

	m.IncrementBridgeOperation("oidc_code_exchange", "success")

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	mf := findFamily(families, "bridge_operations_total")
	if mf == nil {
		t.Fatal("metric family not found")
	}

	metric := mf.GetMetric()[0]
	assertLabel(t, metric, "operation", "oidc_code_exchange")
	assertLabel(t, metric, "result", "success")

	if got := metric.GetCounter().GetValue(); got != 1 {
		t.Errorf("counter = %f, want 1", got)
	}
}

func TestNewMonitor_DoubleRegistration_Panics(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	_ = prommon.NewMonitor("test-svc", reg)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on double registration")
		}
	}()

	_ = prommon.NewMonitor("test-svc", reg)
}

func findFamily(families []*dto.MetricFamily, name string) *dto.MetricFamily {
	for _, mf := range families {
		if mf.GetName() == name {
			return mf
		}
	}
	return nil
}

func assertLabel(t *testing.T, m *dto.Metric, name, want string) {
	t.Helper()
	for _, lp := range m.GetLabel() {
		if lp.GetName() == name {
			if lp.GetValue() != want {
				t.Errorf("label %q = %q, want %q", name, lp.GetValue(), want)
			}
			return
		}
	}
	t.Errorf("label %q not found", name)
}
