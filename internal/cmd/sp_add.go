// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/canonical/identity-saml-provider/internal/app"
	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/repository/postgres"
	"github.com/canonical/identity-saml-provider/internal/service"
	"github.com/canonical/identity-saml-provider/internal/tracing"
	"github.com/kelseyhightower/envconfig"
	"github.com/spf13/cobra"
)

var (
	spEntityID             string
	spACSURL               string
	spACSBinding           string
	spAttributeMappingFile string
)

var spAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Register a SAML service provider",
	Long: `Register a new SAML service provider with the Identity SAML Provider.

Requires database connection via SAML_PROVIDER_DB_* environment variables:
  SAML_PROVIDER_DB_HOST (default: localhost)
  SAML_PROVIDER_DB_PORT (default: 5432)
  SAML_PROVIDER_DB_NAME (default: saml_provider)
  SAML_PROVIDER_DB_USER (default: saml_provider)
  SAML_PROVIDER_DB_PASSWORD (default: saml_provider)`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunHandler(cmd, formatSPRegistered, func(ctx context.Context) (*SPResult, error) {
			// Load config from SAML_PROVIDER_DB_* env vars.
			var cfg app.Config
			if err := envconfig.Process("", &cfg); err != nil {
				return nil, fmt.Errorf("load config from environment: %w", err)
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

			// Build domain object from CLI flags.
			sp, err := buildServiceProvider()
			if err != nil {
				return nil, err
			}

			// Register via service layer.
			repo := postgres.NewServiceProviderRepo(pool, tracing.NewNoopTracer())
			logger := logging.NewNopLogger()
			svc := service.NewServiceProviderService(repo, logger, tracing.NewNoopTracer())

			if err := svc.Register(ctx, sp); err != nil {
				return nil, err
			}

			return &SPResult{
				EntityID:   sp.EntityID,
				ACSURL:     sp.ACSURL,
				ACSBinding: sp.ACSBinding,
			}, nil
		})
	},
}

func init() {
	spAddCmd.Flags().StringVarP(
		&spEntityID, "entity-id", "e", "",
		"Entity ID of the service provider (required)",
	)
	spAddCmd.Flags().StringVarP(
		&spACSURL, "acs-url", "a", "",
		"Assertion Consumer Service URL (required)",
	)
	spAddCmd.Flags().StringVarP(
		&spACSBinding, "acs-binding", "b",
		"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
		"ACS binding type",
	)
	spAddCmd.Flags().StringVar(
		&spAttributeMappingFile,
		"attribute-mapping-file", "",
		"Path to attribute mapping JSON file",
	)

	_ = spAddCmd.MarkFlagRequired("entity-id")
	_ = spAddCmd.MarkFlagRequired("acs-url")

	spCmd.AddCommand(spAddCmd)
}

// buildServiceProvider constructs a domain.ServiceProvider from CLI flags.
// All attribute mapping configuration (including the NameID format) is
// supplied through --attribute-mapping-file. When the flag is omitted,
// the resulting service provider has no attribute mapping attached.
func buildServiceProvider() (*domain.ServiceProvider, error) {
	sp := &domain.ServiceProvider{
		EntityID:   spEntityID,
		ACSURL:     spACSURL,
		ACSBinding: spACSBinding,
	}

	if spAttributeMappingFile != "" {
		data, err := os.ReadFile(spAttributeMappingFile)
		if err != nil {
			return nil, fmt.Errorf("read attribute mapping file %q: %w", spAttributeMappingFile, err)
		}
		var mapping domain.AttributeMapping
		if err := json.Unmarshal(data, &mapping); err != nil {
			return nil, fmt.Errorf("parse attribute mapping JSON from %q: %w", spAttributeMappingFile, err)
		}
		sp.AttributeMapping = &mapping
	}

	// Validate the domain object.
	if err := sp.Validate(); err != nil {
		return nil, err
	}

	return sp, nil
}
