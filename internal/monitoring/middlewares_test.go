package monitoring_test

import (
	"github.com/canonical/identity-saml-provider/internal/monitoring"
	"github.com/canonical/identity-saml-provider/mocks"
	"github.com/go-chi/chi/v5"
	"go.uber.org/mock/gomock"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetricsMiddleware_RecordsBothMetrics(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mon := mocks.NewMockMonitorInterface(ctrl)
	mon.EXPECT().ObserveHTTPRequestDuration("GET", "/test", "200", gomock.Any())
	mon.EXPECT().IncrementHTTPRequestsTotal("GET", "/test", "200")
	mdw := monitoring.NewMiddleware(mon)
	r := chi.NewRouter()
	r.Use(mdw.Metrics())
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
func TestMetricsMiddleware_NilMonitor_NoPanic(t *testing.T) {
	t.Parallel()
	mdw := monitoring.NewMiddleware(nil)
	handler := mdw.Metrics()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
func TestMetricsMiddleware_UsesRoutePattern(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mon := mocks.NewMockMonitorInterface(ctrl)
	mon.EXPECT().ObserveHTTPRequestDuration("GET", "/items/{id}", "200", gomock.Any())
	mon.EXPECT().IncrementHTTPRequestsTotal("GET", "/items/{id}", "200")
	mdw := monitoring.NewMiddleware(mon)
	r := chi.NewRouter()
	r.Use(mdw.Metrics())
	r.Get("/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/items/42", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
