package monitoring

import (
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

const (
	// IDPathRegex swaps route placeholders like {id} for a stable "id" label value.
	IDPathRegex string = "{[^}]+}"
)

type Middleware struct {
	service string
	regex   *regexp.Regexp

	monitor MonitorInterface
	logger  *zap.SugaredLogger
}

func (mdw *Middleware) ResponseTime() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
				startTime := time.Now()

				next.ServeHTTP(ww, r)

				if mdw.monitor == nil {
					return
				}

				routePattern := r.URL.Path
				if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
					if matched := routeCtx.RoutePattern(); matched != "" {
						routePattern = matched
					}
				}

				tags := map[string]string{
					"route":  fmt.Sprintf("%s%s", r.Method, mdw.regex.ReplaceAllString(routePattern, "id")),
					"status": fmt.Sprint(ww.Status()),
				}

				_ = mdw.monitor.SetResponseTimeMetric(tags, time.Since(startTime).Seconds())
			},
		)
	}
}

func NewMiddleware(monitor MonitorInterface, logger *zap.SugaredLogger) *Middleware {
	mdw := new(Middleware)
	mdw.monitor = monitor
	mdw.logger = logger
	mdw.regex = regexp.MustCompile(IDPathRegex)

	if monitor != nil {
		mdw.service = monitor.GetService()
	}

	return mdw
}
