package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"

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

	logger.Infow("Starting identity-saml-provider", "version", version.Version, "logLevel", cfg.LogLevel)

	application, err := app.Build(ctx, cfg, logger)
	if err != nil {
		logger.Fatalw("Failed to build application", "error", err)
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Infow("Starting server", "addr", application.HTTPServer.Addr)
		if err := application.HTTPServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			logger.Errorw("Server error", "error", err)
		}
	case <-ctx.Done():
		stop()
		logger.Infow("Shutting down server", "timeout", cfg.ShutdownTimeout.String())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	application.Shutdown(shutdownCtx)

	logger.Infow("Server exited gracefully")
}
