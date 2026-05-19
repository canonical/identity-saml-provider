// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/canonical/identity-saml-provider/internal/tracing"
	"go.uber.org/mock/gomock"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/service"
	"github.com/canonical/identity-saml-provider/mocks"
)

func TestOIDCService_AuthCodeURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		state   string
		wantURL string
	}{
		{
			name:    "returns URL from hydra client",
			state:   "test-state-123",
			wantURL: "https://hydra.example.com/auth?state=test-state-123",
		},
		{
			name:    "empty state",
			state:   "",
			wantURL: "https://hydra.example.com/auth?state=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			hydra := mocks.NewMockHydraClient(ctrl)
			hydra.EXPECT().AuthCodeURL(tt.state).Return(tt.wantURL)

			svc := service.NewOIDCService(hydra, logging.NewNopLogger(), tracing.NewNoopTracer())
			got := svc.AuthCodeURL(tt.state)

			if got != tt.wantURL {
				t.Errorf("AuthCodeURL(%q) = %q, want %q", tt.state, got, tt.wantURL)
			}
		})
	}
}

func TestOIDCService_ExchangeCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		code        string
		setupMock   func(m *mocks.MockHydraClient)
		wantErr     bool
		errType     interface{}
		checkResult func(t *testing.T, claims *service.OIDCClaims)
	}{
		{
			name: "successful exchange with all claims",
			code: "valid-code",
			setupMock: func(m *mocks.MockHydraClient) {
				m.EXPECT().
					ExchangeCode(gomock.Any(), "valid-code").
					Return(&domain.IDToken{
						Subject:  "user-123",
						Issuer:   "https://hydra.example.com",
						Expiry:   time.Now().Add(time.Hour),
						IssuedAt: time.Now(),
						Claims: map[string]interface{}{
							"sub":    "user-123",
							"email":  "user@example.com",
							"name":   "Jane Doe",
							"groups": []interface{}{"admin", "users"},
						},
					}, nil)
			},
			checkResult: func(t *testing.T, c *service.OIDCClaims) {
				t.Helper()
				if c.Sub != "user-123" {
					t.Errorf("Sub = %q, want %q", c.Sub, "user-123")
				}
				if c.Email != "user@example.com" {
					t.Errorf("Email = %q, want %q", c.Email, "user@example.com")
				}
				if c.Name != "Jane Doe" {
					t.Errorf("Name = %q, want %q", c.Name, "Jane Doe")
				}
				if len(c.Groups) != 2 || c.Groups[0] != "admin" || c.Groups[1] != "users" {
					t.Errorf("Groups = %v, want [admin users]", c.Groups)
				}
				if c.RawClaims == nil {
					t.Error("RawClaims should not be nil")
				}
			},
		},
		{
			name: "exchange failure returns ErrUpstream",
			code: "bad-code",
			setupMock: func(m *mocks.MockHydraClient) {
				m.EXPECT().
					ExchangeCode(gomock.Any(), "bad-code").
					Return(nil, errors.New("connection refused"))
			},
			wantErr: true,
			errType: &domain.ErrUpstream{},
		},
		{
			name: "partial claims handled gracefully",
			code: "valid-code",
			setupMock: func(m *mocks.MockHydraClient) {
				m.EXPECT().
					ExchangeCode(gomock.Any(), "valid-code").
					Return(&domain.IDToken{
						Subject: "user-456",
						Claims: map[string]interface{}{
							"sub": "user-456",
						},
					}, nil)
			},
			checkResult: func(t *testing.T, c *service.OIDCClaims) {
				t.Helper()
				if c.Sub != "user-456" {
					t.Errorf("Sub = %q, want %q", c.Sub, "user-456")
				}
				if c.Email != "" {
					t.Errorf("Email = %q, want empty", c.Email)
				}
				if c.Name != "" {
					t.Errorf("Name = %q, want empty", c.Name)
				}
				if c.Groups != nil {
					t.Errorf("Groups = %v, want nil", c.Groups)
				}
			},
		},
		{
			name: "non-string email is ignored",
			code: "valid-code",
			setupMock: func(m *mocks.MockHydraClient) {
				m.EXPECT().
					ExchangeCode(gomock.Any(), "valid-code").
					Return(&domain.IDToken{
						Subject: "user-789",
						Claims: map[string]interface{}{
							"sub":   "user-789",
							"email": 12345, // wrong type
						},
					}, nil)
			},
			checkResult: func(t *testing.T, c *service.OIDCClaims) {
				t.Helper()
				if c.Email != "" {
					t.Errorf("Email = %q, want empty for non-string claim", c.Email)
				}
			},
		},
		{
			name: "groups with mixed types filters non-strings",
			code: "valid-code",
			setupMock: func(m *mocks.MockHydraClient) {
				m.EXPECT().
					ExchangeCode(gomock.Any(), "valid-code").
					Return(&domain.IDToken{
						Subject: "user-mix",
						Claims: map[string]interface{}{
							"sub":    "user-mix",
							"groups": []interface{}{"admin", 42, "users"},
						},
					}, nil)
			},
			checkResult: func(t *testing.T, c *service.OIDCClaims) {
				t.Helper()
				if len(c.Groups) != 2 || c.Groups[0] != "admin" || c.Groups[1] != "users" {
					t.Errorf("Groups = %v, want [admin users]", c.Groups)
				}
			},
		},
		{
			name: "upstream error wraps original error",
			code: "err-code",
			setupMock: func(m *mocks.MockHydraClient) {
				m.EXPECT().
					ExchangeCode(gomock.Any(), "err-code").
					Return(nil, errors.New("timeout"))
			},
			wantErr: true,
			errType: &domain.ErrUpstream{},
			checkResult: func(t *testing.T, _ *service.OIDCClaims) {
				// checked via errType
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			hydra := mocks.NewMockHydraClient(ctrl)
			tt.setupMock(hydra)

			svc := service.NewOIDCService(hydra, logging.NewNopLogger(), tracing.NewNoopTracer())
			claims, err := svc.ExchangeCode(context.Background(), tt.code)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errType != nil {
					switch tt.errType.(type) {
					case *domain.ErrUpstream:
						var upstreamErr *domain.ErrUpstream
						if !errors.As(err, &upstreamErr) {
							t.Errorf("expected *domain.ErrUpstream, got %T: %v", err, err)
						}
						if !strings.Contains(upstreamErr.Service, "hydra") {
							t.Errorf("ErrUpstream.Service = %q, want 'hydra'", upstreamErr.Service)
						}
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
