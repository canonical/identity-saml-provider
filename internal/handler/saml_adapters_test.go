// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package handler_test

import (
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
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

func TestEnrichMetadataXML(t *testing.T) {
	t.Parallel()

	validMetadataXML := `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="http://localhost:8082/saml/metadata">
  <IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <KeyDescriptor use="signing">
      <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#">
        <X509Data>
          <X509Certificate>MIICTestSigningCert</X509Certificate>
        </X509Data>
      </KeyInfo>
    </KeyDescriptor>
    <KeyDescriptor use="encryption">
      <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#">
        <X509Data>
          <X509Certificate>MIICTestEncryptionCert</X509Certificate>
        </X509Data>
      </KeyInfo>
    </KeyDescriptor>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="http://localhost:8082/saml/sso"/>
  </IDPSSODescriptor>
</EntityDescriptor>`

	t.Run("injects extensions before key descriptor and preserves keys", func(t *testing.T) {
		t.Parallel()

		sigURI := "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"
		digestURI := "http://www.w3.org/2001/04/xmlenc#sha256"

		enriched, err := handler.EnrichMetadataXML([]byte(validMetadataXML), sigURI, digestURI)
		if err != nil {
			t.Fatalf("EnrichMetadataXML() error: %v", err)
		}

		doc := etree.NewDocument()
		if err := doc.ReadFromBytes(enriched); err != nil {
			t.Fatalf("failed to parse enriched XML: %v", err)
		}

		idpSSO := doc.FindElement("//IDPSSODescriptor")
		if idpSSO == nil {
			t.Fatal("missing IDPSSODescriptor")
		}

		// Verify Extensions exists and has correct xmlns:alg
		extensions := idpSSO.SelectElement("Extensions")
		if extensions == nil {
			t.Fatal("missing Extensions element in IDPSSODescriptor")
		}
		if algAttr := extensions.SelectAttr("xmlns:alg"); algAttr == nil || algAttr.Value != "urn:oasis:names:tc:SAML:metadata:algsupport" {
			t.Errorf("xmlns:alg attribute missing or wrong: %v", algAttr)
		}

		// Verify Extensions is positioned before the first KeyDescriptor
		keyDesc := idpSSO.SelectElement("KeyDescriptor")
		if keyDesc == nil {
			t.Fatal("missing KeyDescriptor")
		}
		if extensions.Index() >= keyDesc.Index() {
			t.Errorf("Extensions index (%d) should be < KeyDescriptor index (%d)", extensions.Index(), keyDesc.Index())
		}

		// Verify SigningMethod and DigestMethod elements
		sm := extensions.FindElement("alg:SigningMethod")
		if sm == nil {
			t.Fatal("missing alg:SigningMethod")
		}
		if algo := sm.SelectAttrValue("Algorithm", ""); algo != sigURI {
			t.Errorf("alg:SigningMethod Algorithm = %q, want %q", algo, sigURI)
		}

		dm := extensions.FindElement("alg:DigestMethod")
		if dm == nil {
			t.Fatal("missing alg:DigestMethod")
		}
		if algo := dm.SelectAttrValue("Algorithm", ""); algo != digestURI {
			t.Errorf("alg:DigestMethod Algorithm = %q, want %q", algo, digestURI)
		}

		// Verify both KeyDescriptor elements (signing and encryption) remain intact
		keyDescriptors := idpSSO.SelectElements("KeyDescriptor")
		if len(keyDescriptors) != 2 {
			t.Fatalf("expected 2 KeyDescriptor elements, got %d", len(keyDescriptors))
		}
		if use := keyDescriptors[0].SelectAttrValue("use", ""); use != "signing" {
			t.Errorf("first KeyDescriptor use = %q, want 'signing'", use)
		}
		if use := keyDescriptors[1].SelectAttrValue("use", ""); use != "encryption" {
			t.Errorf("second KeyDescriptor use = %q, want 'encryption'", use)
		}
	})

	t.Run("handles existing Extensions without duplicating", func(t *testing.T) {
		t.Parallel()

		xmlWithExtensions := `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <Extensions>
      <ExistingCustomExtension>custom_value</ExistingCustomExtension>
    </Extensions>
    <KeyDescriptor use="signing"/>
  </IDPSSODescriptor>
</EntityDescriptor>`

		sigURI := "http://www.w3.org/2001/04/xmldsig-more#rsa-sha384"
		digestURI := "http://www.w3.org/2001/04/xmldsig-more#sha384"

		enriched, err := handler.EnrichMetadataXML([]byte(xmlWithExtensions), sigURI, digestURI)
		if err != nil {
			t.Fatalf("EnrichMetadataXML() error: %v", err)
		}

		doc := etree.NewDocument()
		if err := doc.ReadFromBytes(enriched); err != nil {
			t.Fatalf("failed to parse enriched XML: %v", err)
		}

		extensions := doc.FindElements("//IDPSSODescriptor/Extensions")
		if len(extensions) != 1 {
			t.Fatalf("expected exactly 1 Extensions element, got %d", len(extensions))
		}

		if extensions[0].SelectElement("ExistingCustomExtension") == nil {
			t.Error("existing custom extension was removed")
		}
		if extensions[0].FindElement("alg:SigningMethod") == nil {
			t.Error("missing alg:SigningMethod")
		}
		if extensions[0].FindElement("alg:DigestMethod") == nil {
			t.Error("missing alg:DigestMethod")
		}
	})

	t.Run("does not duplicate alg elements if already present", func(t *testing.T) {
		t.Parallel()

		xmlWithAlgs := `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <Extensions xmlns:alg="urn:oasis:names:tc:SAML:metadata:algsupport">
      <alg:SigningMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"/>
      <alg:DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256"/>
    </Extensions>
    <KeyDescriptor use="signing"/>
  </IDPSSODescriptor>
</EntityDescriptor>`

		enriched, err := handler.EnrichMetadataXML([]byte(xmlWithAlgs), "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256", "http://www.w3.org/2001/04/xmlenc#sha256")
		if err != nil {
			t.Fatalf("EnrichMetadataXML() error: %v", err)
		}

		doc := etree.NewDocument()
		if err := doc.ReadFromBytes(enriched); err != nil {
			t.Fatalf("failed to parse enriched XML: %v", err)
		}

		sms := doc.FindElements("//IDPSSODescriptor/Extensions/alg:SigningMethod")
		if len(sms) != 1 {
			t.Errorf("expected 1 alg:SigningMethod, got %d", len(sms))
		}
		dms := doc.FindElements("//IDPSSODescriptor/Extensions/alg:DigestMethod")
		if len(dms) != 1 {
			t.Errorf("expected 1 alg:DigestMethod, got %d", len(dms))
		}
	})

	t.Run("leaves XML unchanged when empty URIs provided", func(t *testing.T) {
		t.Parallel()

		enriched, err := handler.EnrichMetadataXML([]byte(validMetadataXML), "", "")
		if err != nil {
			t.Fatalf("EnrichMetadataXML() error: %v", err)
		}
		if string(enriched) != validMetadataXML {
			t.Errorf("got %s, want %s", string(enriched), validMetadataXML)
		}
	})

	t.Run("injects explicit algorithm and digest methods", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			sigURI    string
			digestURI string
		}{
			{
				name:      "rsa-sha256",
				sigURI:    "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256",
				digestURI: "http://www.w3.org/2001/04/xmlenc#sha256",
			},
			{
				name:      "rsa-sha384",
				sigURI:    "http://www.w3.org/2001/04/xmldsig-more#rsa-sha384",
				digestURI: "http://www.w3.org/2001/04/xmldsig-more#sha384",
			},
			{
				name:      "rsa-sha512",
				sigURI:    "http://www.w3.org/2001/04/xmldsig-more#rsa-sha512",
				digestURI: "http://www.w3.org/2001/04/xmlenc#sha512",
			},
			{
				name:      "rsa-sha1",
				sigURI:    "http://www.w3.org/2000/09/xmldsig#rsa-sha1",
				digestURI: "http://www.w3.org/2000/09/xmldsig#sha1",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				enriched, err := handler.EnrichMetadataXML([]byte(validMetadataXML), tc.sigURI, tc.digestURI)
				if err != nil {
					t.Fatalf("EnrichMetadataXML() error: %v", err)
				}
				doc := etree.NewDocument()
				if err := doc.ReadFromBytes(enriched); err != nil {
					t.Fatalf("parse enriched XML error: %v", err)
				}
				sm := doc.FindElement("//IDPSSODescriptor/Extensions/alg:SigningMethod")
				if sm == nil {
					t.Fatal("missing alg:SigningMethod")
				}
				if got := sm.SelectAttrValue("Algorithm", ""); got != tc.sigURI {
					t.Errorf("signing Algorithm = %q, want %q", got, tc.sigURI)
				}
				dm := doc.FindElement("//IDPSSODescriptor/Extensions/alg:DigestMethod")
				if dm == nil {
					t.Fatal("missing alg:DigestMethod")
				}
				if got := dm.SelectAttrValue("Algorithm", ""); got != tc.digestURI {
					t.Errorf("digest Algorithm = %q, want %q", got, tc.digestURI)
				}
			})
		}
	})

	t.Run("returns raw XML when no IDPSSODescriptor is present", func(t *testing.T) {
		t.Parallel()

		noIDPXML := `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata"><SPSSODescriptor/></EntityDescriptor>`
		enriched, err := handler.EnrichMetadataXML([]byte(noIDPXML), "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256", "http://www.w3.org/2001/04/xmlenc#sha256")
		if err != nil {
			t.Fatalf("EnrichMetadataXML() error: %v", err)
		}
		if string(enriched) != noIDPXML {
			t.Errorf("got %s, want %s", string(enriched), noIDPXML)
		}
	})

	t.Run("returns error on invalid XML", func(t *testing.T) {
		t.Parallel()

		_, err := handler.EnrichMetadataXML([]byte("<invalid xml"), "", "")
		if err == nil {
			t.Fatal("expected error on invalid XML, got nil")
		}
	})

	t.Run("returns error on empty XML", func(t *testing.T) {
		t.Parallel()

		_, err := handler.EnrichMetadataXML([]byte(""), "", "")
		if err == nil {
			t.Fatal("expected error on empty XML, got nil")
		}
	})
}

