// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package handler_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/handler"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/mocks"
	"github.com/crewjam/saml"
	"go.uber.org/mock/gomock"
)

func TestSAMLSPAdapter_GetServiceProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		spID      string
		setup     func(sps *mocks.MockServiceProviderService)
		wantErr   bool
		checkDesc func(t *testing.T, desc *saml.EntityDescriptor)
	}{
		{
			name: "found SP",
			spID: "https://sp.example.com",
			setup: func(sps *mocks.MockServiceProviderService) {
				sps.EXPECT().GetByEntityID(gomock.Any(), "https://sp.example.com").Return(&domain.ServiceProvider{
					EntityID:   "https://sp.example.com",
					ACSURL:     "https://sp.example.com/acs",
					ACSBinding: saml.HTTPPostBinding,
				}, nil)
			},
			checkDesc: func(t *testing.T, desc *saml.EntityDescriptor) {
				t.Helper()
				if desc.EntityID != "https://sp.example.com" {
					t.Errorf("EntityID = %q", desc.EntityID)
				}
				if len(desc.SPSSODescriptors) == 0 {
					t.Fatal("expected SPSSODescriptors")
				}
				acs := desc.SPSSODescriptors[0].AssertionConsumerServices
				if len(acs) == 0 {
					t.Fatal("expected ACS endpoints")
				}
				if acs[0].Location != "https://sp.example.com/acs" {
					t.Errorf("ACS Location = %q", acs[0].Location)
				}
			},
		},
		{
			name: "SP not found → os.ErrNotExist",
			spID: "https://unknown.example.com",
			setup: func(sps *mocks.MockServiceProviderService) {
				sps.EXPECT().GetByEntityID(gomock.Any(), "https://unknown.example.com").
					Return(nil, &domain.ErrNotFound{Resource: "service_provider", ID: "https://unknown.example.com"})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			mockSPs := mocks.NewMockServiceProviderService(ctrl)
			tt.setup(mockSPs)

			adapter := &handler.SAMLSPAdapter{SPs: mockSPs}
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			desc, err := adapter.GetServiceProvider(req, tt.spID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if !errors.Is(err, os.ErrNotExist) {
					t.Errorf("error = %v, want os.ErrNotExist", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.checkDesc != nil {
				tt.checkDesc(t, desc)
			}
		})
	}
}

func TestSAMLSessionAdapter_GetSession(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		cookies      []*http.Cookie
		authnReq     *saml.IdpAuthnRequest
		setup        func(deps *testSessionAdapterDeps)
		wantNil      bool
		wantRedirect bool
		checkResult  func(t *testing.T, result *saml.Session, rec *httptest.ResponseRecorder)
	}{
		{
			name: "session exists — returns session without redirect",
			cookies: []*http.Cookie{
				{Name: "saml_session", Value: "session-123"},
			},
			authnReq: &saml.IdpAuthnRequest{
				Request: saml.AuthnRequest{
					ID: "req-1",
					Issuer: &saml.Issuer{
						Value: "https://sp.example.com",
					},
				},
			},
			setup: func(deps *testSessionAdapterDeps) {
				session := &domain.Session{
					ID:             "session-123",
					CreateTime:     time.Now(),
					ExpireTime:     time.Now().Add(10 * time.Minute),
					Index:          "session-123",
					NameID:         "user@example.com",
					UserEmail:      "user@example.com",
					UserCommonName: "Test User",
				}
				deps.sessions.EXPECT().GetByID(gomock.Any(), "session-123").Return(session, nil)
				deps.mapping.EXPECT().ApplyMapping(gomock.Any(), session, "https://sp.example.com").Return(session, nil)
			},
			wantNil:      false,
			wantRedirect: false,
			checkResult: func(t *testing.T, result *saml.Session, _ *httptest.ResponseRecorder) {
				t.Helper()
				if result.ID != "session-123" {
					t.Errorf("ID = %q, want %q", result.ID, "session-123")
				}
				if result.NameID != "user@example.com" {
					t.Errorf("NameID = %q", result.NameID)
				}
			},
		},
		{
			name:    "no session cookie — redirects to OIDC with oauth_nonce cookie",
			cookies: nil,
			authnReq: &saml.IdpAuthnRequest{
				Request: saml.AuthnRequest{
					ID: "req-42",
				},
				RelayState: "my-relay",
			},
			setup: func(deps *testSessionAdapterDeps) {
				deps.pending.EXPECT().Store(gomock.Any(), gomock.Any()).Return(nil)
				deps.oidc.EXPECT().AuthCodeURL(gomock.Any(), gomock.Any()).Return("https://hydra.example.com/auth?state=test")
			},
			wantNil:      true,
			wantRedirect: true,
			checkResult: func(t *testing.T, _ *saml.Session, rec *httptest.ResponseRecorder) {
				t.Helper()
				loc := rec.Header().Get("Location")
				if !strings.Contains(loc, "hydra.example.com/auth") {
					t.Errorf("Location = %q, want OIDC URL", loc)
				}

				// Verify oauth_nonce cookie attributes
				cookies := rec.Result().Cookies()
				var nonceCookie *http.Cookie
				for _, c := range cookies {
					if c.Name == "oauth_nonce" {
						nonceCookie = c
						break
					}
				}
				if nonceCookie == nil {
					t.Fatal("oauth_nonce cookie not found in response")
				}
				if nonceCookie.Path != "/saml/callback" {
					t.Errorf("cookie Path = %q, want %q", nonceCookie.Path, "/saml/callback")
				}
				if !nonceCookie.HttpOnly {
					t.Error("cookie should be HttpOnly")
				}
				if nonceCookie.MaxAge != 600 {
					t.Errorf("cookie MaxAge = %d, want 600", nonceCookie.MaxAge)
				}
				if nonceCookie.SameSite != http.SameSiteLaxMode {
					t.Errorf("cookie SameSite = %d, want Lax", nonceCookie.SameSite)
				}
				if !strings.Contains(nonceCookie.Value, ":") {
					t.Error("cookie value should contain ':' delimiter for dual values")
				}
			},
		},
		{
			name: "session cookie exists but session not found — redirects to OIDC",
			cookies: []*http.Cookie{
				{Name: "saml_session", Value: "gone-session"},
			},
			authnReq: &saml.IdpAuthnRequest{
				Request: saml.AuthnRequest{ID: "req-99"},
			},
			setup: func(deps *testSessionAdapterDeps) {
				deps.sessions.EXPECT().GetByID(gomock.Any(), "gone-session").
					Return(nil, &domain.ErrNotFound{Resource: "session", ID: "gone-session"})
				deps.pending.EXPECT().Store(gomock.Any(), gomock.Any()).Return(nil)
				deps.oidc.EXPECT().AuthCodeURL(gomock.Any(), gomock.Any()).Return("https://hydra.example.com/auth")
			},
			wantNil:      true,
			wantRedirect: true,
			checkResult: func(t *testing.T, _ *saml.Session, rec *httptest.ResponseRecorder) {
				t.Helper()
				loc := rec.Header().Get("Location")
				if loc == "" {
					t.Error("expected Location header")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			deps := &testSessionAdapterDeps{
				sessions: mocks.NewMockSessionService(ctrl),
				mapping:  mocks.NewMockMappingService(ctrl),
				pending:  mocks.NewMockPendingRequestService(ctrl),
				oidc:     mocks.NewMockOIDCService(ctrl),
			}
			tt.setup(deps)

			adapter := &handler.SAMLSessionAdapter{
				Sessions: deps.sessions,
				Mapping:  deps.mapping,
				Pending:  deps.pending,
				OIDC:     deps.oidc,
				Config:   handler.HandlerConfig{BridgeBaseURL: "http://localhost:8082"},
				Logger:   logging.NewNopLogger(),
			}

			req := httptest.NewRequest(http.MethodGet, "/saml/sso?SAMLRequest=encoded-request", nil)
			for _, c := range tt.cookies {
				req.AddCookie(c)
			}
			rec := httptest.NewRecorder()

			result := adapter.GetSession(rec, req, tt.authnReq)

			if tt.wantNil && result != nil {
				t.Error("expected nil session")
			}
			if !tt.wantNil && result == nil {
				t.Fatal("expected non-nil session")
			}
			if tt.wantRedirect && rec.Code != http.StatusFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusFound)
			}
			if !tt.wantRedirect && rec.Code == http.StatusFound {
				t.Error("should not redirect")
			}
			if tt.checkResult != nil {
				tt.checkResult(t, result, rec)
			}
		})
	}
}

type testSessionAdapterDeps struct {
	sessions *mocks.MockSessionService
	mapping  *mocks.MockMappingService
	pending  *mocks.MockPendingRequestService
	oidc     *mocks.MockOIDCService
}
