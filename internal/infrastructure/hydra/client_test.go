// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package hydra

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
				return
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

// --- Test helpers for CA certificate generation ---

// generateCA creates a self-signed CA certificate and key.
func generateCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate CA key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create CA certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse CA certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	return cert, key, certPEM
}

// generateServerCert creates a server certificate signed by the given CA
// with an IP SAN for 127.0.0.1.
func generateServerCert(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate server key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create server certificate: %v", err)
	}

	leaf, _ := x509.ParseCertificate(certDER)

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
		Leaf:        leaf,
	}
}

// writeTempPEM writes PEM data to a temp file and returns the path.
func writeTempPEM(t *testing.T, pemData []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, pemData, 0o600); err != nil {
		t.Fatalf("failed to write temp PEM: %v", err)
	}
	return path
}

// --- Tests for loadCertPool ---

func Test_loadCertPool(t *testing.T) {
	t.Parallel()

	_, _, validPEM := generateCA(t)

	tests := []struct {
		name       string
		setup      func(t *testing.T) string
		wantErr    bool
		wantErrSub string
	}{
		{
			name: "valid PEM file",
			setup: func(t *testing.T) string {
				return writeTempPEM(t, validPEM)
			},
		},
		{
			name: "nonexistent file",
			setup: func(t *testing.T) string {
				return "/nonexistent/ca.pem"
			},
			wantErr:    true,
			wantErrSub: "failed to read CA certificate",
		},
		{
			name: "invalid PEM content",
			setup: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "bad.pem")
				_ = os.WriteFile(path, []byte("not a PEM block"), 0o600)
				return path
			},
			wantErr:    true,
			wantErrSub: "no valid PEM blocks",
		},
		{
			name: "empty file",
			setup: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "empty.pem")
				_ = os.WriteFile(path, []byte{}, 0o600)
				return path
			},
			wantErr:    true,
			wantErrSub: "no valid PEM blocks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := tt.setup(t)

			pool, err := loadCertPool(path)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if pool == nil {
				t.Error("expected non-nil pool")
			}
		})
	}
}

// --- Tests for buildTransport ---

func Test_buildTransport(t *testing.T) {
	t.Parallel()

	_, _, validPEM := generateCA(t)
	validCAPath := writeTempPEM(t, validPEM)

	invalidPath := filepath.Join(t.TempDir(), "bad.pem")
	_ = os.WriteFile(invalidPath, []byte("not PEM"), 0o600)

	tests := []struct {
		name       string
		issuerURL  string
		caCertPath string
		wantErr    bool
		wantErrSub string
		checkTLS   func(t *testing.T, tr *http.Transport)
	}{
		{
			name:      "http issuer no CA",
			issuerURL: "http://hydra:4444",
			checkTLS: func(t *testing.T, tr *http.Transport) {
				t.Helper()
				if tr.TLSClientConfig != nil && tr.TLSClientConfig.RootCAs != nil {
					t.Error("expected nil RootCAs for HTTP")
				}
			},
		},
		{
			name:       "http issuer with CA ignores CA path",
			issuerURL:  "http://hydra:4444",
			caCertPath: "/some/path.pem",
			checkTLS: func(t *testing.T, tr *http.Transport) {
				t.Helper()
				if tr.TLSClientConfig != nil && tr.TLSClientConfig.RootCAs != nil {
					t.Error("expected nil RootCAs for HTTP")
				}
			},
		},
		{
			name:      "https issuer no CA uses system pool",
			issuerURL: "https://hydra.example.com",
			checkTLS: func(t *testing.T, tr *http.Transport) {
				t.Helper()
				if tr.TLSClientConfig != nil && tr.TLSClientConfig.RootCAs != nil {
					t.Error("expected nil RootCAs (system pool)")
				}
			},
		},
		{
			name:       "https issuer with valid CA uses isolated pool",
			issuerURL:  "https://hydra.example.com",
			caCertPath: validCAPath,
			checkTLS: func(t *testing.T, tr *http.Transport) {
				t.Helper()
				if tr.TLSClientConfig == nil {
					t.Fatal("expected non-nil TLSClientConfig")
				}
				if tr.TLSClientConfig.RootCAs == nil {
					t.Error("expected non-nil RootCAs")
				}
				if tr.TLSClientConfig.MinVersion != tls.VersionTLS12 {
					t.Error("expected MinVersion TLS 1.2")
				}
			},
		},
		{
			name:       "https issuer with missing CA file",
			issuerURL:  "https://hydra.example.com",
			caCertPath: "/nonexistent/ca.pem",
			wantErr:    true,
			wantErrSub: "failed to read CA certificate",
		},
		{
			name:       "https issuer with invalid PEM content",
			issuerURL:  "https://hydra.example.com",
			caCertPath: invalidPath,
			wantErr:    true,
			wantErrSub: "no valid PEM blocks",
		},
		{
			name:       "unsupported URL scheme",
			issuerURL:  "ftp://hydra.example.com",
			wantErr:    true,
			wantErrSub: "unsupported Hydra issuer URL scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tr, err := buildTransport(tt.issuerURL, tt.caCertPath, logging.NewNopLogger())

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tr == nil {
				t.Fatal("expected non-nil transport")
			}
			if tt.checkTLS != nil {
				tt.checkTLS(t, tr)
			}
		})
	}
}

