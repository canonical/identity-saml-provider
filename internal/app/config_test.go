// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/canonical/identity-saml-provider/internal/app"
	"github.com/kelseyhightower/envconfig"
)

// validConfig returns a Config populated with the minimum fields needed
// to pass Validate. Tests that need specific overrides can mutate the
// returned value before calling the method under test.
func validConfig() app.Config {
	return app.Config{
		BridgeBasePort:    8082,
		BridgeBaseURL:     "http://localhost:8082",
		HydraPublicURL:    "http://localhost:4444",
		ClientID:          "service-bridge-client",
		ClientSecret:      "secret",
		DBHost:            "localhost",
		DBPort:            5432,
		DBName:            "saml_provider",
		DBUser:            "saml_provider",
		DBPassword:        "saml_provider",
		DBSSLMode:         "disable",
		DBMaxConns:        10,
		DBMinConns:        2,
		DBMaxConnLifetime: 30 * time.Minute,
		DBMaxConnIdleTime: 5 * time.Minute,
		SAMLCertPath:      ".local/certs/bridge.crt",
		SAMLKeyPath:       ".local/certs/bridge.key",
		PendingRequestTTL: 15 * time.Minute,
	}
}

func TestConfig_DatabaseDSN(t *testing.T) {
	tests := []struct {
		name string
		cfg  app.Config
		want string
	}{
		{
			name: "default values",
			cfg: app.Config{
				DBUser:     "saml_provider",
				DBPassword: "saml_provider",
				DBHost:     "localhost",
				DBPort:     5432,
				DBName:     "saml_provider",
				DBSSLMode:  "disable",
			},
			want: "postgres://saml_provider:saml_provider@localhost:5432/saml_provider?sslmode=disable",
		},
		{
			name: "custom values",
			cfg: app.Config{
				DBUser:     "admin",
				DBPassword: "s3cret",
				DBHost:     "db.example.com",
				DBPort:     5433,
				DBName:     "mydb",
				DBSSLMode:  "disable",
			},
			want: "postgres://admin:s3cret@db.example.com:5433/mydb?sslmode=disable",
		},
		{
			name: "special characters in password are URL-encoded",
			cfg: app.Config{
				DBUser:     "admin",
				DBPassword: "p@ss:word/123",
				DBHost:     "localhost",
				DBPort:     5432,
				DBName:     "saml_provider",
				DBSSLMode:  "disable",
			},
			want: "postgres://admin:p%40ss%3Aword%2F123@localhost:5432/saml_provider?sslmode=disable",
		},
		{
			name: "custom sslmode",
			cfg: app.Config{
				DBUser:     "admin",
				DBPassword: "pass",
				DBHost:     "db.example.com",
				DBPort:     5432,
				DBName:     "mydb",
				DBSSLMode:  "require",
			},
			want: "postgres://admin:pass@db.example.com:5432/mydb?sslmode=require",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cfg.DatabaseDSN()
			if got != tc.want {
				t.Errorf("DatabaseDSN() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestConfig_PoolConfig(t *testing.T) {
	tests := []struct {
		name            string
		cfg             app.Config
		wantDSN         string
		wantMaxConns    int32
		wantMinConns    int32
		wantMaxLifetime time.Duration
		wantMaxIdleTime time.Duration
	}{
		{
			name: "default pool settings",
			cfg: app.Config{
				DBUser:            "user",
				DBPassword:        "pass",
				DBHost:            "host",
				DBPort:            5432,
				DBName:            "db",
				DBSSLMode:         "disable",
				DBMaxConns:        10,
				DBMinConns:        2,
				DBMaxConnLifetime: 30 * time.Minute,
				DBMaxConnIdleTime: 5 * time.Minute,
			},
			wantDSN:         "postgres://user:pass@host:5432/db?sslmode=disable",
			wantMaxConns:    10,
			wantMinConns:    2,
			wantMaxLifetime: 30 * time.Minute,
			wantMaxIdleTime: 5 * time.Minute,
		},
		{
			name: "custom pool settings",
			cfg: app.Config{
				DBUser:            "admin",
				DBPassword:        "secret",
				DBHost:            "db.prod",
				DBPort:            5433,
				DBName:            "prod_db",
				DBSSLMode:         "require",
				DBMaxConns:        20,
				DBMinConns:        5,
				DBMaxConnLifetime: 1 * time.Hour,
				DBMaxConnIdleTime: 10 * time.Minute,
			},
			wantDSN:         "postgres://admin:secret@db.prod:5433/prod_db?sslmode=require",
			wantMaxConns:    20,
			wantMinConns:    5,
			wantMaxLifetime: 1 * time.Hour,
			wantMaxIdleTime: 10 * time.Minute,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pc := tc.cfg.PoolConfig()

			if pc.DSN != tc.wantDSN {
				t.Errorf("PoolConfig().DSN = %q, want %q", pc.DSN, tc.wantDSN)
			}
			if pc.MaxConns != tc.wantMaxConns {
				t.Errorf("PoolConfig().MaxConns = %d, want %d", pc.MaxConns, tc.wantMaxConns)
			}
			if pc.MinConns != tc.wantMinConns {
				t.Errorf("PoolConfig().MinConns = %d, want %d", pc.MinConns, tc.wantMinConns)
			}
			if pc.MaxConnLifetime != tc.wantMaxLifetime {
				t.Errorf("PoolConfig().MaxConnLifetime = %v, want %v", pc.MaxConnLifetime, tc.wantMaxLifetime)
			}
			if pc.MaxConnIdleTime != tc.wantMaxIdleTime {
				t.Errorf("PoolConfig().MaxConnIdleTime = %v, want %v", pc.MaxConnIdleTime, tc.wantMaxIdleTime)
			}
		})
	}
}

func TestConfig_HydraConfig(t *testing.T) {
	tests := []struct {
		name           string
		hydraURL       string
		caCertPath     string
		devMode        bool
		wantIssuer     string
		wantCACertPath string
		wantDevMode    bool
	}{
		{
			name:       "default URL",
			hydraURL:   "http://localhost:4444",
			wantIssuer: "http://localhost:4444",
		},
		{
			name:       "custom URL",
			hydraURL:   "https://hydra.example.com",
			wantIssuer: "https://hydra.example.com",
		},
		{
			name:        "dev mode enabled",
			hydraURL:    "http://localhost:4444",
			devMode:     true,
			wantIssuer:  "http://localhost:4444",
			wantDevMode: true,
		},
		{
			name:           "with CA cert path",
			hydraURL:       "https://hydra.example.com",
			caCertPath:     "/etc/ssl/hydra/ca.pem",
			wantIssuer:     "https://hydra.example.com",
			wantCACertPath: "/etc/ssl/hydra/ca.pem",
		},
		{
			name:       "empty CA cert path",
			hydraURL:   "https://hydra.example.com",
			caCertPath: "",
			wantIssuer: "https://hydra.example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := app.Config{
				HydraPublicURL:  tc.hydraURL,
				HydraCACertPath: tc.caCertPath,
				DevMode:         tc.devMode,
			}
			hc := cfg.HydraConfig()

			if hc.IssuerURL != tc.wantIssuer {
				t.Errorf("HydraConfig().IssuerURL = %q, want %q", hc.IssuerURL, tc.wantIssuer)
			}
			if hc.CACertPath != tc.wantCACertPath {
				t.Errorf("HydraConfig().CACertPath = %q, want %q", hc.CACertPath, tc.wantCACertPath)
			}
			if hc.DevMode != tc.wantDevMode {
				t.Errorf("HydraConfig().DevMode = %v, want %v", hc.DevMode, tc.wantDevMode)
			}
		})
	}
}

func TestConfig_OIDCConfig(t *testing.T) {
	tests := []struct {
		name            string
		clientID        string
		clientSecret    string
		bridgeBaseURL   string
		wantRedirectURL string
		wantScopes      []string
	}{
		{
			name:            "default bridge URL",
			clientID:        "my-client",
			clientSecret:    "my-secret",
			bridgeBaseURL:   "http://localhost:8082",
			wantRedirectURL: "http://localhost:8082/saml/callback",
			wantScopes:      []string{"openid", "email", "profile"},
		},
		{
			name:            "custom bridge URL",
			clientID:        "prod-client",
			clientSecret:    "prod-secret",
			bridgeBaseURL:   "https://saml.example.com",
			wantRedirectURL: "https://saml.example.com/saml/callback",
			wantScopes:      []string{"openid", "email", "profile"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := app.Config{
				ClientID:      tc.clientID,
				ClientSecret:  tc.clientSecret,
				BridgeBaseURL: tc.bridgeBaseURL,
			}
			oc := cfg.OIDCConfig()

			if oc.ClientID != tc.clientID {
				t.Errorf("OIDCConfig().ClientID = %q, want %q", oc.ClientID, tc.clientID)
			}
			if oc.ClientSecret != tc.clientSecret {
				t.Errorf("OIDCConfig().ClientSecret = %q, want %q", oc.ClientSecret, tc.clientSecret)
			}
			if oc.RedirectURL != tc.wantRedirectURL {
				t.Errorf("OIDCConfig().RedirectURL = %q, want %q", oc.RedirectURL, tc.wantRedirectURL)
			}
			if len(oc.Scopes) != len(tc.wantScopes) {
				t.Fatalf("OIDCConfig().Scopes length = %d, want %d", len(oc.Scopes), len(tc.wantScopes))
			}
			for i, s := range oc.Scopes {
				if s != tc.wantScopes[i] {
					t.Errorf("OIDCConfig().Scopes[%d] = %q, want %q", i, s, tc.wantScopes[i])
				}
			}
		})
	}
}

func TestConfig_SAMLConfig(t *testing.T) {
	tests := []struct {
		name      string
		bridgeURL string
		certPath  string
		keyPath   string
	}{
		{
			name:      "default paths",
			bridgeURL: "http://localhost:8082",
			certPath:  ".local/certs/bridge.crt",
			keyPath:   ".local/certs/bridge.key",
		},
		{
			name:      "custom paths",
			bridgeURL: "https://saml.example.com",
			certPath:  "/etc/ssl/certs/saml.pem",
			keyPath:   "/etc/ssl/private/saml.key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := app.Config{
				BridgeBaseURL: tc.bridgeURL,
				SAMLCertPath:  tc.certPath,
				SAMLKeyPath:   tc.keyPath,
			}
			sc := cfg.SAMLConfig()

			if sc.BridgeBaseURL != tc.bridgeURL {
				t.Errorf("SAMLConfig().BridgeBaseURL = %q, want %q", sc.BridgeBaseURL, tc.bridgeURL)
			}
			if sc.CertPath != tc.certPath {
				t.Errorf("SAMLConfig().CertPath = %q, want %q", sc.CertPath, tc.certPath)
			}
			if sc.KeyPath != tc.keyPath {
				t.Errorf("SAMLConfig().KeyPath = %q, want %q", sc.KeyPath, tc.keyPath)
			}
		})
	}
}

