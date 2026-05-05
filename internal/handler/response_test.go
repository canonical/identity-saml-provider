package handler_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/handler"
)

func TestWriteError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		err            error
		wantStatusCode int
		wantMessage    string
	}{
		{
			name:           "ErrNotFound → 404",
			err:            &domain.ErrNotFound{Resource: "session", ID: "abc"},
			wantStatusCode: http.StatusNotFound,
			wantMessage:    "session not found: abc",
		},
		{
			name:           "ErrValidation → 400",
			err:            &domain.ErrValidation{Field: "email", Message: "required"},
			wantStatusCode: http.StatusBadRequest,
			wantMessage:    "validation error on email: required",
		},
		{
			name:           "ErrConflict → 409",
			err:            &domain.ErrConflict{Resource: "service_provider", ID: "sp1"},
			wantStatusCode: http.StatusConflict,
			wantMessage:    "service_provider already exists: sp1",
		},
		{
			name:           "ErrAuthentication → 403",
			err:            &domain.ErrAuthentication{Reason: "invalid token"},
			wantStatusCode: http.StatusForbidden,
			wantMessage:    "authentication failed: invalid token",
		},
		{
			name:           "ErrUpstream → 502",
			err:            &domain.ErrUpstream{Service: "hydra", Err: errors.New("connection refused")},
			wantStatusCode: http.StatusBadGateway,
			wantMessage:    "upstream service error",
		},
		{
			name:           "wrapped ErrValidation → 400",
			err:            fmt.Errorf("register failed: %w", &domain.ErrValidation{Field: "acs_url", Message: "invalid"}),
			wantStatusCode: http.StatusBadRequest,
			wantMessage:    "validation error on acs_url: invalid",
		},
		{
			name:           "generic error → 500",
			err:            errors.New("something went wrong"),
			wantStatusCode: http.StatusInternalServerError,
			wantMessage:    "internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler.WriteError(rec, tt.err)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatusCode)
			}

			var apiErr handler.APIError
			if err := json.NewDecoder(rec.Body).Decode(&apiErr); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if apiErr.Status != tt.wantStatusCode {
				t.Errorf("body status = %d, want %d", apiErr.Status, tt.wantStatusCode)
			}
			if apiErr.Message != tt.wantMessage {
				t.Errorf("message = %q, want %q", apiErr.Message, tt.wantMessage)
			}
		})
	}
}
