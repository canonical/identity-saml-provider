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
	"github.com/canonical/identity-saml-provider/internal/infrastructure/samlkit"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/monitoring"
	"github.com/canonical/identity-saml-provider/mocks"
	"github.com/crewjam/saml"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
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
		modifyReq    func(r *http.Request)
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
					return
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
		{
			name:    "no session cookie — extracts client metadata from proxy headers",
			cookies: nil,
			authnReq: &saml.IdpAuthnRequest{
				Request: saml.AuthnRequest{
					ID: "req-with-headers",
				},
				RelayState: "relay-headers",
			},
			modifyReq: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "192.168.1.100, 10.0.0.1")
				r.Header.Set("User-Agent", "Mozilla-Custom/1.0")
			},
			setup: func(deps *testSessionAdapterDeps) {
				deps.pending.EXPECT().Store(gomock.Any(), &pendingRequestMatcher{
					expectedID: "req-with-headers",
					check: func(p *domain.PendingAuthnRequest) bool {
						return p.SAMLRequest == "encoded-request" &&
							p.RelayState == "relay-headers" &&
							p.ClientMetadata != nil &&
							p.ClientMetadata["client_ip"] == "192.168.1.100" &&
							p.ClientMetadata["user_agent"] == "Mozilla-Custom/1.0" &&
							!p.ExpireAt.IsZero() &&
							p.ExpireAt.Sub(p.CreatedAt) == 15*time.Minute
					},
				}).Return(nil)
				deps.oidc.EXPECT().AuthCodeURL(gomock.Any(), gomock.Any()).Return("https://hydra.example.com/auth")
			},
			wantNil:      true,
			wantRedirect: true,
		},
		{
			name:    "no session cookie — extracts client metadata fallback to RemoteAddr",
			cookies: nil,
			authnReq: &saml.IdpAuthnRequest{
				Request: saml.AuthnRequest{
					ID: "req-with-remoteaddr",
				},
				RelayState: "relay-remoteaddr",
			},
			modifyReq: func(r *http.Request) {
				r.RemoteAddr = "127.0.0.1:12345"
				r.Header.Set("User-Agent", "Mozilla-Remote/2.0")
			},
			setup: func(deps *testSessionAdapterDeps) {
				deps.pending.EXPECT().Store(gomock.Any(), &pendingRequestMatcher{
					expectedID: "req-with-remoteaddr",
					check: func(p *domain.PendingAuthnRequest) bool {
						return p.ClientMetadata != nil &&
							p.ClientMetadata["client_ip"] == "127.0.0.1" &&
							p.ClientMetadata["user_agent"] == "Mozilla-Remote/2.0" &&
							!p.ExpireAt.IsZero() &&
							p.ExpireAt.Sub(p.CreatedAt) == 15*time.Minute
					},
				}).Return(nil)
				deps.oidc.EXPECT().AuthCodeURL(gomock.Any(), gomock.Any()).Return("https://hydra.example.com/auth")
			},
			wantNil:      true,
			wantRedirect: true,
		},
		{
			name: "ApplyMapping failure aborts assertion with 500 — fail closed",
			cookies: []*http.Cookie{
				{Name: "saml_session", Value: "session-fc"},
			},
			authnReq: &saml.IdpAuthnRequest{
				Request: saml.AuthnRequest{
					ID: "req-fc",
					Issuer: &saml.Issuer{
						Value: "https://sp.example.com",
					},
				},
			},
			setup: func(deps *testSessionAdapterDeps) {
				session := &domain.Session{
					ID:         "session-fc",
					ExpireTime: time.Now().Add(10 * time.Minute),
				}
				deps.sessions.EXPECT().GetByID(gomock.Any(), "session-fc").Return(session, nil)
				deps.mapping.EXPECT().
					ApplyMapping(gomock.Any(), session, "https://sp.example.com").
					Return(nil, &domain.ErrNameIDResolution{
						EntityID: "https://sp.example.com",
						Format:   "persistent",
						Reason:   "missing or empty OIDC sub claim in session",
					})
			},
			wantNil:      true,
			wantRedirect: false,
			checkResult: func(t *testing.T, _ *saml.Session, rec *httptest.ResponseRecorder) {
				t.Helper()
				if rec.Code != http.StatusInternalServerError {
					t.Errorf("status = %d, want 500", rec.Code)
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
				Config: handler.HandlerConfig{
					BridgeBaseURL:     "http://localhost:8082",
					PendingRequestTTL: 15 * time.Minute,
				},
				Logger: logging.NewNopLogger(),
			}

			req := httptest.NewRequest(http.MethodGet, "/saml/sso?SAMLRequest=encoded-request", nil)
			for _, c := range tt.cookies {
				req.AddCookie(c)
			}
			if tt.modifyReq != nil {
				tt.modifyReq(req)
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

func TestHandlers_HandleSSO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		method         string
		url            string
		setupIDP       func() *saml.IdentityProvider
		wantStatus     int
		wantJSON       bool
		wantBodySubset string
	}{
		{
			name:           "GET with missing SAMLRequest → 400 Bad Request JSON",
			method:         http.MethodGet,
			url:            "/saml/sso",
			wantStatus:     http.StatusBadRequest,
			wantJSON:       true,
			wantBodySubset: `"message":"missing or expired SAMLRequest parameter"`,
		},
		{
			name:           "GET with empty SAMLRequest → 400 Bad Request JSON",
			method:         http.MethodGet,
			url:            "/saml/sso?SAMLRequest=",
			wantStatus:     http.StatusBadRequest,
			wantJSON:       true,
			wantBodySubset: `"message":"missing or expired SAMLRequest parameter"`,
		},
		{
			name:   "GET with present but invalid SAMLRequest → 400 Bad Request (from IDP)",
			method: http.MethodGet,
			url:    "/saml/sso?SAMLRequest=invalid_base64_data",
			setupIDP: func() *saml.IdentityProvider {
				return &saml.IdentityProvider{
					Logger: samlkit.NewZapLoggerAdapter(zap.NewNop().Sugar()),
				}
			},
			wantStatus: http.StatusBadRequest,
			wantJSON:   false,
		},
		{
			name:   "POST with empty SAMLRequest → 400 Bad Request (from IDP)",
			method: http.MethodPost,
			url:    "/saml/sso",
			setupIDP: func() *saml.IdentityProvider {
				return &saml.IdentityProvider{
					Logger: samlkit.NewZapLoggerAdapter(zap.NewNop().Sugar()),
				}
			},
			wantStatus: http.StatusBadRequest,
			wantJSON:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			sessions := mocks.NewMockSessionService(ctrl)
			sps := mocks.NewMockServiceProviderService(ctrl)
			mapping := mocks.NewMockMappingService(ctrl)
			oidc := mocks.NewMockOIDCService(ctrl)
			pending := mocks.NewMockPendingRequestService(ctrl)

			var idp *saml.IdentityProvider
			if tt.setupIDP != nil {
				idp = tt.setupIDP()
			}

			h := handler.NewHandlers(
				sessions, sps, mapping, oidc, pending, idp,
				handler.HandlerConfig{BridgeBaseURL: "http://localhost:8082"},
				logging.NewNopLogger(),
				monitoring.NewNoopMonitor(),
			)

			req := httptest.NewRequest(tt.method, tt.url, nil)
			if tt.method == http.MethodPost {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			rec := httptest.NewRecorder()

			h.HandleSSO(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			body := rec.Body.String()
			if tt.wantJSON {
				ct := rec.Header().Get("Content-Type")
				if !strings.Contains(ct, "application/json") {
					t.Errorf("Content-Type = %q, want containing application/json", ct)
				}
			}
			if tt.wantBodySubset != "" && !strings.Contains(body, tt.wantBodySubset) {
				t.Errorf("body %q does not contain %q", body, tt.wantBodySubset)
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

type pendingRequestMatcher struct {
	expectedID string
	check      func(p *domain.PendingAuthnRequest) bool
}

func (m *pendingRequestMatcher) Matches(x interface{}) bool {
	p, ok := x.(*domain.PendingAuthnRequest)
	if !ok {
		return false
	}
	return p.RequestID == m.expectedID && (m.check == nil || m.check(p))
}

func (m *pendingRequestMatcher) String() string {
	return "is pending request with ID " + m.expectedID
}
