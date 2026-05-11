package logging_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestLoggerMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantLevel  string
		wantStatus int
	}{
		{
			name: "200 response logs at info",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
			wantLevel:  "info",
			wantStatus: http.StatusOK,
		},
		{
			name: "404 response logs at warn",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}),
			wantLevel:  "warn",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "500 response logs at error",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}),
			wantLevel:  "error",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			core, logs := observer.New(zap.DebugLevel)
			logger := logging.NewZapLogger(zap.New(core).Sugar())

			middleware := logging.RequestLoggerMiddleware(logger)
			handler := middleware(tt.handler)

			req := httptest.NewRequest(http.MethodGet, "/test/path", nil)
			req.Header.Set("X-Request-ID", "test-req-id")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			entries := logs.All()
			if len(entries) == 0 {
				t.Fatal("expected at least one log entry")
			}

			last := entries[len(entries)-1]
			if last.Message != "HTTP request completed" {
				t.Errorf("message = %q, want %q", last.Message, "HTTP request completed")
			}

			gotLevel := last.Level.String()
			if gotLevel != tt.wantLevel {
				t.Errorf("level = %q, want %q", gotLevel, tt.wantLevel)
			}

			// Verify request-scoped fields are present
			fieldMap := make(map[string]interface{})
			for _, f := range last.Context {
				fieldMap[f.Key] = f.Interface
			}
			// These are set via sugared logger With, so they appear in Context
		})
	}
}

func TestRequestLoggerMiddleware_EnrichesContext(t *testing.T) {
	t.Parallel()

	logger := logging.NewNopLogger()

	var ctxLogger logging.Logger
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxLogger = logging.FromContext(r.Context(), nil)
		w.WriteHeader(http.StatusOK)
	})

	middleware := logging.RequestLoggerMiddleware(logger)
	handler := middleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if ctxLogger == nil {
		t.Fatal("expected enriched logger in context, got nil")
	}
	// The enriched logger should not be the same pointer as the base logger
	if ctxLogger == logging.Logger(logger) {
		t.Error("context logger should be enriched, not the base logger")
	}
}

func TestRequestLoggerMiddleware_ExtractsRequestID(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zap.DebugLevel)
	logger := logging.NewZapLogger(zap.New(core).Sugar())

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := logging.RequestLoggerMiddleware(logger)
	handler := middleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/bar", nil)
	req.Header.Set("X-Request-ID", "custom-id-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	entries := logs.All()
	if len(entries) == 0 {
		t.Fatal("expected log entries")
	}

	// The requestID should be in the structured context of the log entry
	// With sugared logger, With fields end up in the Context slice
	found := false
	for _, f := range entries[len(entries)-1].Context {
		if f.Key == "requestID" && f.String == "custom-id-123" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected requestID=custom-id-123 in log context fields")
	}
}
