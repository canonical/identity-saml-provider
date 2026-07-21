// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package monitoring_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/monitoring"
	"github.com/canonical/identity-saml-provider/mocks"
	"github.com/go-chi/chi/v5"
	"go.uber.org/mock/gomock"
)

func TestMetricsMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		method         string
		reqPath        string
		setupRouter    func(r chi.Router, mdw *monitoring.Middleware)
		setupMock      func(mon *mocks.MockMonitorInterface)
		expectedStatus int
	}{
		{
			name:    "records successful GET requests",
			method:  http.MethodGet,
			reqPath: "/test",
			setupRouter: func(r chi.Router, mdw *monitoring.Middleware) {
				r.Use(mdw.Metrics())
				r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
			},
			setupMock: func(mon *mocks.MockMonitorInterface) {
				mon.EXPECT().ObserveHTTPRequestDuration(http.MethodGet, "/test", "200", gomock.Any())
				mon.EXPECT().IncrementHTTPRequestsTotal(http.MethodGet, "/test", "200")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:    "records unsuccessful POST requests and handles route variables",
			method:  http.MethodPost,
			reqPath: "/items/123",
			setupRouter: func(r chi.Router, mdw *monitoring.Middleware) {
				r.Use(mdw.Metrics())
				r.Post("/items/{id}", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
				})
			},
			setupMock: func(mon *mocks.MockMonitorInterface) {
				mon.EXPECT().ObserveHTTPRequestDuration(http.MethodPost, "/items/{id}", "400", gomock.Any())
				mon.EXPECT().IncrementHTTPRequestsTotal(http.MethodPost, "/items/{id}", "400")
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:    "falls back to raw path when route context is missing",
			method:  http.MethodGet,
			reqPath: "/raw/path",
			setupRouter: func(r chi.Router, mdw *monitoring.Middleware) {
				// We call the middleware directly, bypassing the Chi router context
				r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
					handler := mdw.Metrics()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
					}))
					// Remove route context to test the fallback
					r = r.WithContext(context.Background())
					handler.ServeHTTP(w, r)
				})
			},
			setupMock: func(mon *mocks.MockMonitorInterface) {
				mon.EXPECT().ObserveHTTPRequestDuration(http.MethodGet, "/raw/path", "200", gomock.Any())
				mon.EXPECT().IncrementHTTPRequestsTotal(http.MethodGet, "/raw/path", "200")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:    "does not panic when monitor is nil",
			method:  http.MethodGet,
			reqPath: "/test",
			setupRouter: func(r chi.Router, mdw *monitoring.Middleware) {
				r.Use(mdw.Metrics())
				r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
			},
			setupMock:      func(mon *mocks.MockMonitorInterface) {}, // No expectations since monitor is nil
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			var mon *mocks.MockMonitorInterface
			var mdw *monitoring.Middleware

			if tt.name == "does not panic when monitor is nil" {
				mdw = monitoring.NewMiddleware(nil)
			} else {
				mon = mocks.NewMockMonitorInterface(ctrl)
				tt.setupMock(mon)
				mdw = monitoring.NewMiddleware(mon)
			}

			r := chi.NewRouter()
			tt.setupRouter(r, mdw)

			req := httptest.NewRequest(tt.method, tt.reqPath, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}
