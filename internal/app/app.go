package app

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/canonical/identity-saml-provider/internal/handler"
	"github.com/canonical/identity-saml-provider/internal/infrastructure/hydra"
	"github.com/canonical/identity-saml-provider/internal/infrastructure/samlkit"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/monitoring"
	"github.com/canonical/identity-saml-provider/internal/monitoring/prometheus"
	"github.com/canonical/identity-saml-provider/internal/repository/memory"
	"github.com/canonical/identity-saml-provider/internal/repository/postgres"
	"github.com/canonical/identity-saml-provider/internal/service"
	"github.com/canonical/identity-saml-provider/internal/tracing"
)

// App holds the fully wired application.
type App struct {
	HTTPServer *http.Server
	Tracer     tracing.TracingInterface
	Pool       *pgxpool.Pool
}

// Build constructs the application from the given configuration.
func Build(ctx context.Context, cfg Config, logger *logging.ZapLogger) (*App, error) {
	// --- Database (pgxpool) ---
	pool, err := postgres.NewPool(ctx, cfg.PoolConfig())
	if err != nil {
		return nil, err
	}

	// --- Repositories ---
	sessionRepo := postgres.NewSessionRepo(pool)
	spRepo := postgres.NewServiceProviderRepo(pool)
	pendingRepo := memory.NewPendingRequestRepo()

	// --- Infrastructure ---
	hydraClient, err := hydra.NewClient(ctx, cfg.HydraConfig(), cfg.OIDCConfig(), logger)
	if err != nil {
		pool.Close()
		return nil, err
	}

	samlIDP, err := samlkit.NewIdentityProvider(cfg.SAMLConfig(), logger)
	if err != nil {
		pool.Close()
		return nil, err
	}

	// --- Monitoring & Tracing ---
	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = "identity-saml-provider"
	}
	monitor := prometheus.NewMonitor(serviceName, logger)

	tracingCfg := tracing.NewConfig(
		cfg.TracingEnabled,
		cfg.OtelGRPCEndpoint,
		cfg.OtelHTTPEndpoint,
		cfg.OtelSampler,
		cfg.OtelSamplerRatio,
		logger,
	)
	tracer := tracing.NewTracer(tracingCfg)

	// --- Services ---
	sessionSvc := service.NewSessionService(sessionRepo, logger)
	spSvc := service.NewServiceProviderService(spRepo, logger)
	mappingSvc := service.NewMappingService(spRepo, logger)
	oidcSvc := service.NewOIDCService(hydraClient, logger)
	pendingSvc := service.NewPendingRequestService(pendingRepo, logger)

	// --- Handlers ---
	handlers := handler.NewHandlers(
		sessionSvc, spSvc, mappingSvc, oidcSvc, pendingSvc,
		samlIDP,
		handler.HandlerConfig{BridgeBaseURL: cfg.BridgeBaseURL},
		logger, monitor, tracer,
	)

	// Wire SAML adapters (these need the services)
	samlIDP.SessionProvider = &handler.SAMLSessionAdapter{
		Sessions: sessionSvc,
		Mapping:  mappingSvc,
		Pending:  pendingSvc,
		OIDC:     oidcSvc,
		Config:   handler.HandlerConfig{BridgeBaseURL: cfg.BridgeBaseURL},
		Logger:   logger,
	}
	samlIDP.ServiceProviderProvider = &handler.SAMLSPAdapter{
		SPs: spSvc,
	}

	// --- Health Handler ---
	healthHandler := handler.NewHealthHandler(pool)

	// --- HTTP Server ---
	router := chi.NewRouter()

	// Health probes — registered on the root router without
	// middleware so they are lightweight and do not appear in
	// metrics/logs.
	router.Get("/healthz", healthHandler.HandleHealthz)
	router.Get("/readyz", healthHandler.HandleReadyz)

	// Business routes — wrapped in a Group so middleware only
	// applies to these routes, not to health probes.
	router.Group(func(r chi.Router) {
		// Apply middleware (order matters):
		// 1. RequestID: generates or reads X-Request-ID for each request
		// 2. Tracing: sets span names after routing
		// 3. Request logger: enriches context logger with requestID/traceID, logs each request
		// 4. Response time: records Prometheus metrics
		r.Use(middleware.RequestID)
		r.Use(tracing.NewMiddleware(monitor, logger).RouteSpanNameMiddleware())
		r.Use(logging.RequestLoggerMiddleware(logger))
		r.Use(monitoring.NewMiddleware(monitor, logger).ResponseTime())

		handlers.RegisterRoutes(r)
	})

	otelHandler := tracing.NewMiddleware(monitor, logger).OpenTelemetry(router)
	httpServer := &http.Server{
		Addr:    ":" + cfg.BridgeBasePort,
		Handler: otelHandler,
	}

	return &App{
		HTTPServer: httpServer,
		Tracer:     tracer,
		Pool:       pool,
	}, nil
}
