package app_test

import (
	"testing"
	"time"

	"github.com/canonical/identity-saml-provider/internal/app"
	"github.com/kelseyhightower/envconfig"
)

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
				DBPort:     "5432",
				DBName:     "saml_provider",
			},
			want: "postgres://saml_provider:saml_provider@localhost:5432/saml_provider?sslmode=disable",
		},
		{
			name: "custom values",
			cfg: app.Config{
				DBUser:     "admin",
				DBPassword: "s3cret",
				DBHost:     "db.example.com",
				DBPort:     "5433",
				DBName:     "mydb",
			},
			want: "postgres://admin:s3cret@db.example.com:5433/mydb?sslmode=disable",
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
	cfg := app.Config{
		DBUser:     "user",
		DBPassword: "pass",
		DBHost:     "host",
		DBPort:     "5432",
		DBName:     "db",
	}

	pc := cfg.PoolConfig()

	expectedDSN := "postgres://user:pass@host:5432/db?sslmode=disable"
	if pc.DSN != expectedDSN {
		t.Errorf("PoolConfig().DSN = %q, want %q", pc.DSN, expectedDSN)
	}
	if pc.MaxConns != 10 {
		t.Errorf("PoolConfig().MaxConns = %d, want 10", pc.MaxConns)
	}
	if pc.MinConns != 2 {
		t.Errorf("PoolConfig().MinConns = %d, want 2", pc.MinConns)
	}
	if pc.MaxConnLifetime != 30*time.Minute {
		t.Errorf("PoolConfig().MaxConnLifetime = %v, want 30m", pc.MaxConnLifetime)
	}
	if pc.MaxConnIdleTime != 5*time.Minute {
		t.Errorf("PoolConfig().MaxConnIdleTime = %v, want 5m", pc.MaxConnIdleTime)
	}
}

func TestConfig_HydraConfig(t *testing.T) {
	cfg := app.Config{
		HydraCACertPath:            "/path/to/ca.pem",
		HydraInsecureSkipTLSVerify: true,
		HydraPublicURL:             "https://hydra.example.com",
	}

	hc := cfg.HydraConfig()

	if hc.CACertPath != "/path/to/ca.pem" {
		t.Errorf("HydraConfig().CACertPath = %q, want %q", hc.CACertPath, "/path/to/ca.pem")
	}
	if !hc.InsecureSkipTLSVerify {
		t.Error("HydraConfig().InsecureSkipTLSVerify = false, want true")
	}
	if hc.IssuerURL != "https://hydra.example.com" {
		t.Errorf("HydraConfig().IssuerURL = %q, want %q", hc.IssuerURL, "https://hydra.example.com")
	}
}

func TestConfig_OIDCConfig(t *testing.T) {
	cfg := app.Config{
		ClientID:     "my-client",
		ClientSecret: "my-secret",
		RedirectURL:  "http://localhost:8082/saml/callback",
	}

	oc := cfg.OIDCConfig()

	if oc.ClientID != "my-client" {
		t.Errorf("OIDCConfig().ClientID = %q, want %q", oc.ClientID, "my-client")
	}
	if oc.ClientSecret != "my-secret" {
		t.Errorf("OIDCConfig().ClientSecret = %q, want %q", oc.ClientSecret, "my-secret")
	}
	if oc.RedirectURL != "http://localhost:8082/saml/callback" {
		t.Errorf("OIDCConfig().RedirectURL = %q, want %q", oc.RedirectURL, "http://localhost:8082/saml/callback")
	}
	if len(oc.Scopes) != 3 || oc.Scopes[0] != "openid" || oc.Scopes[1] != "email" || oc.Scopes[2] != "profile" {
		t.Errorf("OIDCConfig().Scopes = %v, want [openid email profile]", oc.Scopes)
	}
}

func TestConfig_SAMLConfig(t *testing.T) {
	cfg := app.Config{
		BridgeBaseURL: "http://localhost:8082",
		SAMLCertPath:  "/path/to/cert.pem",
		SAMLKeyPath:   "/path/to/key.pem",
	}

	sc := cfg.SAMLConfig()

	if sc.BridgeBaseURL != "http://localhost:8082" {
		t.Errorf("SAMLConfig().BridgeBaseURL = %q, want %q", sc.BridgeBaseURL, "http://localhost:8082")
	}
	if sc.CertPath != "/path/to/cert.pem" {
		t.Errorf("SAMLConfig().CertPath = %q, want %q", sc.CertPath, "/path/to/cert.pem")
	}
	if sc.KeyPath != "/path/to/key.pem" {
		t.Errorf("SAMLConfig().KeyPath = %q, want %q", sc.KeyPath, "/path/to/key.pem")
	}
}

func TestConfig_TracingConfig(t *testing.T) {
	cfg := app.Config{
		TracingEnabled:   true,
		OtelGRPCEndpoint: "grpc.example.com:4317",
		OtelHTTPEndpoint: "http.example.com:4318",
		OtelSampler:      "always_on",
		OtelSamplerRatio: 0.5,
	}

	tc := cfg.TracingConfig()

	if !tc.Enabled {
		t.Error("TracingConfig().Enabled = false, want true")
	}
	if tc.OtelGRPCEndpoint != "grpc.example.com:4317" {
		t.Errorf("TracingConfig().OtelGRPCEndpoint = %q, want %q", tc.OtelGRPCEndpoint, "grpc.example.com:4317")
	}
	if tc.OtelHTTPEndpoint != "http.example.com:4318" {
		t.Errorf("TracingConfig().OtelHTTPEndpoint = %q, want %q", tc.OtelHTTPEndpoint, "http.example.com:4318")
	}
	if tc.OtelSampler != "always_on" {
		t.Errorf("TracingConfig().OtelSampler = %q, want %q", tc.OtelSampler, "always_on")
	}
	if tc.OtelSamplerRatio != 0.5 {
		t.Errorf("TracingConfig().OtelSamplerRatio = %f, want 0.5", tc.OtelSamplerRatio)
	}
}

func TestConfig_EnvconfigProcess(t *testing.T) {
	// Verify that envconfig.Process works with the Config struct
	// (all env vars use the same names as the original provider.Config)
	var cfg app.Config
	// Process with no env vars set — should use defaults
	if err := envconfig.Process("", &cfg); err != nil {
		t.Fatalf("envconfig.Process() error: %v", err)
	}

	if cfg.BridgeBasePort != "8082" {
		t.Errorf("BridgeBasePort = %q, want %q", cfg.BridgeBasePort, "8082")
	}
	if cfg.BridgeBaseURL != "http://localhost:8082" {
		t.Errorf("BridgeBaseURL = %q, want %q", cfg.BridgeBaseURL, "http://localhost:8082")
	}
	if cfg.DBHost != "localhost" {
		t.Errorf("DBHost = %q, want %q", cfg.DBHost, "localhost")
	}
	if cfg.DBPort != "5432" {
		t.Errorf("DBPort = %q, want %q", cfg.DBPort, "5432")
	}
	if cfg.ClientID != "service-bridge-client" {
		t.Errorf("ClientID = %q, want %q", cfg.ClientID, "service-bridge-client")
	}
}
