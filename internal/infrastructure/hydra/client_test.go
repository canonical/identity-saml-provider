// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package hydra

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/canonical/identity-saml-provider/internal/logging"
)

// newTestClient builds a Client with a custom oauth2Config and
// verifier for unit testing, bypassing OIDC discovery.
func newTestClient(
	oauth2Cfg *oauth2.Config,
	verifier *oidc.IDTokenVerifier,
	httpClient *http.Client,
) *Client {
	return &Client{
		oauth2Config: oauth2Cfg,
		verifier:     verifier,
		httpClient:   httpClient,
		logger:       logging.NewNopLogger(),
	}
}

func TestClient_AuthCodeURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		state   string
		nonce   string
		authURL string
		wantSub []string // substrings the URL must contain
	}{
		{
			name:    "includes auth endpoint, state, and nonce",
			state:   "test-state-123",
			nonce:   "test-nonce-456",
			authURL: "https://hydra.example.com/oauth2/auth",
			wantSub: []string{
				"hydra.example.com/oauth2/auth",
				"state=test-state-123",
				"nonce=test-nonce-456",
			},
		},
		{
			name:    "empty state and nonce",
			state:   "",
			nonce:   "",
			authURL: "https://hydra.example.com/oauth2/auth",
			wantSub: []string{
				"hydra.example.com/oauth2/auth",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := newTestClient(
				&oauth2.Config{
					ClientID: "test-client",
					Endpoint: oauth2.Endpoint{AuthURL: tt.authURL},
				},
				nil, nil,
			)

			got := c.AuthCodeURL(tt.state, tt.nonce)
			if got == "" {
				t.Fatal("AuthCodeURL returned empty string")
			}
			for _, sub := range tt.wantSub {
				if !strings.Contains(got, sub) {
					t.Errorf("AuthCodeURL() = %q, want substring %q", got, sub)
				}
			}
		})
	}
}

func TestClient_ExchangeCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantErr    bool
		wantErrSub string
	}{
		{
			name: "token exchange failure",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr:    true,
			wantErrSub: "token exchange",
		},
		{
			name: "missing id_token in response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"access_token": "at-123",
					"token_type":   "Bearer",
				})
			},
			wantErr:    true,
			wantErrSub: "no id_token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := httptest.NewServer(tt.handler)
			defer ts.Close()

			c := newTestClient(
				&oauth2.Config{
					ClientID:     "test-client",
					ClientSecret: "test-secret",
					Endpoint:     oauth2.Endpoint{TokenURL: ts.URL},
				},
				nil, nil,
			)

			_, err := c.ExchangeCode(context.Background(), "test-code", "test-nonce")
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.wantErrSub != "" && !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErrSub)
			}
		})
	}
}

func TestNewClient_DiscoveryFailure(t *testing.T) {
	t.Parallel()

	_, err := NewClient(
		context.Background(),
		Config{IssuerURL: "http://127.0.0.1:1/nonexistent"},
		OIDCConfig{ClientID: "test"},
		logging.NewNopLogger(),
	)
	if err == nil {
		t.Fatal("NewClient() expected error for unreachable issuer, got nil")
	}
	if !strings.Contains(err.Error(), "failed to query Hydra OIDC provider") {
		t.Errorf("error = %q, want substring %q", err.Error(), "failed to query Hydra OIDC provider")
	}
}

func TestNewClient_WithDiscovery(t *testing.T) {
	t.Parallel()

	// Simulate a minimal OIDC discovery endpoint.
	discoveryHandler := http.NewServeMux()
	var issuerURL string
	discoveryHandler.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 issuerURL,
			"authorization_endpoint": issuerURL + "/oauth2/auth",
			"token_endpoint":         issuerURL + "/oauth2/token",
			"jwks_uri":               issuerURL + "/.well-known/jwks.json",
		})
	})
	discoveryHandler.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	})
	ts := httptest.NewServer(discoveryHandler)
	t.Cleanup(ts.Close)
	issuerURL = ts.URL

	tests := []struct {
		name    string
		oidcCfg OIDCConfig
		wantN   int // expected number of scopes
	}{
		{
			name: "default scopes",
			oidcCfg: OIDCConfig{
				ClientID:     "client-1",
				ClientSecret: "secret-1",
				RedirectURL:  "http://localhost/callback",
			},
			wantN: 3, // openid, email, profile
		},
		{
			name: "custom scopes",
			oidcCfg: OIDCConfig{
				ClientID:     "client-2",
				ClientSecret: "secret-2",
				RedirectURL:  "http://localhost/callback",
				Scopes:       []string{"openid", "custom"},
			},
			wantN: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			c, err := NewClient(ctx, Config{IssuerURL: ts.URL}, tt.oidcCfg, logging.NewNopLogger())
			if err != nil {
				t.Fatalf("NewClient() unexpected error: %v", err)
			}
			if c == nil {
				t.Fatal("NewClient() returned nil")
			}
			if c.httpClient == nil {
				t.Error("NewClient().httpClient is nil")
			}
			if c.verifier == nil {
				t.Error("NewClient().verifier is nil")
			}
			if len(c.oauth2Config.Scopes) != tt.wantN {
				t.Errorf("scopes = %v (len %d), want len %d",
					c.oauth2Config.Scopes, len(c.oauth2Config.Scopes), tt.wantN)
			}
		})
	}
}
