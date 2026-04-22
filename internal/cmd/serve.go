package cmd

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/identity-saml-provider/internal/monitoring/prometheus"
	"github.com/canonical/identity-saml-provider/internal/provider"
	"github.com/canonical/identity-saml-provider/internal/tracing"
	"github.com/canonical/identity-saml-provider/internal/version"
	"github.com/kelseyhightower/envconfig"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var verbose bool

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the SAML-OIDC bridge HTTP server",
	Long:  "Launch the SAML-OIDC bridge HTTP server",
	Run: func(cmd *cobra.Command, args []string) {
		runServe()
	},
}

func init() {
	serveCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose (development) logging")
	rootCmd.AddCommand(serveCmd)
}

func runServe() {
	ctx := context.Background()

	// Initialize zap logger with appropriate level
	var zapLogger *zap.Logger
	var err error
	if verbose {
		zapLogger, err = zap.NewDevelopment()
	} else {
		zapLogger, err = zap.NewProduction()
	}
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	defer zapLogger.Sync()
	logger := zapLogger.Sugar()

	// Print startup version information
	logger.Infow("Starting identity-saml-provider", "version", version.Version)

	// Load configuration from environment variables
	var config provider.Config
	if err := envconfig.Process("", &config); err != nil {
		logger.Fatalw("Failed to process configuration", "error", err)
	}

	// -------------------------------------------------------------------------
	// 1. Initialize Database Connection
	// -------------------------------------------------------------------------
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName)
	logger.Infow("Connecting to PostgreSQL", "host", config.DBHost, "port", config.DBPort)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		logger.Fatalw("Failed to open database connection", "error", err)
	}
	defer db.Close()

	// Verify the connection
	if err = db.PingContext(ctx); err != nil {
		logger.Fatalw("Failed to connect to database", "error", err)
	}
	logger.Info("Database connection established")

	// -------------------------------------------------------------------------
	// 2. Create and Initialize Server
	// -------------------------------------------------------------------------
	monitor := prometheus.NewMonitor("identity-saml-provider", logger)
	tracer := tracing.NewTracer(tracing.NewConfig(
		config.TracingEnabled,
		config.OtelGRPCEndpoint,
		config.OtelHTTPEndpoint,
		config.OtelSampler,
		config.OtelSamplerRatio,
		logger,
	))
	defer func() {
		if err := tracer.Shutdown(); err != nil {
			logger.Warnw("Failed to shutdown tracer", "error", err)
		}
	}()

	server, err := provider.NewServer(config, logger, db, monitor, tracer)
	if err != nil {
		logger.Fatalw("Failed to create server", "error", err)
	}

	// Initialize OIDC and SAML providers
	if err = server.Initialize(ctx, zapLogger); err != nil {
		logger.Fatalw("Failed to initialize server", "error", err)
	}

	// -------------------------------------------------------------------------
	// 3. Setup Routes and Start Server
	// -------------------------------------------------------------------------
	server.SetupRoutes()

	logger.Fatalw("Server error", "error", server.Start())
}
