package tracing

import (
	"net/http"
	"strings"

	"github.com/canonical/identity-saml-provider/internal/monitoring"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Middleware struct {
	service string

	monitor monitoring.MonitorInterface
	logger  *zap.SugaredLogger
}

func (mdw *Middleware) OpenTelemetry(handler http.Handler) http.Handler {
	if router, ok := handler.(chi.Router); ok {
		router.Use(mdw.routeSpanNameMiddleware)
	}

	return otelhttp.NewHandler(
		handler,
		"server",
	)
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

func NewMiddleware(monitor monitoring.MonitorInterface, logger *zap.SugaredLogger) *Middleware {
	mdw := new(Middleware)
	mdw.monitor = monitor
	mdw.logger = logger
	if monitor != nil {
		mdw.service = monitor.GetService()
	}

	return mdw
}
