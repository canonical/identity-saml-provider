// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package hydra

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"

	"github.com/canonical/identity-saml-provider/internal/crypto"
	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
)

// Config holds the Hydra connection settings.
type Config struct {
	// IssuerURL is the Hydra public URL used for OIDC discovery.
	IssuerURL string

	// DevMode, when true, enables InsecureIssuerURLContext for
	// local development where the issuer URL seen by the provider
	// may differ from the publicly-facing URL.
	DevMode bool

	// CACertPath is the path to a PEM file containing the CA
	// certificate(s) to trust for Hydra HTTPS connections. When
	// set and the IssuerURL uses https://, an isolated certificate
	// pool is created containing only these certificates. When
	// empty, Go's default system certificate pool is used.
	CACertPath string
}

// OIDCConfig holds the OIDC client credentials and redirect settings.
type OIDCConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// Client is the Hydra OIDC client. It handles auth URL generation,
// token exchange, and ID token verification.
type Client struct {
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
	httpClient   *http.Client
	logger       logging.Logger
}

// loadCertPool reads a PEM file and returns an isolated
// x509.CertPool containing only the certificates from that file.
func loadCertPool(path string) (*x509.CertPool, error) {
	pemData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate %q: %w", path, err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemData) {
		return nil, fmt.Errorf("CA certificate file %q contains no valid PEM blocks", path)
	}

	return pool, nil
}

// buildTransport returns an *http.Transport configured for the given
// issuer URL and optional CA certificate.
func buildTransport(issuerURL, caCertPath string, logger logging.Logger) (*http.Transport, error) {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("unexpected default transport type %T", http.DefaultTransport)
	}
	transport := base.Clone()

	u, err := url.Parse(issuerURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Hydra issuer URL %q: %w", issuerURL, err)
	}

	switch u.Scheme {
	case "https":
		// handled below
	case "http":
		if caCertPath != "" {
			logger.Warnw("CACertPath is set but issuer URL uses plain HTTP; CA certificate will not be used",
				"issuer_url", issuerURL,
				"ca_cert_path", caCertPath,
			)
		}
		logger.Infow("hydra transport: plain HTTP (no TLS)", "issuer_url", issuerURL)
		return transport, nil
	default:
		return nil, fmt.Errorf("unsupported Hydra issuer URL scheme %q", u.Scheme)
	}

	if caCertPath == "" {
		logger.Infow("hydra TLS: system certificate pool", "issuer_url", issuerURL)
		return transport, nil
	}

	pool, err := loadCertPool(caCertPath)
	if err != nil {
		return nil, err
	}

	transport.TLSClientConfig = &tls.Config{
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12,
	}

	logger.Infow("hydra TLS: custom CA (isolated pool)",
		"issuer_url", issuerURL,
		"ca_cert_path", caCertPath,
	)

	return transport, nil
}

// NewClient performs OIDC discovery against the Hydra issuer URL and
// returns a fully initialised Client.
func NewClient(
	ctx context.Context,
	cfg Config,
	oidcCfg OIDCConfig,
	logger logging.Logger,
) (*Client, error) {
	transport, err := buildTransport(cfg.IssuerURL, cfg.CACertPath, logger)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: otelhttp.NewTransport(transport),
	}

	// Inject httpClient into context for OIDC discovery.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	if cfg.DevMode {
		// InsecureIssuerURLContext allows local testing where the
		// issuer URL seen by the provider may not match the
		// publicly-facing URL.
		ctx = oidc.InsecureIssuerURLContext(ctx, cfg.IssuerURL)
	}

	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to query Hydra OIDC provider: %w", err)
	}

	scopes := oidcCfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "email", "profile"}
	}

	return &Client{
		oauth2Config: &oauth2.Config{
			ClientID:     oidcCfg.ClientID,
			ClientSecret: oidcCfg.ClientSecret,
			RedirectURL:  oidcCfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       scopes,
		},
		verifier: provider.Verifier(&oidc.Config{
			ClientID: oidcCfg.ClientID,
		}),
		httpClient: httpClient,
		logger:     logger,
	}, nil
}

// AuthCodeURL returns the URL to redirect the user to for OIDC
// authentication. The nonce is included as an OIDC authorization
// parameter so that Hydra stamps it into the ID token.
func (c *Client) AuthCodeURL(state, nonce string) string {
	return c.oauth2Config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("nonce", nonce))
}

// ExchangeCode exchanges an authorization code for an OAuth2 token,
// verifies the embedded ID token (including nonce claim), and returns
// a domain IDToken with structured metadata and raw claims.
func (c *Client) ExchangeCode(ctx context.Context, code, expectedNonce string) (*domain.IDToken, error) {
	// Inject the same httpClient used for discovery so that TLS
	// trust is consistent across all outbound calls.
	if c.httpClient != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)
	}

	token, err := c.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	idToken, err := c.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("id token verification: %w", err)
	}

	if !crypto.NonceEqual(idToken.Nonce, expectedNonce) {
		return nil, &domain.ErrAuthentication{
			Reason: "id token nonce mismatch",
		}
	}

	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims: %w", err)
	}

	return &domain.IDToken{
		Issuer:   idToken.Issuer,
		Subject:  idToken.Subject,
		Expiry:   idToken.Expiry,
		IssuedAt: idToken.IssuedAt,
		Claims:   claims,
	}, nil
}
