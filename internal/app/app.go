package app

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/canonical/identity-saml-provider/internal/handler"
	"github.com/canonical/identity-saml-provider/internal/infrastructure/hydra"
	"github.com/canonical/identity-saml-provider/internal/infrastructure/samlkit"
	"github.com/canonical/identity-saml-provider/internal/monitoring"
	"github.com/canonical/identity-saml-provider/internal/monitoring/prometheus"
	"github.com/canonical/identity-saml-provider/internal/repository/memory"
	"github.com/canonical/identity-saml-provider/internal/repository/postgres"
	"github.com/canonical/identity-saml-provider/internal/service"
	"github.com/canonical/identity-saml-provider/internal/tracing"
	"go.uber.org/zap"
)

// App holds the fully wired application.
type App struct {
	HTTPServer *http.Server
	Tracer     tracing.TracingInterface
	Pool       *pgxpool.Pool
}

// Build constructs the application from the given configuration.
func Build(ctx context.Context, cfg Config, zapLogger *zap.Logger) (*App, error) {
	logger := zapLogger.Sugar()

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
	hydraClient, err := hydra.NewClient(cfg.HydraConfig())
	if err != nil {
		pool.Close()
		return nil, err
	}
	discovery, err := hydra.DiscoverOIDC(ctx, hydraClient, cfg.HydraConfig(), cfg.OIDCConfig())
	if err != nil {
		pool.Close()
		return nil, err
	}

	samlIDP, err := samlkit.NewIdentityProvider(cfg.SAMLConfig(), zapLogger)
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
	oidcSvc := service.NewOIDCService(discovery.OAuth2Config, service.NewIDTokenVerifierAdapter(discovery.Verifier), logger)
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

	// --- HTTP Server ---
	router := chi.NewRouter()

	// Apply middleware
	router.Use(tracing.NewMiddleware(monitor, logger).RouteSpanNameMiddleware())
	router.Use(monitoring.NewMiddleware(monitor, logger).ResponseTime())

	handlers.RegisterRoutes(router)

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
