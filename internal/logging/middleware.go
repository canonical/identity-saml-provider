// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package logging

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"
)

// RequestLoggerMiddleware returns a chi-compatible middleware that logs
// each HTTP request with method, path, status, duration, and request/trace IDs.
// It also stores a request-enriched logger in the context for downstream use.
func RequestLoggerMiddleware(logger Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Generate or extract request ID
			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = middleware.GetReqID(r.Context())
			}

			// Enrich logger with request-scoped fields
			enriched := logger.With(
				"requestID", reqID,
				"method", r.Method,
				"path", r.URL.Path,
			)

			// Extract trace ID from OpenTelemetry span context if available
			if span := trace.SpanFromContext(r.Context()); span.SpanContext().IsValid() {
				enriched = enriched.With("traceID", span.SpanContext().TraceID().String())
			}

			// Store enriched logger in context for downstream handlers/services
			ctx := WithLogger(r.Context(), enriched)
			next.ServeHTTP(ww, r.WithContext(ctx))

			// Log the completed request
			duration := time.Since(start)
			status := ww.Status()

			fields := []interface{}{
				"status", status,
				"duration", duration.String(),
				"bytes", ww.BytesWritten(),
			}

			switch {
			case status >= 500:
				enriched.Errorw("HTTP request completed", fields...)
			case status >= 400:
				enriched.Warnw("HTTP request completed", fields...)
			default:
				enriched.Infow("HTTP request completed", fields...)
			}
		})
	}
}
