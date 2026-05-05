package cmd

import (
	"context"
	"fmt"
	"net/http"

	"github.com/canonical/identity-saml-provider/internal/app"
	"github.com/canonical/identity-saml-provider/internal/version"
	"github.com/kelseyhightower/envconfig"
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
	defer zapLogger.Sync() //nolint:errcheck
	logger := zapLogger.Sugar()

	// Print startup version information
	logger.Infow("Starting identity-saml-provider", "version", version.Version)

	// Load configuration from environment variables
	var cfg app.Config
	if err := envconfig.Process("", &cfg); err != nil {
		logger.Fatalw("Failed to process configuration", "error", err)
	}

	// Build the application (pool → repos → services → handlers → HTTP server)
	application, err := app.Build(ctx, cfg, zapLogger)
	if err != nil {
		logger.Fatalw("Failed to build application", "error", err)
	}
	defer application.Pool.Close()
	defer func() {
		if err := application.Tracer.Shutdown(); err != nil {
			logger.Warnw("Failed to shutdown tracer", "error", err)
		}
	}()

	logger.Infow("Starting server", "addr", application.HTTPServer.Addr)
	if err := application.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalw("Server error", "error", err)
	}
}
