// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package handler_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/handler"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/service"
	"github.com/canonical/identity-saml-provider/mocks"
	"go.uber.org/mock/gomock"
)

func TestHandleOIDCCallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		query      string
		setup      func(deps *testHandlerDeps)
		wantStatus int
		wantHeader string // expected Location header substring
	}{
		{
			name:  "success — redirect to SSO with pending request",
			query: "code=valid-code&state=req-123:relay-abc",
			setup: func(deps *testHandlerDeps) {
				deps.oidc.EXPECT().ExchangeCode(gomock.Any(), "valid-code").Return(&service.OIDCClaims{
					Sub:   "user-sub",
					Email: "user@example.com",
					Name:  "Jane",
				}, nil)
				deps.sessions.EXPECT().CreateFromOIDC(gomock.Any(), gomock.Any()).Return(&domain.Session{
					ID:         "session-1",
					ExpireTime: time.Now().Add(10 * time.Minute),
				}, nil)
				deps.pending.EXPECT().Retrieve(gomock.Any(), "req-123").Return(&domain.PendingAuthnRequest{
					RequestID:   "req-123",
					SAMLRequest: "encoded-saml",
					RelayState:  "relay-abc",
				}, nil)
			},
			wantStatus: http.StatusFound,
			wantHeader: "/saml/sso?",
		},
		{
			name:  "success — no pending request, relay in state",
			query: "code=valid-code&state=req-xyz:relay-state",
			setup: func(deps *testHandlerDeps) {
				deps.oidc.EXPECT().ExchangeCode(gomock.Any(), "valid-code").Return(&service.OIDCClaims{
					Sub:   "user-sub",
					Email: "user@example.com",
				}, nil)
				deps.sessions.EXPECT().CreateFromOIDC(gomock.Any(), gomock.Any()).Return(&domain.Session{
					ID:         "session-2",
					ExpireTime: time.Now().Add(10 * time.Minute),
				}, nil)
				deps.pending.EXPECT().Retrieve(gomock.Any(), "req-xyz").
					Return(nil, &domain.ErrNotFound{Resource: "pending_request", ID: "req-xyz"})
			},
			wantStatus: http.StatusFound,
			wantHeader: "RelayState=relay-state",
		},
		{
			name:       "missing code — 400",
			query:      "state=req-123",
			setup:      func(deps *testHandlerDeps) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:  "token exchange failure — upstream error",
			query: "code=bad-code&state=req-123",
			setup: func(deps *testHandlerDeps) {
				deps.oidc.EXPECT().ExchangeCode(gomock.Any(), "bad-code").
					Return(nil, &domain.ErrUpstream{Service: "hydra", Err: errors.New("connection refused")})
			},
			wantStatus: http.StatusBadGateway,
		},
		{
			name:  "session creation failure — validation error",
			query: "code=valid-code&state=req-123",
			setup: func(deps *testHandlerDeps) {
				deps.oidc.EXPECT().ExchangeCode(gomock.Any(), "valid-code").Return(&service.OIDCClaims{
					Sub: "user-sub",
					// Email is empty → SessionService returns ErrValidation
				}, nil)
				deps.sessions.EXPECT().CreateFromOIDC(gomock.Any(), gomock.Any()).
					Return(nil, &domain.ErrValidation{Field: "email", Message: "email claim is required"})
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			logger := logging.NewNopLogger()
			noopMonitor := &noopMon{}

			deps := &testHandlerDeps{
				sessions: mocks.NewMockSessionService(ctrl),
				sps:      mocks.NewMockServiceProviderService(ctrl),
				mapping:  mocks.NewMockMappingService(ctrl),
				oidc:     mocks.NewMockOIDCService(ctrl),
				pending:  mocks.NewMockPendingRequestService(ctrl),
			}
			tt.setup(deps)

			h := handler.NewHandlers(
				deps.sessions, deps.sps, deps.mapping,
				deps.oidc, deps.pending,
				nil,
				handler.HandlerConfig{BridgeBaseURL: "http://localhost:8082"},
				logger, noopMonitor,
			)

			req := httptest.NewRequest(http.MethodGet, "/saml/callback?"+tt.query, nil)
			rec := httptest.NewRecorder()

			h.HandleOIDCCallback(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantHeader != "" {
				loc := rec.Header().Get("Location")
				if loc == "" || !strings.Contains(loc, tt.wantHeader) {
					t.Errorf("Location = %q, want containing %q", loc, tt.wantHeader)
				}
			}
		})
	}
}

func TestHandleOIDCCallback_CookieSecureAttribute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		devMode    bool
		wantSecure bool
	}{
		{
			name:       "production mode sets Secure cookie",
			devMode:    false,
			wantSecure: true,
		},
		{
			name:       "dev mode omits Secure cookie",
			devMode:    true,
			wantSecure: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			logger := logging.NewNopLogger()
			noopMonitor := &noopMon{}

			deps := &testHandlerDeps{
				sessions: mocks.NewMockSessionService(ctrl),
				sps:      mocks.NewMockServiceProviderService(ctrl),
				mapping:  mocks.NewMockMappingService(ctrl),
				oidc:     mocks.NewMockOIDCService(ctrl),
				pending:  mocks.NewMockPendingRequestService(ctrl),
			}

			deps.oidc.EXPECT().ExchangeCode(gomock.Any(), "valid-code").Return(&service.OIDCClaims{
				Sub:   "user-sub",
				Email: "user@example.com",
			}, nil)
			deps.sessions.EXPECT().CreateFromOIDC(gomock.Any(), gomock.Any()).Return(&domain.Session{
				ID:         "session-1",
				ExpireTime: time.Now().Add(10 * time.Minute),
			}, nil)
			deps.pending.EXPECT().Retrieve(gomock.Any(), "req-123").
				Return(nil, &domain.ErrNotFound{Resource: "pending_request", ID: "req-123"})

			h := handler.NewHandlers(
				deps.sessions, deps.sps, deps.mapping,
				deps.oidc, deps.pending,
				nil,
				handler.HandlerConfig{BridgeBaseURL: "http://localhost:8082", DevMode: tt.devMode},
				logger, noopMonitor,
			)

			req := httptest.NewRequest(http.MethodGet, "/saml/callback?code=valid-code&state=req-123", nil)
			rec := httptest.NewRecorder()

			h.HandleOIDCCallback(rec, req)

			cookies := rec.Result().Cookies()
			var found bool
			for _, c := range cookies {
				if c.Name == "saml_session" {
					found = true
					if c.Secure != tt.wantSecure {
						t.Errorf("cookie Secure = %v, want %v", c.Secure, tt.wantSecure)
					}
				}
			}
			if !found {
				t.Error("saml_session cookie not found in response")
			}
		})
	}
}
