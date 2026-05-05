package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/handler"
	"github.com/canonical/identity-saml-provider/internal/tracing"
	"github.com/canonical/identity-saml-provider/mocks"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
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

	logger := zap.NewNop().Sugar()
	tracer := tracing.NewNoopTracer()
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
		tracer,
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

func (n *noopMon) GetService() string                                         { return "test" }
func (n *noopMon) SetResponseTimeMetric(map[string]string, float64) error     { return nil }
func (n *noopMon) SetDependencyAvailability(map[string]string, float64) error { return nil }

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
