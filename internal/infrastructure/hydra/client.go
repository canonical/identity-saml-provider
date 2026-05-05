package hydra

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Config holds the HTTP client configuration for connecting to Ory Hydra.
type Config struct {
	// CACertPath is the optional file path to a custom CA certificate (PEM format).
	CACertPath string
	// InsecureSkipTLSVerify disables TLS certificate verification.
	// WARNING: Do not use this in production.
	InsecureSkipTLSVerify bool
	// IssuerURL is the Hydra public URL used for OIDC discovery.
	IssuerURL string
}

// OIDCConfig holds the OIDC client credentials and redirect settings.
type OIDCConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// DiscoveryResult holds the result of OIDC provider discovery.
type DiscoveryResult struct {
	OAuth2Config *oauth2.Config
	Verifier     *oidc.IDTokenVerifier
}

// NewClient builds an *http.Client configured for communicating with Ory Hydra.
// It supports optional custom CA certificates and insecure TLS skipping.
func NewClient(cfg Config) (*http.Client, error) {
	var rootCAs *x509.CertPool

	if cfg.CACertPath != "" {
		caCert, err := os.ReadFile(cfg.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read Hydra CA certificate %q: %w", cfg.CACertPath, err)
		}

		rootCAs, err = x509.SystemCertPool()
		if err != nil || rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}

		if ok := rootCAs.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("failed to parse Hydra CA certificate PEM from %q", cfg.CACertPath)
		}
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipTLSVerify, //nolint:gosec // controlled by operator config
		RootCAs:            rootCAs,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}, nil
}

// DiscoverOIDC performs OIDC provider discovery against the Hydra issuer URL
// and returns an OAuth2Config and IDTokenVerifier ready for use.
func DiscoverOIDC(ctx context.Context, httpClient *http.Client, cfg Config, oidcCfg OIDCConfig) (*DiscoveryResult, error) {
	// Inject the custom HTTP client into the context for the OIDC library.
	if httpClient != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	}

	// InsecureIssuerURLContext allows local testing where the issuer URL
	// seen by the provider may not match the publicly-facing URL.
	ctx = oidc.InsecureIssuerURLContext(ctx, cfg.IssuerURL)

	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to query Hydra OIDC provider: %w", err)
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: oidcCfg.ClientID})

	scopes := oidcCfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "email", "profile"}
	}

	oauth2Config := &oauth2.Config{
		ClientID:     oidcCfg.ClientID,
		ClientSecret: oidcCfg.ClientSecret,
		RedirectURL:  oidcCfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	return &DiscoveryResult{
		OAuth2Config: oauth2Config,
		Verifier:     verifier,
	}, nil
}
