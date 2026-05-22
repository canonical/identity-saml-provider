// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/canonical/identity-saml-provider/internal/handler"
	"github.com/canonical/identity-saml-provider/internal/infrastructure/hydra"
	"github.com/canonical/identity-saml-provider/internal/infrastructure/samlkit"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/monitoring"
	prommon "github.com/canonical/identity-saml-provider/internal/monitoring/prometheus"
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
	logger     *logging.ZapLogger
}

// Shutdown performs an ordered teardown of the application. It first
// shuts down the HTTP server (draining in-flight requests), then
// closes the database pool and tracer exporter, and finally syncs the
// logger. ctx controls the overall deadline for the shutdown.
func (a *App) Shutdown(ctx context.Context) {
	if err := a.HTTPServer.Shutdown(ctx); err != nil {
		a.logger.Errorw("HTTP server shutdown error", "error", err)
	}

	a.Pool.Close()

	if err := a.Tracer.Shutdown(); err != nil {
		a.logger.Warnw("Failed to shutdown tracer", "error", err)
	}

	_ = a.logger.Sync()
}

// Build constructs the application from the given configuration.
func Build(ctx context.Context, cfg Config, logger *logging.ZapLogger) (*App, error) {
	// --- Database (pgxpool) ---
	pool, err := postgres.NewPool(ctx, cfg.PoolConfig())
	if err != nil {
		return nil, err
	}

	// --- Monitoring & Tracing ---
	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = "identity-saml-provider"
	}
	monitor := prommon.NewMonitor(serviceName, prom.DefaultRegisterer)

	// Register pgxpool connection pool collector.
	poolCollector := prommon.NewPoolCollector(serviceName, func() prommon.PoolStats {
		return pool.Stat()
	})
	prom.DefaultRegisterer.MustRegister(poolCollector)

	tracingCfg := tracing.NewConfig(
		cfg.TracingEnabled,
		cfg.OtelGRPCEndpoint,
		cfg.OtelHTTPEndpoint,
		cfg.OtelSampler,
		cfg.OtelSamplerRatio,
		logger,
	)
	tracer := tracing.NewTracer(ctx, tracingCfg)

	// --- Repositories ---
	sessionRepo := postgres.NewSessionRepo(pool, tracer)
	spRepo := postgres.NewServiceProviderRepo(pool, tracer)
	pendingRepo := memory.NewPendingRequestRepo(tracer)

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

	// --- Services ---
	sessionSvc := service.NewSessionService(sessionRepo, logger, tracer)
	spSvc := service.NewServiceProviderService(spRepo, logger, tracer)
	mappingSvc := service.NewMappingService(spRepo, logger, tracer)
	oidcSvc := service.NewOIDCService(hydraClient, logger, tracer)
	pendingSvc := service.NewPendingRequestService(pendingRepo, logger, tracer)

	// --- Handlers ---
	handlers := handler.NewHandlers(
		sessionSvc, spSvc, mappingSvc, oidcSvc, pendingSvc,
		samlIDP,
		handler.HandlerConfig{BridgeBaseURL: cfg.BridgeBaseURL},
		logger, monitor,
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

	// Operational endpoints — registered on the root router without
	// middleware so they are lightweight and do not appear in
	// metrics/logs.
	router.Get("/healthz", healthHandler.HandleHealthz)
	router.Get("/readyz", healthHandler.HandleReadyz)
	router.Handle("/metrics", promhttp.Handler())

	// Business routes — wrapped in a Group so middleware only
	// applies to these routes, not to health/metrics probes.
	router.Group(func(r chi.Router) {
		// Apply middleware (order matters):
		// 1. RequestID: generates or reads X-Request-ID for each request
		// 2. Tracing: sets span names after routing
		// 3. Request logger: enriches context logger with requestID/traceID, logs each request
		// 4. Metrics: records Prometheus request duration and counter
		r.Use(middleware.RequestID)
		r.Use(tracing.NewTracingMiddleware().RouteSpanNameMiddleware())
		r.Use(logging.RequestLoggerMiddleware(logger))
		r.Use(monitoring.NewMiddleware(monitor).Metrics())

		handlers.RegisterRoutes(r)
	})

	otelHandler := tracing.NewTracingMiddleware().OpenTelemetry(router)
	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.BridgeBasePort),
		Handler:           otelHandler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	return &App{
		HTTPServer: httpServer,
		Tracer:     tracer,
		Pool:       pool,
		logger:     logger,
	}, nil
}
