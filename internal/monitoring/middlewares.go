// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package monitoring

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Middleware records HTTP metrics for each request.
type Middleware struct {
	monitor MonitorInterface
}

// NewMiddleware creates a monitoring middleware.
func NewMiddleware(monitor MonitorInterface) *Middleware {
	return &Middleware{monitor: monitor}
}

// Metrics returns a chi-compatible middleware that observes request
// duration and increments the request counter after every response.
func (mdw *Middleware) Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			next.ServeHTTP(ww, r)

			if mdw.monitor == nil {
				return
			}

			// Use the chi route pattern to keep label cardinality
			// bounded. Falls back to the raw path only if chi has
			// no route context.
			route := r.URL.Path
			if rctx := chi.RouteContext(r.Context()); rctx != nil {
				if pattern := rctx.RoutePattern(); pattern != "" {
					route = pattern
				}
			}

			method := r.Method
			status := fmt.Sprint(ww.Status())
			duration := time.Since(start).Seconds()

			mdw.monitor.ObserveHTTPRequestDuration(method, route, status, duration)
			mdw.monitor.IncrementHTTPRequestsTotal(method, route, status)
		})
	}
}