func TestHandleMetadata(t *testing.T) {
	t.Parallel()

	ssoURL, _ := url.Parse("http://localhost:8082/saml/sso")
	metadataURL, _ := url.Parse("http://localhost:8082/saml/metadata")

	testIDP := &saml.IdentityProvider{
		Certificate:     &x509.Certificate{Raw: []byte("test-raw-cert")},
		MetadataURL:     *metadataURL,
		SSOURL:          *ssoURL,
		SignatureMethod: "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256",
	}

	t.Run("serves 200 OK with enriched metadata XML", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)

		sessions := mocks.NewMockSessionService(ctrl)
		sps := mocks.NewMockServiceProviderService(ctrl)
		mapping := mocks.NewMockMappingService(ctrl)
		oidc := mocks.NewMockOIDCService(ctrl)
		pending := mocks.NewMockPendingRequestService(ctrl)

		h := handler.NewHandlers(
			sessions, sps, mapping, oidc, pending, testIDP,
			handler.HandlerConfig{
				BridgeBaseURL:       "http://localhost:8082",
				SAMLSignatureMethod: "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256",
				SAMLDigestMethod:    "http://www.w3.org/2001/04/xmlenc#sha256",
			},
			logging.NewNopLogger(),
			monitoring.NewNoopMonitor(),
		)

		req := httptest.NewRequest(http.MethodGet, "/saml/metadata", nil)
		rec := httptest.NewRecorder()

		h.HandleMetadata(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		ct := rec.Header().Get("Content-Type")
		if ct != "application/samlmetadata+xml" {
			t.Errorf("Content-Type = %q, want application/samlmetadata+xml", ct)
		}

		body := rec.Body.String()
		if !strings.Contains(body, `<KeyDescriptor use="signing">`) {
			t.Errorf("body missing <KeyDescriptor use=\"signing\">:\n%s", body)
		}
		if !strings.Contains(body, `<alg:SigningMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"`) {
			t.Errorf("body missing alg:SigningMethod:\n%s", body)
		}
		if !strings.Contains(body, `<alg:DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256"`) {
			t.Errorf("body missing alg:DigestMethod:\n%s", body)
		}
	})

	t.Run("serves metadata with custom signature method from config", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)

		sessions := mocks.NewMockSessionService(ctrl)
		sps := mocks.NewMockServiceProviderService(ctrl)
		mapping := mocks.NewMockMappingService(ctrl)
		oidc := mocks.NewMockOIDCService(ctrl)
		pending := mocks.NewMockPendingRequestService(ctrl)

		h := handler.NewHandlers(
			sessions, sps, mapping, oidc, pending, testIDP,
			handler.HandlerConfig{
				BridgeBaseURL:       "http://localhost:8082",
				SAMLSignatureMethod: "http://www.w3.org/2001/04/xmldsig-more#rsa-sha512",
				SAMLDigestMethod:    "http://www.w3.org/2001/04/xmlenc#sha512",
			},
			logging.NewNopLogger(),
			monitoring.NewNoopMonitor(),
		)

		req := httptest.NewRequest(http.MethodGet, "/saml/metadata", nil)
		rec := httptest.NewRecorder()

		h.HandleMetadata(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "http://www.w3.org/2001/04/xmldsig-more#rsa-sha512") {
			t.Errorf("body missing sha512 signing method:\n%s", body)
		}
		if !strings.Contains(body, "http://www.w3.org/2001/04/xmlenc#sha512") {
			t.Errorf("body missing sha512 digest method:\n%s", body)
		}
	})

	t.Run("returns 500 when SAML IdP is nil", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)

		sessions := mocks.NewMockSessionService(ctrl)
		sps := mocks.NewMockServiceProviderService(ctrl)
		mapping := mocks.NewMockMappingService(ctrl)
		oidc := mocks.NewMockOIDCService(ctrl)
		pending := mocks.NewMockPendingRequestService(ctrl)

		h := handler.NewHandlers(
			sessions, sps, mapping, oidc, pending, nil,
			handler.HandlerConfig{BridgeBaseURL: "http://localhost:8082"},
			logging.NewNopLogger(),
			monitoring.NewNoopMonitor(),
		)

		req := httptest.NewRequest(http.MethodGet, "/saml/metadata", nil)
		rec := httptest.NewRecorder()

		h.HandleMetadata(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})
}
