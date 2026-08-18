// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/handler"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/mocks"
	"go.uber.org/mock/gomock"
)

func newTestHandlers(
	t *testing.T,
	ctrl *gomock.Controller,
	opts ...func(*testHandlerDeps),
) (*handler.Handlers, *testHandlerDeps) {
	t.Helper()

	deps := &testHandlerDeps{
		sessions: mocks.NewMockSessionService(ctrl),
		sps:      mocks.NewMockServiceProviderService(ctrl),
		mapping:  mocks.NewMockMappingService(ctrl),
		oidc:     mocks.NewMockOIDCService(ctrl),
		pending:  mocks.NewMockPendingRequestService(ctrl),
	}
	for _, o := range opts {
		o(deps)
	}

	logger := logging.NewNopLogger()
	noopMonitor := &noopMon{}

	h := handler.NewHandlers(
		deps.sessions,
		deps.sps,
		deps.mapping,
		deps.oidc,
		deps.pending,
		nil, // samlIDP not needed for admin tests
		handler.HandlerConfig{BridgeBaseURL: "http://localhost:8082"},
		logger,
		noopMonitor,
	)
	return h, deps
}

type testHandlerDeps struct {
	sessions *mocks.MockSessionService
	sps      *mocks.MockServiceProviderService
	mapping  *mocks.MockMappingService
	oidc     *mocks.MockOIDCService
	pending  *mocks.MockPendingRequestService
}

// noopMon implements monitoring.MonitorInterface.
type noopMon struct{}

func (n *noopMon) ObserveHTTPRequestDuration(_, _, _ string, _ float64) {}
func (n *noopMon) IncrementHTTPRequestsTotal(_, _, _ string)            {}
func (n *noopMon) IncrementBridgeOperation(_, _ string)                 {}

func TestHandleRegisterServiceProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		setup      func(deps *testHandlerDeps)
		wantStatus int
		wantBody   func(t *testing.T, body []byte)
	}{
		{
			name: "success — 201",
			body: `{"entity_id":"https://sp.example.com","acs_url":"https://sp.example.com/acs"}`,
			setup: func(deps *testHandlerDeps) {
				deps.sps.EXPECT().Register(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantStatus: http.StatusCreated,
			wantBody: func(t *testing.T, body []byte) {
				t.Helper()
				var resp handler.RegisterSPResponse
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if resp.Status != "success" {
					t.Errorf("status = %q, want %q", resp.Status, "success")
				}
				if resp.EntityID != "https://sp.example.com" {
					t.Errorf("entity_id = %q, want %q", resp.EntityID, "https://sp.example.com")
				}
			},
		},
		{
			name:       "invalid JSON — 400",
			body:       `{`,
			setup:      func(deps *testHandlerDeps) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing entity_id — 400",
			body:       `{"acs_url":"https://sp.example.com/acs"}`,
			setup:      func(deps *testHandlerDeps) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing acs_url — 400",
			body:       `{"entity_id":"https://sp.example.com"}`,
			setup:      func(deps *testHandlerDeps) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid acs_url — 400",
			body:       `{"entity_id":"https://sp.example.com","acs_url":"not-a-url"}`,
			setup:      func(deps *testHandlerDeps) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "conflict — 409",
			body: `{"entity_id":"https://sp.example.com","acs_url":"https://sp.example.com/acs"}`,
			setup: func(deps *testHandlerDeps) {
				deps.sps.EXPECT().Register(gomock.Any(), gomock.Any()).
					Return(&domain.ErrConflict{Resource: "service_provider", ID: "https://sp.example.com"})
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "validation error from service — 400",
			body: `{"entity_id":"https://sp.example.com","acs_url":"https://sp.example.com/acs","attribute_mapping":{"nameid_format":"INVALID"}}`,
			setup: func(deps *testHandlerDeps) {
				// Validate() is called on the DTO before reaching the service
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			h, deps := newTestHandlers(t, ctrl)
			tt.setup(deps)

			req := httptest.NewRequest(http.MethodPost, "/admin/service-providers", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			h.HandleRegisterServiceProvider(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantBody != nil {
				tt.wantBody(t, rec.Body.Bytes())
			}
		})
	}
}

func TestHandleGetServiceProvider(t *testing.T) {
	t.Parallel()

	const entityID = "https://sp.example.com"

	mappedSP := &domain.ServiceProvider{
		EntityID:   entityID,
		ACSURL:     "https://sp.example.com/acs",
		ACSBinding: "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
		AttributeMapping: &domain.AttributeMapping{
			NameIDFormat: "persistent",
		},
	}
	unmappedSP := &domain.ServiceProvider{
		EntityID:   entityID,
		ACSURL:     "https://sp.example.com/acs",
		ACSBinding: "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
	}

	tests := []struct {
		name       string
		query      string
		setup      func(deps *testHandlerDeps)
		wantStatus int
		wantBody   func(t *testing.T, body []byte)
	}{
		{
			name:  "success — mapped SP includes attribute_mapping",
			query: "?entity_id=" + url.QueryEscape(entityID),
			setup: func(deps *testHandlerDeps) {
				deps.sps.EXPECT().GetByEntityID(gomock.Any(), entityID).Return(mappedSP, nil)
			},
			wantStatus: http.StatusOK,
			wantBody: func(t *testing.T, body []byte) {
				t.Helper()
				var resp handler.GetServiceProviderResponse
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if resp.EntityID != entityID {
					t.Errorf("entity_id = %q, want %q", resp.EntityID, entityID)
				}
				if resp.AttributeMapping == nil {
					t.Errorf("attribute_mapping is nil, want non-nil")
				}
				var raw map[string]json.RawMessage
				if err := json.Unmarshal(body, &raw); err != nil {
					t.Fatalf("raw decode: %v", err)
				}
				if _, ok := raw["attribute_mapping"]; !ok {
					t.Errorf("attribute_mapping field absent from JSON, want present")
				}
			},
		},
		{
			name:  "success — unmapped SP omits attribute_mapping",
			query: "?entity_id=" + url.QueryEscape(entityID),
			setup: func(deps *testHandlerDeps) {
				deps.sps.EXPECT().GetByEntityID(gomock.Any(), entityID).Return(unmappedSP, nil)
			},
			wantStatus: http.StatusOK,
			wantBody: func(t *testing.T, body []byte) {
				t.Helper()
				var raw map[string]json.RawMessage
				if err := json.Unmarshal(body, &raw); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if _, ok := raw["attribute_mapping"]; ok {
					t.Errorf("attribute_mapping present, want omitted (raw=%s)", string(body))
				}
				if _, ok := raw["entity_id"]; !ok {
					t.Errorf("entity_id missing, want present")
				}
			},
		},
		{
			name:       "missing entity_id — 400",
			query:      "",
			setup:      func(deps *testHandlerDeps) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty entity_id — 400",
			query:      "?entity_id=",
			setup:      func(deps *testHandlerDeps) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:  "not found — 404",
			query: "?entity_id=" + url.QueryEscape(entityID),
			setup: func(deps *testHandlerDeps) {
				deps.sps.EXPECT().GetByEntityID(gomock.Any(), entityID).
					Return(nil, &domain.ErrNotFound{Resource: "service_provider", ID: entityID})
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:  "internal error — 500 without leaking",
			query: "?entity_id=" + url.QueryEscape(entityID),
			setup: func(deps *testHandlerDeps) {
				deps.sps.EXPECT().GetByEntityID(gomock.Any(), entityID).
					Return(nil, errors.New("database: connection refused on host db-internal:5432"))
			},
			wantStatus: http.StatusInternalServerError,
			wantBody: func(t *testing.T, body []byte) {
				t.Helper()
				if strings.Contains(string(body), "database:") ||
					strings.Contains(string(body), "db-internal") {
					t.Errorf("response body leaks internals: %s", string(body))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			h, deps := newTestHandlers(t, ctrl)
			tt.setup(deps)

			req := httptest.NewRequest(http.MethodGet, "/admin/service-providers"+tt.query, nil)
			rec := httptest.NewRecorder()
			h.HandleGetServiceProvider(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantBody != nil {
				tt.wantBody(t, rec.Body.Bytes())
			}
		})
	}
}

func TestHandleUpdateAttributeMapping(t *testing.T) {
	t.Parallel()

	const entityID = "https://sp.example.com"
	validBody := `{"nameid_format":"persistent","saml_attribute_mappings":{"email":{"name":"mail"}}}`

	tests := []struct {
		name       string
		query      string
		body       string
		setup      func(deps *testHandlerDeps)
		wantStatus int
		wantBody   func(t *testing.T, body []byte)
	}{
		{
			name:  "success — 200 and round-trips through the service",
			query: "?entity_id=" + url.QueryEscape(entityID),
			body:  validBody,
			setup: func(deps *testHandlerDeps) {
				deps.sps.EXPECT().
					UpdateAttributeMapping(gomock.Any(), entityID, gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, m *domain.AttributeMapping) error {
						if m == nil {
							t.Errorf("service received nil mapping, want non-nil")
							return nil
						}
						if m.NameIDFormat != "persistent" {
							t.Errorf("NameIDFormat = %q, want %q", m.NameIDFormat, "persistent")
						}
						if def, ok := m.SAMLAttributeMappings["email"]; !ok || def.Name != "mail" {
							t.Errorf("SAMLAttributeMappings[email] = %+v, want {Name:mail}", def)
						}
						return nil
					})
			},
			wantStatus: http.StatusOK,
			wantBody: func(t *testing.T, body []byte) {
				t.Helper()
				var resp map[string]string
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if resp["status"] != "success" || resp["message"] != "Attribute mapping updated" {
					t.Errorf("body = %s, want success envelope", string(body))
				}
			},
		},
		{
			name:       "invalid JSON — 400",
			query:      "?entity_id=" + url.QueryEscape(entityID),
			body:       `{`,
			setup:      func(deps *testHandlerDeps) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:  "invalid mapping — 400 with field path",
			query: "?entity_id=" + url.QueryEscape(entityID),
			body:  `{"nameid_format":"foobar"}`,
			setup: func(deps *testHandlerDeps) {
				deps.sps.EXPECT().
					UpdateAttributeMapping(gomock.Any(), entityID, gomock.Any()).
					Return(&domain.ErrValidation{
						Field:   "nameid_format",
						Message: "unsupported value \"foobar\"",
					})
			},
			wantStatus: http.StatusBadRequest,
			wantBody: func(t *testing.T, body []byte) {
				t.Helper()
				if !strings.Contains(string(body), "nameid_format") {
					t.Errorf("body = %s, want it to identify nameid_format", string(body))
				}
			},
		},
		{
			name:  "invalid mapping — empty SAML attribute name — 400 with field path",
			query: "?entity_id=" + url.QueryEscape(entityID),
			body:  `{"saml_attribute_mappings":{"email":{"name":""}}}`,
			setup: func(deps *testHandlerDeps) {
				deps.sps.EXPECT().
					UpdateAttributeMapping(gomock.Any(), entityID, gomock.Any()).
					Return(&domain.ErrValidation{
						Field:   "saml_attribute_mappings.email.name",
						Message: "must not be empty",
					})
			},
			wantStatus: http.StatusBadRequest,
			wantBody: func(t *testing.T, body []byte) {
				t.Helper()
				if !strings.Contains(string(body), "saml_attribute_mappings.email.name") {
					t.Errorf("body = %s, want it to identify the field path", string(body))
				}
			},
		},
		{
			name:       "missing entity_id — 400",
			query:      "",
			body:       validBody,
			setup:      func(deps *testHandlerDeps) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:  "not found — 404",
			query: "?entity_id=" + url.QueryEscape(entityID),
			body:  validBody,
			setup: func(deps *testHandlerDeps) {
				deps.sps.EXPECT().
					UpdateAttributeMapping(gomock.Any(), entityID, gomock.Any()).
					Return(&domain.ErrNotFound{Resource: "service_provider", ID: entityID})
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:  "unexpected service error — 500 with sanitized envelope",
			query: "?entity_id=" + url.QueryEscape(entityID),
			body:  validBody,
			setup: func(deps *testHandlerDeps) {
				deps.sps.EXPECT().
					UpdateAttributeMapping(gomock.Any(), entityID, gomock.Any()).
					Return(errors.New("boom: secret internal detail"))
			},
			wantStatus: http.StatusInternalServerError,
			wantBody: func(t *testing.T, body []byte) {
				t.Helper()
				var resp handler.APIError
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if resp.Status != http.StatusInternalServerError || resp.Message != "internal error" {
					t.Errorf("body = %s, want sanitized 500 envelope", string(body))
				}
				if strings.Contains(string(body), "boom") || strings.Contains(string(body), "secret internal detail") {
					t.Errorf("body leaks internal error details: %s", string(body))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			h, deps := newTestHandlers(t, ctrl)
			tt.setup(deps)

			req := httptest.NewRequest(http.MethodPut,
				"/admin/service-providers/attribute-mapping"+tt.query,
				strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.HandleUpdateAttributeMapping(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantBody != nil {
				tt.wantBody(t, rec.Body.Bytes())
			}
		})
	}
}

func TestHandleDeleteAttributeMapping(t *testing.T) {
	t.Parallel()

	const entityID = "https://sp.example.com"

	tests := []struct {
		name       string
		query      string
		setup      func(deps *testHandlerDeps)
		wantStatus int
		wantBody   func(t *testing.T, body []byte)
	}{
		{
			name:  "success — 200",
			query: "?entity_id=" + url.QueryEscape(entityID),
			setup: func(deps *testHandlerDeps) {
				deps.sps.EXPECT().ClearAttributeMapping(gomock.Any(), entityID).Return(nil)
			},
			wantStatus: http.StatusOK,
			wantBody: func(t *testing.T, body []byte) {
				t.Helper()
				var resp map[string]string
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if resp["status"] != "success" || resp["message"] != "Attribute mapping cleared" {
					t.Errorf("body = %s, want success envelope", string(body))
				}
			},
		},
		{
			name:  "idempotent — second call still returns 200",
			query: "?entity_id=" + url.QueryEscape(entityID),
			setup: func(deps *testHandlerDeps) {
				// The service-layer round-trips through GetByEntityID +
				// UpdateAttributeMapping(nil) regardless of whether
				// there is currently a mapping. The handler simply
				// surfaces whatever the service returns. Calling clear
				// on an unmapped SP still succeeds with 200.
				deps.sps.EXPECT().ClearAttributeMapping(gomock.Any(), entityID).Return(nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing entity_id — 400",
			query:      "",
			setup:      func(deps *testHandlerDeps) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:  "not found — 404",
			query: "?entity_id=" + url.QueryEscape(entityID),
			setup: func(deps *testHandlerDeps) {
				deps.sps.EXPECT().ClearAttributeMapping(gomock.Any(), entityID).
					Return(&domain.ErrNotFound{Resource: "service_provider", ID: entityID})
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:  "unexpected service error — 500 with sanitized envelope",
			query: "?entity_id=" + url.QueryEscape(entityID),
			setup: func(deps *testHandlerDeps) {
				deps.sps.EXPECT().ClearAttributeMapping(gomock.Any(), entityID).
					Return(errors.New("boom: secret internal detail"))
			},
			wantStatus: http.StatusInternalServerError,
			wantBody: func(t *testing.T, body []byte) {
				t.Helper()
				var resp handler.APIError
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if resp.Status != http.StatusInternalServerError || resp.Message != "internal error" {
					t.Errorf("body = %s, want sanitized 500 envelope", string(body))
				}
				if strings.Contains(string(body), "boom") || strings.Contains(string(body), "secret internal detail") {
					t.Errorf("body leaks internal error details: %s", string(body))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			h, deps := newTestHandlers(t, ctrl)
			tt.setup(deps)

			req := httptest.NewRequest(http.MethodDelete,
				"/admin/service-providers/attribute-mapping"+tt.query, nil)
			rec := httptest.NewRecorder()
			h.HandleDeleteAttributeMapping(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantBody != nil {
				tt.wantBody(t, rec.Body.Bytes())
			}
		})
	}
}

// TestPutEmptyObjectVsDelete encodes the spec contract that
// `PUT /admin/service-providers/attribute-mapping` with body `{}` is
// a configured-empty mapping (UpdateAttributeMapping called with a
// non-nil zero-value mapping) and is NOT equivalent to DELETE
// (ClearAttributeMapping called with no UpdateAttributeMapping
// invocation).
func TestPutEmptyObjectVsDelete(t *testing.T) {
	t.Parallel()

	const entityID = "https://sp.example.com"

	t.Run("PUT {} sends non-nil zero-value mapping to UpdateAttributeMapping", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		h, deps := newTestHandlers(t, ctrl)

		deps.sps.EXPECT().
			UpdateAttributeMapping(gomock.Any(), entityID, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, m *domain.AttributeMapping) error {
				if m == nil {
					t.Fatalf("UpdateAttributeMapping received nil mapping; want non-nil zero-value")
					return nil
				}
				if m.NameIDFormat != "" {
					t.Errorf("NameIDFormat = %q, want empty zero value", m.NameIDFormat)
				}
				if len(m.SAMLAttributeMappings) != 0 {
					t.Errorf("SAMLAttributeMappings len = %d, want 0", len(m.SAMLAttributeMappings))
				}
				if len(m.OIDCClaimMappings) != 0 {
					t.Errorf("OIDCClaimMappings len = %d, want 0", len(m.OIDCClaimMappings))
				}
				if m.Options != (domain.MappingOptions{}) {
					t.Errorf("Options = %+v, want zero value", m.Options)
				}
				return nil
			})
		// ClearAttributeMapping must not be called.
		// (Unset EXPECT defaults to zero calls under gomock.Controller.)

		req := httptest.NewRequest(http.MethodPut,
			"/admin/service-providers/attribute-mapping?entity_id="+url.QueryEscape(entityID),
			strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.HandleUpdateAttributeMapping(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("DELETE calls ClearAttributeMapping and never UpdateAttributeMapping", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		h, deps := newTestHandlers(t, ctrl)

		deps.sps.EXPECT().ClearAttributeMapping(gomock.Any(), entityID).Return(nil)
		// UpdateAttributeMapping must not be called; unset EXPECT means
		// zero invocations expected under the controller.

		req := httptest.NewRequest(http.MethodDelete,
			"/admin/service-providers/attribute-mapping?entity_id="+url.QueryEscape(entityID),
			nil)
		rec := httptest.NewRecorder()
		h.HandleDeleteAttributeMapping(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
		}
	})
}
