// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package app

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/canonical/identity-saml-provider/internal/infrastructure/hydra"
	"github.com/canonical/identity-saml-provider/internal/infrastructure/samlkit"
	"github.com/canonical/identity-saml-provider/internal/repository/postgres"
)

// Config defines the configuration for the SAML provider application.
type Config struct {
	// Bridge Configuration
	BridgeBasePort int    `envconfig:"SAML_PROVIDER_BRIDGE_BASE_PORT" default:"8082"`
	BridgeBaseURL  string `envconfig:"SAML_PROVIDER_BRIDGE_BASE_URL"  default:"http://localhost:8082"`

	// DevMode relaxes security settings for local development.
	// When true: session cookies omit the Secure attribute, OIDC
	// issuer URL validation is relaxed, and the logger uses
	// human-readable output. Must not be enabled in production.
	DevMode bool `envconfig:"SAML_PROVIDER_DEV_MODE" default:"false"`

	// ServiceName is set programmatically (not from env).
	ServiceName string `envconfig:"-"`

	// Observability Configuration
	TracingEnabled   bool    `envconfig:"SAML_PROVIDER_TRACING_ENABLED" default:"false"`
	OtelHTTPEndpoint string  `envconfig:"SAML_PROVIDER_OTEL_HTTP_ENDPOINT" default:""`
	OtelGRPCEndpoint string  `envconfig:"SAML_PROVIDER_OTEL_GRPC_ENDPOINT" default:""`
	OtelSampler      string  `envconfig:"SAML_PROVIDER_OTEL_SAMPLER" default:"parentbased_traceidratio"`
	OtelSamplerRatio float64 `envconfig:"SAML_PROVIDER_OTEL_SAMPLER_RATIO" default:"0.1"`

	// Ory Hydra Configuration
	HydraPublicURL  string `envconfig:"SAML_PROVIDER_HYDRA_PUBLIC_URL" default:"http://localhost:4444"`
	HydraCACertPath string `envconfig:"SAML_PROVIDER_HYDRA_CA_CERT_PATH" default:""`
	ClientID        string `envconfig:"SAML_PROVIDER_OIDC_CLIENT_ID" default:"service-bridge-client"`
	ClientSecret    string `envconfig:"SAML_PROVIDER_OIDC_CLIENT_SECRET" default:"secret"`

	// Database Configuration
	DBHost       string `envconfig:"SAML_PROVIDER_DB_HOST" default:"localhost"`
	DBPort       int    `envconfig:"SAML_PROVIDER_DB_PORT" default:"5432"`
	DBName       string `envconfig:"SAML_PROVIDER_DB_NAME" default:"saml_provider"`
	DBUser       string `envconfig:"SAML_PROVIDER_DB_USER" default:"saml_provider"`
	DBPassword   string `envconfig:"SAML_PROVIDER_DB_PASSWORD" default:"saml_provider"`
	DBSSLMode    string `envconfig:"SAML_PROVIDER_DB_SSLMODE" default:""`
	DBCACertPath string `envconfig:"SAML_PROVIDER_DB_CA_CERT_PATH" default:""`

	// Database Pool Configuration
	DBMaxConns        int32         `envconfig:"SAML_PROVIDER_DB_MAX_CONNS" default:"10"`
	DBMinConns        int32         `envconfig:"SAML_PROVIDER_DB_MIN_CONNS" default:"2"`
	DBMaxConnLifetime time.Duration `envconfig:"SAML_PROVIDER_DB_MAX_CONN_LIFETIME" default:"30m"`
	DBMaxConnIdleTime time.Duration `envconfig:"SAML_PROVIDER_DB_MAX_CONN_IDLE_TIME" default:"5m"`

	// Certificate Configuration
	SAMLCertPath string `envconfig:"SAML_PROVIDER_CERT_PATH" default:".local/certs/bridge.crt"`
	SAMLKeyPath  string `envconfig:"SAML_PROVIDER_KEY_PATH" default:".local/certs/bridge.key"`

	// Logging Configuration
	// LogLevel sets the minimum log level.
	LogLevel string `envconfig:"SAML_PROVIDER_LOG_LEVEL" default:"info"`

	// Server Configuration
	// ShutdownTimeout is the maximum duration to wait for in-flight
	// requests to complete during graceful shutdown.
	ShutdownTimeout time.Duration `envconfig:"SAML_PROVIDER_SHUTDOWN_TIMEOUT" default:"30s"`
	// ReadHeaderTimeout is the amount of time allowed to read request headers.
	ReadHeaderTimeout time.Duration `envconfig:"SAML_PROVIDER_READ_HEADER_TIMEOUT" default:"10s"`
	// IdleTimeout is the maximum amount of time to wait for the next
	// request when keep-alives are enabled.
	IdleTimeout time.Duration `envconfig:"SAML_PROVIDER_IDLE_TIMEOUT" default:"120s"`

	// PendingRequestTTL is the lifetime of a pending SAML authentication request before it is considered expired.
	PendingRequestTTL time.Duration `envconfig:"SAML_PROVIDER_PENDING_REQUEST_TTL" default:"15m"`
}

// validSSLModes is the set of PostgreSQL SSL modes accepted by Validate.
var validSSLModes = map[string]bool{
	"disable":     true,
	"allow":       true,
	"prefer":      true,
	"require":     true,
	"verify-ca":   true,
	"verify-full": true,
}

