// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package hydra

import (
	"context"
	"fmt"
	"net/http"
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

// NewClient performs OIDC discovery against the Hydra issuer URL and
// returns a fully initialised Client.
func NewClient(
	ctx context.Context,
	cfg Config,
	oidcCfg OIDCConfig,
	logger logging.Logger,
) (*Client, error) {
	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
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
