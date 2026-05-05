package handler_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/handler"
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

func TestSAMLSessionAdapter_GetSession_SessionExists(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockSessions := mocks.NewMockSessionService(ctrl)
	mockMapping := mocks.NewMockMappingService(ctrl)
	mockPending := mocks.NewMockPendingRequestService(ctrl)
	mockOIDC := mocks.NewMockOIDCService(ctrl)
	logger := zap.NewNop().Sugar()

	session := &domain.Session{
		ID:             "session-123",
		CreateTime:     time.Now(),
		ExpireTime:     time.Now().Add(10 * time.Minute),
		Index:          "session-123",
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "Test User",
	}

	mockSessions.EXPECT().GetByID(gomock.Any(), "session-123").Return(session, nil)
	mockMapping.EXPECT().ApplyMapping(gomock.Any(), session, "https://sp.example.com").Return(session, nil)

	adapter := &handler.SAMLSessionAdapter{
		Sessions: mockSessions,
		Mapping:  mockMapping,
		Pending:  mockPending,
		OIDC:     mockOIDC,
		Config:   handler.HandlerConfig{BridgeBaseURL: "http://localhost:8082"},
		Logger:   logger,
	}

	req := httptest.NewRequest(http.MethodGet, "/saml/sso?SAMLRequest=test", nil)
	req.AddCookie(&http.Cookie{Name: "saml_session", Value: "session-123"})
	rec := httptest.NewRecorder()

	authnReq := &saml.IdpAuthnRequest{
		Request: saml.AuthnRequest{
			ID: "req-1",
			Issuer: &saml.Issuer{
				Value: "https://sp.example.com",
			},
		},
	}

	result := adapter.GetSession(rec, req, authnReq)

	if result == nil {
		t.Fatal("expected non-nil session")
	}
	if result.ID != "session-123" {
		t.Errorf("ID = %q, want %q", result.ID, "session-123")
	}
	if result.NameID != "user@example.com" {
		t.Errorf("NameID = %q", result.NameID)
	}
	if rec.Code == http.StatusFound {
		t.Error("should not redirect when session exists")
	}
}

func TestSAMLSessionAdapter_GetSession_NoSession_RedirectsToOIDC(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockSessions := mocks.NewMockSessionService(ctrl)
	mockMapping := mocks.NewMockMappingService(ctrl)
	mockPending := mocks.NewMockPendingRequestService(ctrl)
	mockOIDC := mocks.NewMockOIDCService(ctrl)
	logger := zap.NewNop().Sugar()

	mockPending.EXPECT().Store(gomock.Any(), gomock.Any()).Return(nil)
	mockOIDC.EXPECT().AuthCodeURL("req-42:my-relay").Return("https://hydra.example.com/auth?state=req-42:my-relay")

	adapter := &handler.SAMLSessionAdapter{
		Sessions: mockSessions,
		Mapping:  mockMapping,
		Pending:  mockPending,
		OIDC:     mockOIDC,
		Config:   handler.HandlerConfig{BridgeBaseURL: "http://localhost:8082"},
		Logger:   logger,
	}

	req := httptest.NewRequest(http.MethodGet, "/saml/sso?SAMLRequest=encoded-request", nil)
	rec := httptest.NewRecorder()

	authnReq := &saml.IdpAuthnRequest{
		Request: saml.AuthnRequest{
			ID: "req-42",
		},
		RelayState: "my-relay",
	}

	result := adapter.GetSession(rec, req, authnReq)

	if result != nil {
		t.Error("expected nil session when unauthenticated")
	}
	if rec.Code != http.StatusFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	loc := rec.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header")
	}
	if !containsSubstr(loc, "hydra.example.com/auth") {
		t.Errorf("Location = %q, want OIDC URL", loc)
	}
}

func TestSAMLSessionAdapter_GetSession_SessionNotFound_RedirectsToOIDC(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockSessions := mocks.NewMockSessionService(ctrl)
	mockMapping := mocks.NewMockMappingService(ctrl)
	mockPending := mocks.NewMockPendingRequestService(ctrl)
	mockOIDC := mocks.NewMockOIDCService(ctrl)
	logger := zap.NewNop().Sugar()

	// Session cookie exists but session not found in repo
	mockSessions.EXPECT().GetByID(gomock.Any(), "gone-session").
		Return(nil, &domain.ErrNotFound{Resource: "session", ID: "gone-session"})

	mockPending.EXPECT().Store(gomock.Any(), gomock.Any()).Return(nil)
	mockOIDC.EXPECT().AuthCodeURL(gomock.Any()).Return("https://hydra.example.com/auth")

	adapter := &handler.SAMLSessionAdapter{
		Sessions: mockSessions,
		Mapping:  mockMapping,
		Pending:  mockPending,
		OIDC:     mockOIDC,
		Config:   handler.HandlerConfig{BridgeBaseURL: "http://localhost:8082"},
		Logger:   logger,
	}

	req := httptest.NewRequest(http.MethodGet, "/saml/sso?SAMLRequest=encoded-request", nil)
	req.AddCookie(&http.Cookie{Name: "saml_session", Value: "gone-session"})
	rec := httptest.NewRecorder()

	authnReq := &saml.IdpAuthnRequest{
		Request: saml.AuthnRequest{ID: "req-99"},
	}

	result := adapter.GetSession(rec, req, authnReq)

	if result != nil {
		t.Error("expected nil session for not-found session")
	}
	if rec.Code != http.StatusFound {
		t.Errorf("status = %d, want redirect", rec.Code)
	}
}
