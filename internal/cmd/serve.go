package cmd

import (
	"context"
	"fmt"
	"net/http"

	"github.com/canonical/identity-saml-provider/internal/app"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/version"
	"github.com/kelseyhightower/envconfig"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the SAML-OIDC bridge HTTP server",
	Long:  "Launch the SAML-OIDC bridge HTTP server",
	Run: func(cmd *cobra.Command, args []string) {
		runServe()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe() {
	ctx := context.Background()

	var cfg app.Config
	if err := envconfig.Process("", &cfg); err != nil {
		panic(fmt.Sprintf("Failed to process configuration: %v", err))
	}

	logger, err := logging.BuildLogger(cfg.LogLevel)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	defer logger.Sync() //nolint:errcheck

	logger.Infow("Starting identity-saml-provider", "version", version.Version, "logLevel", cfg.LogLevel)

	application, err := app.Build(ctx, cfg, logger)
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
