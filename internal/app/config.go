package app

import (
	"fmt"
	"time"

	"github.com/canonical/identity-saml-provider/internal/infrastructure/hydra"
	"github.com/canonical/identity-saml-provider/internal/infrastructure/samlkit"
	"github.com/canonical/identity-saml-provider/internal/repository/postgres"
	"github.com/canonical/identity-saml-provider/internal/tracing"
)

// Config defines the configuration for the SAML provider application.
// All environment variable names are kept identical to the original
// provider.Config for backward compatibility.
type Config struct {
	// Bridge Configuration
	BridgeBasePort string `envconfig:"SAML_PROVIDER_BRIDGE_BASE_PORT" default:"8082"`
	BridgeBaseURL  string `envconfig:"SAML_PROVIDER_BRIDGE_BASE_URL"  default:"http://localhost:8082"`

	// ServiceName is set programmatically (not from env).
	ServiceName string `envconfig:"-"`

	// Observability Configuration
	TracingEnabled   bool    `envconfig:"SAML_PROVIDER_TRACING_ENABLED" default:"false"`
	OtelHTTPEndpoint string  `envconfig:"SAML_PROVIDER_OTEL_HTTP_ENDPOINT" default:""`
	OtelGRPCEndpoint string  `envconfig:"SAML_PROVIDER_OTEL_GRPC_ENDPOINT" default:""`
	OtelSampler      string  `envconfig:"SAML_PROVIDER_OTEL_SAMPLER" default:"parentbased_traceidratio"`
	OtelSamplerRatio float64 `envconfig:"SAML_PROVIDER_OTEL_SAMPLER_RATIO" default:"0.1"`

	// Ory Hydra Configuration
	HydraPublicURL             string `envconfig:"SAML_PROVIDER_HYDRA_PUBLIC_URL" default:"http://localhost:4444"`
	HydraInsecureSkipTLSVerify bool   `envconfig:"SAML_PROVIDER_HYDRA_INSECURE_SKIP_TLS_VERIFY" default:"false"`
	HydraCACertPath            string `envconfig:"SAML_PROVIDER_HYDRA_CA_CERT_PATH" default:""`
	ClientID                   string `envconfig:"SAML_PROVIDER_OIDC_CLIENT_ID" default:"service-bridge-client"`
	ClientSecret               string `envconfig:"SAML_PROVIDER_OIDC_CLIENT_SECRET" default:"secret"`
	RedirectURL                string `envconfig:"SAML_PROVIDER_OIDC_REDIRECT_URL" default:"http://localhost:8082/saml/callback"`

	// Service Configuration
	ServiceACS      string `envconfig:"SAML_PROVIDER_SERVICE_ACS" default:"http://localhost:8083/saml/acs"`
	ServiceEntityID string `envconfig:"SAML_PROVIDER_SERVICE_ENTITY_ID" default:"http://localhost:8083/saml/metadata"`

	// Database Configuration
	DBHost     string `envconfig:"SAML_PROVIDER_DB_HOST" default:"localhost"`
	DBPort     string `envconfig:"SAML_PROVIDER_DB_PORT" default:"5432"`
	DBName     string `envconfig:"SAML_PROVIDER_DB_NAME" default:"saml_provider"`
	DBUser     string `envconfig:"SAML_PROVIDER_DB_USER" default:"saml_provider"`
	DBPassword string `envconfig:"SAML_PROVIDER_DB_PASSWORD" default:"saml_provider"`

	// Certificate Configuration
	SAMLCertPath string `envconfig:"SAML_PROVIDER_CERT_PATH" default:".local/certs/bridge.crt"`
	SAMLKeyPath  string `envconfig:"SAML_PROVIDER_KEY_PATH" default:".local/certs/bridge.key"`
}

// PoolConfig returns pgxpool configuration derived from the database settings.
func (c Config) PoolConfig() postgres.PoolConfig {
	return postgres.PoolConfig{
		DSN:             c.DatabaseDSN(),
		MaxConns:        10,
		MinConns:        2,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
	}
}

// DatabaseDSN builds the PostgreSQL connection string.
func (c Config) DatabaseDSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}

// HydraConfig returns the subset of config needed by the Hydra HTTP client.
func (c Config) HydraConfig() hydra.Config {
	return hydra.Config{
		CACertPath:            c.HydraCACertPath,
		InsecureSkipTLSVerify: c.HydraInsecureSkipTLSVerify,
		IssuerURL:             c.HydraPublicURL,
	}
}

// OIDCConfig returns the OIDC client credentials/redirect settings.
func (c Config) OIDCConfig() hydra.OIDCConfig {
	return hydra.OIDCConfig{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  c.RedirectURL,
		Scopes:       []string{"openid", "email", "profile"},
	}
}

// SAMLConfig returns the subset needed for SAML IdP setup.
func (c Config) SAMLConfig() samlkit.Config {
	return samlkit.Config{
		BridgeBaseURL: c.BridgeBaseURL,
		CertPath:      c.SAMLCertPath,
		KeyPath:       c.SAMLKeyPath,
	}
}

// TracingConfig returns tracing configuration.
func (c Config) TracingConfig() *tracing.Config {
	return tracing.NewConfig(
		c.TracingEnabled,
		c.OtelGRPCEndpoint,
		c.OtelHTTPEndpoint,
		c.OtelSampler,
		c.OtelSamplerRatio,
		nil, // logger is set in Build()
	)
}