// Validate checks that the configuration is semantically valid.
func (c *Config) Validate() error {
	if c.BridgeBaseURL == "" {
		return fmt.Errorf("SAML_PROVIDER_BRIDGE_BASE_URL must not be empty")
	}
	if _, err := url.ParseRequestURI(c.BridgeBaseURL); err != nil {
		return fmt.Errorf("invalid SAML_PROVIDER_BRIDGE_BASE_URL %q: %w", c.BridgeBaseURL, err)
	}

	if c.BridgeBasePort < 1 || c.BridgeBasePort > 65535 {
		return fmt.Errorf("SAML_PROVIDER_BRIDGE_BASE_PORT must be 1–65535, got %d", c.BridgeBasePort)
	}

	if c.HydraPublicURL == "" {
		return fmt.Errorf("SAML_PROVIDER_HYDRA_PUBLIC_URL must not be empty")
	}
	hydraURL, err := url.ParseRequestURI(c.HydraPublicURL)
	if err != nil {
		return fmt.Errorf("invalid SAML_PROVIDER_HYDRA_PUBLIC_URL %q: %w", c.HydraPublicURL, err)
	}
	if hydraURL.Scheme != "http" && hydraURL.Scheme != "https" {
		return fmt.Errorf("SAML_PROVIDER_HYDRA_PUBLIC_URL scheme must be http or https, got %q", hydraURL.Scheme)
	}

	if c.ClientID == "" {
		return fmt.Errorf("SAML_PROVIDER_OIDC_CLIENT_ID must not be empty")
	}
	if c.ClientSecret == "" {
		return fmt.Errorf("SAML_PROVIDER_OIDC_CLIENT_SECRET must not be empty")
	}

	if c.DBPort < 1 || c.DBPort > 65535 {
		return fmt.Errorf("SAML_PROVIDER_DB_PORT must be 1–65535, got %d", c.DBPort)
	}

	if c.DBSSLMode != "" && !validSSLModes[c.DBSSLMode] {
		return fmt.Errorf("invalid SAML_PROVIDER_DB_SSLMODE %q", c.DBSSLMode)
	}

	if c.DBCACertPath != "" && c.DBSSLMode == "disable" {
		return fmt.Errorf("SAML_PROVIDER_DB_CA_CERT_PATH cannot be used with SAML_PROVIDER_DB_SSLMODE %q", c.DBSSLMode)
	}

	if c.DBMaxConns < c.DBMinConns {
		return fmt.Errorf("SAML_PROVIDER_DB_MAX_CONNS (%d) must be >= SAML_PROVIDER_DB_MIN_CONNS (%d)",
			c.DBMaxConns, c.DBMinConns)
	}

	if c.SAMLCertPath == "" {
		return fmt.Errorf("SAML_PROVIDER_CERT_PATH must not be empty")
	}
	if c.SAMLKeyPath == "" {
		return fmt.Errorf("SAML_PROVIDER_KEY_PATH must not be empty")
	}

	if c.PendingRequestTTL <= 0 {
		return fmt.Errorf("SAML_PROVIDER_PENDING_REQUEST_TTL must be positive, got %s", c.PendingRequestTTL)
	}

	if c.DevMode {
		fmt.Println("WARNING: SAML_PROVIDER_DEV_MODE is enabled." +
			" Secure cookie attribute is disabled and" +
			" OIDC issuer validation is relaxed." +
			" Do not use in production.")
	}

	return nil
}

// PoolConfig returns pgxpool configuration derived from the database settings.
func (c *Config) PoolConfig() postgres.PoolConfig {
	return postgres.PoolConfig{
		DSN:             c.DatabaseDSN(),
		MaxConns:        c.DBMaxConns,
		MinConns:        c.DBMinConns,
		MaxConnLifetime: c.DBMaxConnLifetime,
		MaxConnIdleTime: c.DBMaxConnIdleTime,
	}
}

// DatabaseDSN builds a safely-encoded PostgreSQL connection string.
func (c *Config) DatabaseDSN() string {
	sslMode := c.DBSSLMode
	if sslMode == "" {
		if c.DBCACertPath != "" {
			sslMode = "verify-full"
		} else {
			sslMode = "disable"
		}
	}

	q := url.Values{
		"sslmode": {sslMode},
	}
	if c.DBCACertPath != "" {
		q.Set("sslrootcert", c.DBCACertPath)
	}

	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.DBUser, c.DBPassword),
		Host:     net.JoinHostPort(c.DBHost, strconv.Itoa(c.DBPort)),
		Path:     c.DBName,
		RawQuery: q.Encode(),
	}
	return u.String()
}

// HydraConfig returns the subset of config needed by the Hydra OIDC client.
func (c *Config) HydraConfig() hydra.Config {
	return hydra.Config{
		IssuerURL:  c.HydraPublicURL,
		DevMode:    c.DevMode,
		CACertPath: c.HydraCACertPath,
	}
}

// OIDCConfig returns the OIDC client credentials and redirect settings.
func (c *Config) OIDCConfig() hydra.OIDCConfig {
	return hydra.OIDCConfig{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  c.BridgeBaseURL + "/saml/callback",
		Scopes:       []string{"openid", "email", "profile"},
	}
}

// SAMLConfig returns the subset needed for SAML IdP setup.
func (c *Config) SAMLConfig() samlkit.Config {
	return samlkit.Config{
		BridgeBaseURL: c.BridgeBaseURL,
		CertPath:      c.SAMLCertPath,
		KeyPath:       c.SAMLKeyPath,
	}
}
