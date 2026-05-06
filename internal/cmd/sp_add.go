package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/canonical/identity-saml-provider/internal/app"
	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/repository/postgres"
	"github.com/canonical/identity-saml-provider/internal/service"
	"github.com/kelseyhightower/envconfig"
	"github.com/spf13/cobra"
)

var (
	spEntityID             string
	spACSURL               string
	spACSBinding           string
	spAttributeMappingFile string
	spNameIDFormat         string
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
		ctx := cmd.Context()

		formatter, err := newSPFormatter(spFormat)
		if err != nil {
			return err
		}

		// Load config from SAML_PROVIDER_DB_* env vars.
		var cfg app.Config
		if err := envconfig.Process("", &cfg); err != nil {
			return fmt.Errorf("load config from environment: %w", err)
		}

		// Open DB connection.
		pool, err := openSPDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer pool.Close()

		// Build domain object from CLI flags.
		sp, err := buildServiceProvider()
		if err != nil {
			return err
		}

		// Register via service layer.
		repo := postgres.NewServiceProviderRepo(pool)
		logger := newCLILogger()
		svc := service.NewServiceProviderService(repo, logger)

		if err := svc.Register(ctx, sp); err != nil {
			cmd.SilenceErrors = true
			return formatter.SPError(cmd.OutOrStdout(), sp, err)
		}

		return formatter.SPRegistered(cmd.OutOrStdout(), sp)
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
	spAddCmd.Flags().StringVar(
		&spNameIDFormat, "nameid-format", "",
		"NameID format (persistent, transient, emailAddress)",
	)

	_ = spAddCmd.MarkFlagRequired("entity-id")
	_ = spAddCmd.MarkFlagRequired("acs-url")

	spCmd.AddCommand(spAddCmd)
}

// buildServiceProvider constructs a domain.ServiceProvider from CLI flags.
// It reads the attribute mapping from file if provided, or creates a
// minimal mapping from --nameid-format.
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
	} else if spNameIDFormat != "" {
		// If only nameid-format is provided without a full mapping file,
		// create a minimal attribute mapping.
		sp.AttributeMapping = &domain.AttributeMapping{
			NameIDFormat: spNameIDFormat,
		}
	}

	// Validate the domain object.
	if err := sp.Validate(); err != nil {
		return nil, err
	}

	return sp, nil
}
