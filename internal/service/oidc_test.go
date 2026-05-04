package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/service"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// mockTokenVerifier implements service.OIDCTokenVerifier for testing.
type mockTokenVerifier struct {
	token service.OIDCIDToken
	err   error
}

func (m *mockTokenVerifier) Verify(_ context.Context, _ string) (service.OIDCIDToken, error) {
	return m.token, m.err
}

// mockIDToken implements service.OIDCIDToken for testing.
type mockIDToken struct {
	claims interface{}
	err    error
}

func (m *mockIDToken) Claims(v interface{}) error {
	if m.err != nil {
		return m.err
	}
	data, err := json.Marshal(m.claims)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func TestOIDCService_AuthCodeURL(t *testing.T) {
	t.Parallel()

	oauth2Config := &oauth2.Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://hydra.example.com/oauth2/auth",
			TokenURL: "https://hydra.example.com/oauth2/token",
		},
		RedirectURL: "https://bridge.example.com/callback",
		Scopes:      []string{"openid", "email", "profile"},
	}

	svc := service.NewOIDCService(oauth2Config, nil, zap.NewNop().Sugar())
	url := svc.AuthCodeURL("test-state-123")

	if url == "" {
		t.Fatal("AuthCodeURL returned empty string")
	}
	// Should contain the auth URL and state
	if !contains(url, "hydra.example.com/oauth2/auth") {
		t.Errorf("URL should contain auth endpoint, got %q", url)
	}
	if !contains(url, "state=test-state-123") {
		t.Errorf("URL should contain state parameter, got %q", url)
	}
}

func TestOIDCService_ExchangeCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		tokenServer func() *httptest.Server
		verifier    service.OIDCTokenVerifier
		wantErr     bool
		errType     interface{}
		checkResult func(t *testing.T, claims *service.OIDCClaims)
	}{
		{
			name: "successful exchange with raw claims",
			tokenServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"access_token": "access-token-123",
						"token_type":   "Bearer",
						"id_token":     "raw-id-token-123",
					})
				}))
			},
			verifier: &mockTokenVerifier{
				token: &mockIDToken{
					claims: map[string]interface{}{
						"sub":    "user-123",
						"email":  "user@example.com",
						"name":   "Jane Doe",
						"groups": []string{"admin", "users"},
					},
				},
			},
			checkResult: func(t *testing.T, claims *service.OIDCClaims) {
				t.Helper()
				if claims.Sub != "user-123" {
					t.Errorf("Sub = %q, want %q", claims.Sub, "user-123")
				}
				if claims.Email != "user@example.com" {
					t.Errorf("Email = %q, want %q", claims.Email, "user@example.com")
				}
				if claims.Name != "Jane Doe" {
					t.Errorf("Name = %q, want %q", claims.Name, "Jane Doe")
				}
				if claims.RawClaims == nil {
					t.Error("RawClaims should not be nil")
				}
			},
		},
		{
			name: "token exchange failure returns ErrUpstream",
			tokenServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = fmt.Fprint(w, "internal server error")
				}))
			},
			wantErr: true,
			errType: &domain.ErrUpstream{},
		},
		{
			name: "invalid ID token returns ErrAuthentication",
			tokenServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"access_token": "access-token-123",
						"token_type":   "Bearer",
						"id_token":     "invalid-token",
					})
				}))
			},
			verifier: &mockTokenVerifier{
				err: errors.New("token signature verification failed"),
			},
			wantErr: true,
			errType: &domain.ErrAuthentication{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := tt.tokenServer()
			defer ts.Close()

			oauth2Config := &oauth2.Config{
				ClientID:     "test-client",
				ClientSecret: "test-secret",
				Endpoint: oauth2.Endpoint{
					TokenURL: ts.URL,
				},
				RedirectURL: "https://bridge.example.com/callback",
			}

			verifier := tt.verifier
			if verifier == nil {
				verifier = &mockTokenVerifier{}
			}

			svc := service.NewOIDCService(oauth2Config, verifier, zap.NewNop().Sugar())
			claims, err := svc.ExchangeCode(context.Background(), "test-code")

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				switch tt.errType.(type) {
				case *domain.ErrUpstream:
					var upstreamErr *domain.ErrUpstream
					if !errors.As(err, &upstreamErr) {
						t.Errorf("expected *domain.ErrUpstream, got %T: %v", err, err)
					}
				case *domain.ErrAuthentication:
					var authErr *domain.ErrAuthentication
					if !errors.As(err, &authErr) {
						t.Errorf("expected *domain.ErrAuthentication, got %T: %v", err, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.checkResult != nil {
				tt.checkResult(t, claims)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
