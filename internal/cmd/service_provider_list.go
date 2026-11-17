// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"context"
	"fmt"

	"github.com/canonical/identity-saml-provider/internal/app"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/repository/postgres"
	"github.com/canonical/identity-saml-provider/internal/service"
	"github.com/canonical/identity-saml-provider/internal/tracing"
	"github.com/kelseyhightower/envconfig"
	"github.com/spf13/cobra"
)

var serviceProviderListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered SAML service providers",
	Long: `List every SAML service provider registered with the Identity SAML Provider.

Requires database connection via SAML_PROVIDER_DB_* environment variables:
  SAML_PROVIDER_DB_HOST (default: localhost)
  SAML_PROVIDER_DB_PORT (default: 5432)
  SAML_PROVIDER_DB_NAME (default: saml_provider)
  SAML_PROVIDER_DB_USER (default: saml_provider)
  SAML_PROVIDER_DB_PASSWORD (default: saml_provider)
  SAML_PROVIDER_DB_SSLMODE (optional)
  SAML_PROVIDER_DB_CA_CERT_PATH (optional)`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunHandler(cmd, formatSPList, func(ctx context.Context) ([]*SPListItem, error) {
			// Load config from SAML_PROVIDER_DB_* env vars.
			var cfg app.Config
			if err := envconfig.Process("", &cfg); err != nil {
				return nil, fmt.Errorf("cannot load config from environment: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("invalid configuration: %w", err)
			}

			// Open DB connection.
			pool, err := openDB(ctx, cfg)
			if err != nil {
				return nil, err
			}
			defer pool.Close()

			// List via service layer.
			repo := postgres.NewServiceProviderRepo(pool, tracing.NewNoopTracer())
			logger := logging.NewNopLogger()
			svc := service.NewServiceProviderService(repo, logger, tracing.NewNoopTracer())

			sps, err := svc.List(ctx)
			if err != nil {
				return nil, fmt.Errorf("cannot list service providers: %w", err)
			}

			items := make([]*SPListItem, 0, len(sps))
			for _, sp := range sps {
				items = append(items, &SPListItem{
					EntityID:         sp.EntityID,
					ACSURL:           sp.ACSURL,
					ACSBinding:       sp.ACSBinding,
					AttributeMapping: sp.AttributeMapping,
				})
			}

			return items, nil
		})
	},
}

func init() {
	serviceProviderCmd.AddCommand(serviceProviderListCmd)
}
