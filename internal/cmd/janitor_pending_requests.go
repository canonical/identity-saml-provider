// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/canonical/identity-saml-provider/internal/app"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/repository/postgres"
	"github.com/canonical/identity-saml-provider/internal/service"
	"github.com/canonical/identity-saml-provider/internal/tracing"
	"github.com/kelseyhightower/envconfig"
	"github.com/spf13/cobra"
)

var (
	janitorBatchSize int
)

func validateBatchSize(batchSize int) error {
	if batchSize <= 0 {
		return fmt.Errorf("batch-size must be greater than 0")
	}
	return nil
}

var janitorPendingRequestsCmd = &cobra.Command{
	Use:   "pending-requests",
	Short: "Clean up expired SAML pending requests from the database",
	Long: `Clean up transient SAML pending authentication requests that have exceeded their configured TTL in the database.

Requires database connection via SAML_PROVIDER_DB_* environment variables:
  SAML_PROVIDER_DB_HOST (default: localhost)
  SAML_PROVIDER_DB_PORT (default: 5432)
  SAML_PROVIDER_DB_NAME (default: saml_provider)
  SAML_PROVIDER_DB_USER (default: saml_provider)
  SAML_PROVIDER_DB_PASSWORD (default: saml_provider)`,
	SilenceErrors: true,
	SilenceUsage:  true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return validateBatchSize(janitorBatchSize)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunHandler(cmd, formatJanitorPendingRequests, func(ctx context.Context) (JanitorResult, error) {
			var cfg app.Config
			if err := envconfig.Process("", &cfg); err != nil {
				return JanitorResult{}, fmt.Errorf("failed to process configuration: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return JanitorResult{}, fmt.Errorf("invalid configuration: %w", err)
			}

			pool, err := openDB(ctx, cfg)
			if err != nil {
				return JanitorResult{}, err
			}
			defer pool.Close()

			var totalDeleted int64

			logger := logging.NewNopLogger()
			tracer := tracing.NewNoopTracer()
			pendingRepo := postgres.NewPendingRequestRepo(pool, tracer)
			pendingSvc := service.NewPendingRequestService(pendingRepo, logger, tracer)

			timeout := 1 * time.Minute
			loopCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// Throttling duration to prevent hammering the database
			const throttleDelay = 50 * time.Millisecond

			for {
				deleted, err := pendingSvc.CleanupExpired(loopCtx, janitorBatchSize)
				if err != nil {
					// Gracefully handle context cancellation/timeouts to report partial progress
					if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
						break
					}
					return JanitorResult{}, fmt.Errorf("failed to execute cleanup batch: %w", err)
				}

				totalDeleted += deleted

				// If we deleted fewer rows than the batch size, we have fully caught up
				// and can break early, saving a wasted database query roundtrip.
				if deleted < int64(janitorBatchSize) {
					break
				}

				if loopCtx.Err() != nil {
					break
				}

				time.Sleep(throttleDelay)
			}

			return JanitorResult{DeletedCount: totalDeleted}, nil
		})
	},
}

func init() {
	janitorPendingRequestsCmd.Flags().IntVar(&janitorBatchSize, "batch-size", 1000, "Maximum number of pending requests to delete per batch")

	janitorCmd.AddCommand(janitorPendingRequestsCmd)
}