// --- TLS integration tests for NewClient ---

func TestNewClient_WithCustomCA(t *testing.T) {
	t.Parallel()

	// Generate CA and server cert with IP SAN for 127.0.0.1.
	ca, caKey, caPEM := generateCA(t)
	serverCert := generateServerCert(t, ca, caKey)

	// Start a TLS server with OIDC discovery endpoints.
	mux := http.NewServeMux()
	var issuerURL string
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 issuerURL,
			"authorization_endpoint": issuerURL + "/oauth2/auth",
			"token_endpoint":         issuerURL + "/oauth2/token",
			"jwks_uri":               issuerURL + "/.well-known/jwks.json",
		})
	})
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	})

	ts := httptest.NewUnstartedServer(mux)
	ts.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}}
	ts.StartTLS()
	t.Cleanup(ts.Close)
	issuerURL = ts.URL

	caPath := writeTempPEM(t, caPEM)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := NewClient(ctx, Config{
		IssuerURL:  issuerURL,
		CACertPath: caPath,
		DevMode:    true,
	}, OIDCConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost/callback",
	}, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("NewClient() with custom CA: unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient() returned nil")
	}

	// Verify the client can generate an AuthCodeURL.
	authURL := c.AuthCodeURL("test-state", "test-nonce")
	if !strings.Contains(authURL, "oauth2/auth") {
		t.Errorf("AuthCodeURL() = %q, want substring %q", authURL, "oauth2/auth")
	}
}

func TestNewClient_WithWrongCA(t *testing.T) {
	t.Parallel()

	// Generate CA-A (used to sign the server cert).
	caA, caAKey, _ := generateCA(t)
	serverCert := generateServerCert(t, caA, caAKey)

	// Generate CA-B (provided as the trusted CA — wrong one).
	_, _, caBPEM := generateCA(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts := httptest.NewUnstartedServer(mux)
	ts.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}}
	ts.StartTLS()
	t.Cleanup(ts.Close)

	caPath := writeTempPEM(t, caBPEM)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := NewClient(ctx, Config{
		IssuerURL:  ts.URL,
		CACertPath: caPath,
		DevMode:    true,
	}, OIDCConfig{ClientID: "test"}, logging.NewNopLogger())

	if err == nil {
		t.Fatal("NewClient() with wrong CA: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "certificate signed by unknown authority") &&
		!strings.Contains(err.Error(), "certificate is not trusted") {
		t.Errorf("error = %q, want CA trust failure", err.Error())
	}
}