func TestConfig_EnvconfigProcess(t *testing.T) {
	// Verify that envconfig.Process works with the Config struct
	// (all env vars use the same names as the original provider.Config).
	var cfg app.Config
	// Process with no env vars set — should use defaults.
	if err := envconfig.Process("", &cfg); err != nil {
		t.Fatalf("envconfig.Process() error: %v", err)
	}

	if cfg.BridgeBasePort != 8082 {
		t.Errorf("BridgeBasePort = %d, want %d", cfg.BridgeBasePort, 8082)
	}
	if cfg.BridgeBaseURL != "http://localhost:8082" {
		t.Errorf("BridgeBaseURL = %q, want %q", cfg.BridgeBaseURL, "http://localhost:8082")
	}
	if cfg.DBHost != "localhost" {
		t.Errorf("DBHost = %q, want %q", cfg.DBHost, "localhost")
	}
	if cfg.DBPort != 5432 {
		t.Errorf("DBPort = %d, want %d", cfg.DBPort, 5432)
	}
	if cfg.DBSSLMode != "disable" {
		t.Errorf("DBSSLMode = %q, want %q", cfg.DBSSLMode, "disable")
	}
	if cfg.DBMaxConns != 10 {
		t.Errorf("DBMaxConns = %d, want %d", cfg.DBMaxConns, 10)
	}
	if cfg.DBMinConns != 2 {
		t.Errorf("DBMinConns = %d, want %d", cfg.DBMinConns, 2)
	}
	if cfg.ClientID != "service-bridge-client" {
		t.Errorf("ClientID = %q, want %q", cfg.ClientID, "service-bridge-client")
	}
	if cfg.HydraCACertPath != "" {
		t.Errorf("HydraCACertPath = %q, want %q", cfg.HydraCACertPath, "")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*app.Config)
		wantErr string
	}{
		{
			name:   "valid defaults",
			modify: func(_ *app.Config) {},
		},
		{
			name:    "empty BridgeBaseURL",
			modify:  func(c *app.Config) { c.BridgeBaseURL = "" },
			wantErr: "SAML_PROVIDER_BRIDGE_BASE_URL must not be empty",
		},
		{
			name:    "invalid BridgeBaseURL",
			modify:  func(c *app.Config) { c.BridgeBaseURL = "://bad" },
			wantErr: "invalid SAML_PROVIDER_BRIDGE_BASE_URL",
		},
		{
			name:    "port zero",
			modify:  func(c *app.Config) { c.BridgeBasePort = 0 },
			wantErr: "SAML_PROVIDER_BRIDGE_BASE_PORT must be 1",
		},
		{
			name:    "port too high",
			modify:  func(c *app.Config) { c.BridgeBasePort = 70000 },
			wantErr: "SAML_PROVIDER_BRIDGE_BASE_PORT must be 1",
		},
		{
			name:    "empty HydraPublicURL",
			modify:  func(c *app.Config) { c.HydraPublicURL = "" },
			wantErr: "SAML_PROVIDER_HYDRA_PUBLIC_URL must not be empty",
		},
		{
			name:    "unsupported HydraPublicURL scheme",
			modify:  func(c *app.Config) { c.HydraPublicURL = "ftp://hydra:4444" },
			wantErr: "SAML_PROVIDER_HYDRA_PUBLIC_URL scheme must be http or https",
		},
		{
			name:    "empty ClientID",
			modify:  func(c *app.Config) { c.ClientID = "" },
			wantErr: "SAML_PROVIDER_OIDC_CLIENT_ID must not be empty",
		},
		{
			name:    "empty ClientSecret",
			modify:  func(c *app.Config) { c.ClientSecret = "" },
			wantErr: "SAML_PROVIDER_OIDC_CLIENT_SECRET must not be empty",
		},
		{
			name:    "DB port out of range",
			modify:  func(c *app.Config) { c.DBPort = 0 },
			wantErr: "SAML_PROVIDER_DB_PORT must be 1",
		},
		{
			name:    "invalid DBSSLMode",
			modify:  func(c *app.Config) { c.DBSSLMode = "bogus" },
			wantErr: "invalid SAML_PROVIDER_DB_SSLMODE",
		},
		{
			name: "MaxConns less than MinConns",
			modify: func(c *app.Config) {
				c.DBMaxConns = 1
				c.DBMinConns = 5
			},
			wantErr: "SAML_PROVIDER_DB_MAX_CONNS (1) must be >= SAML_PROVIDER_DB_MIN_CONNS (5)",
		},
		{
			name:    "empty SAMLCertPath",
			modify:  func(c *app.Config) { c.SAMLCertPath = "" },
			wantErr: "SAML_PROVIDER_CERT_PATH must not be empty",
		},
		{
			name:    "empty SAMLKeyPath",
			modify:  func(c *app.Config) { c.SAMLKeyPath = "" },
			wantErr: "SAML_PROVIDER_KEY_PATH must not be empty",
		},
		{
			name:   "dev mode is valid",
			modify: func(c *app.Config) { c.DevMode = true },
		},
		{
			name:    "invalid non-positive PendingRequestTTL",
			modify:  func(c *app.Config) { c.PendingRequestTTL = 0 },
			wantErr: "SAML_PROVIDER_PENDING_REQUEST_TTL must be positive",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.modify(&cfg)

			err := cfg.Validate()

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tc.wantErr)
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Errorf("Validate() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}
