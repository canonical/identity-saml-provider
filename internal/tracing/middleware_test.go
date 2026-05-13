package tracing_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/tracing"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestMiddleware_OpenTelemetry(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{
			name:       "GET request produces span",
			method:     http.MethodGet,
			path:       "/test",
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST request produces span",
			method:     http.MethodPost,
			path:       "/submit",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(
				sdktrace.WithSyncer(exporter),
				sdktrace.WithSampler(sdktrace.AlwaysSample()),
			)
			defer tp.Shutdown(context.Background()) //nolint:errcheck
			otel.SetTracerProvider(tp)

			mdw := tracing.NewTracingMiddleware()

			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := mdw.OpenTelemetry(inner)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			tp.ForceFlush(context.Background()) //nolint:errcheck

			spans := exporter.GetSpans()
			if len(spans) == 0 {
				t.Fatal("expected at least one span from OpenTelemetry middleware")
			}
		})
	}
}

func TestMiddleware_RouteSpanNameMiddleware(t *testing.T) {
	tests := []struct {
		name         string
		pattern      string
		requestPath  string
		method       string
		wantSpanName string
	}{
		{
			name:         "parameterised route",
			pattern:      "/users/{id}",
			requestPath:  "/users/123",
			method:       http.MethodGet,
			wantSpanName: "GET /users/{id}",
		},
		{
			name:         "static route",
			pattern:      "/healthz",
			requestPath:  "/healthz",
			method:       http.MethodGet,
			wantSpanName: "GET /healthz",
		},
		{
			name:         "POST method",
			pattern:      "/items",
			requestPath:  "/items",
			method:       http.MethodPost,
			wantSpanName: "POST /items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(
				sdktrace.WithSyncer(exporter),
				sdktrace.WithSampler(sdktrace.AlwaysSample()),
			)
			defer tp.Shutdown(context.Background()) //nolint:errcheck
			otel.SetTracerProvider(tp)

			mdw := tracing.NewTracingMiddleware()

			r := chi.NewRouter()
			r.Use(mdw.RouteSpanNameMiddleware())
			r.MethodFunc(tt.method, tt.pattern, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := mdw.OpenTelemetry(r)

			req := httptest.NewRequest(tt.method, tt.requestPath, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			tp.ForceFlush(context.Background()) //nolint:errcheck

			spans := exporter.GetSpans()
			found := false
			for _, s := range spans {
				if s.Name == tt.wantSpanName {
					found = true
					break
				}
			}
			if !found {
				names := make([]string, len(spans))
				for i, s := range spans {
					names[i] = s.Name
				}
				t.Errorf("expected span named %q, got spans: %v", tt.wantSpanName, names)
			}
		})
	}
}

func TestMiddleware_RouteSpanNameMiddleware_NoRecordingSpan(t *testing.T) {
	// When there's no recording span, the middleware should not panic.
	mdw := tracing.NewTracingMiddleware()

	r := chi.NewRouter()
	r.Use(mdw.RouteSpanNameMiddleware())
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	// Serve without the otelhttp wrapper — no span exists.
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
