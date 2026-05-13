package tracing

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

// Middleware provides OpenTelemetry tracing middleware for HTTP handlers.
type Middleware struct{}

func (mdw *Middleware) OpenTelemetry(handler http.Handler) http.Handler {
	return otelhttp.NewHandler(
		handler,
		"server",
	)
}

func (mdw *Middleware) RouteSpanNameMiddleware() func(http.Handler) http.Handler {
	return mdw.routeSpanNameMiddleware
}

func (mdw *Middleware) routeSpanNameMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)

		span := trace.SpanFromContext(r.Context())
		if !span.IsRecording() {
			return
		}

		span.SetName(mdw.spanName(r))
	})
}

func (mdw *Middleware) spanName(r *http.Request) string {
	routePattern := r.URL.Path
	if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
		if matched := strings.TrimSpace(routeCtx.RoutePattern()); matched != "" {
			routePattern = matched
		}
	}

	return r.Method + " " + routePattern
}

// NewTracingMiddleware creates a tracing middleware.
func NewTracingMiddleware() *Middleware {
	return &Middleware{}
}
