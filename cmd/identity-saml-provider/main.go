package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"

	"github.com/canonical/identity-saml-provider/internal/provider"
	"github.com/kelseyhightower/envconfig"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()

	// Parse command-line flags
	verbose := flag.Bool("verbose", false, "Enable verbose (development) logging")
	flag.Parse()

	// Initialize zap logger with appropriate level
	var zapLogger *zap.Logger
	var err error
	if *verbose {
		zapLogger, err = zap.NewDevelopment()
	} else {
		zapLogger, err = zap.NewProduction()
	}
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	defer zapLogger.Sync()
	logger := zapLogger.Sugar()

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
	server, err := provider.NewServer(config, logger, db)
	if err != nil {
		logger.Fatalw("Failed to create server", "error", err)
	}

	// Initialize database schema
	dbWrapper := provider.NewDatabase(db, logger)
	if err = dbWrapper.InitSchema(); err != nil {
		logger.Fatalw("Failed to initialize database schema", "error", err)
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
