// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/handler"
)

// stubPinger is a test double for handler.Pinger.
type stubPinger struct {
	err error
}

func (s *stubPinger) Ping(context.Context) error {
	return s.err
}

func TestHandleHealthz(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		wantStatus int
		wantBody   handler.HealthResponse
	}{
		{
			name:       "returns 200 OK",
			wantStatus: http.StatusOK,
			wantBody:   handler.HealthResponse{Status: "ok"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := handler.NewHealthHandler(&stubPinger{})
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

			h.HandleHealthz(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want %q", ct, "application/json")
			}

			var got handler.HealthResponse
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if got != tt.wantBody {
				t.Errorf("body = %+v, want %+v", got, tt.wantBody)
			}
		})
	}
}

func TestHandleReadyz(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pingErr    error
		wantStatus int
		wantBody   handler.ReadyResponse
	}{
		{
			name:       "database healthy",
			pingErr:    nil,
			wantStatus: http.StatusOK,
			wantBody:   handler.ReadyResponse{Status: "ready"},
		},
		{
			name:       "database unreachable",
			pingErr:    errors.New("connection refused"),
			wantStatus: http.StatusServiceUnavailable,
			wantBody: handler.ReadyResponse{
				Status: "not ready",
				Checks: map[string]string{"postgres": "fail"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := handler.NewHealthHandler(&stubPinger{err: tt.pingErr})
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

			h.HandleReadyz(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want %q", ct, "application/json")
			}

			var got handler.ReadyResponse
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if got.Status != tt.wantBody.Status {
				t.Errorf("status = %q, want %q", got.Status, tt.wantBody.Status)
			}
			if len(tt.wantBody.Checks) > 0 {
				for k, want := range tt.wantBody.Checks {
					if gotV, ok := got.Checks[k]; !ok {
						t.Errorf("missing check %q", k)
					} else if gotV != want {
						t.Errorf("checks[%q] = %q, want %q", k, gotV, want)
					}
				}
			}
		})
	}
}
