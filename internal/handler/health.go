// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package handler

import (
	"context"
	"net/http"
	"time"
)

//go:generate mockgen -destination=../../mocks/mock_pinger.go -package=mocks . Pinger

// Pinger abstracts the ability to check a dependency's connectivity.
// *pgxpool.Pool satisfies this interface.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler serves health and readiness probe endpoints.
// It is separate from Handlers to keep concerns isolated — health
// probes share no dependencies with the SAML/OIDC business logic.
type HealthHandler struct {
	pinger Pinger
}

// NewHealthHandler creates a HealthHandler with the given Pinger.
func NewHealthHandler(pinger Pinger) *HealthHandler {
	return &HealthHandler{pinger: pinger}
}

// readyzTimeout is the maximum time allowed for readiness dependency checks.
const readyzTimeout = 2 * time.Second

// HealthResponse is the JSON body for the /healthz endpoint.
type HealthResponse struct {
	Status string `json:"status"`
}

// ReadyResponse is the JSON body for the /readyz endpoint.
type ReadyResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// HandleHealthz is the liveness probe handler. It returns 200 OK
// unconditionally — if this handler can execute, the process is alive.
func (h *HealthHandler) HandleHealthz(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

// HandleReadyz is the readiness probe handler. It verifies that
// critical dependencies (PostgreSQL) are reachable before reporting
// the application as ready to serve traffic.
func (h *HealthHandler) HandleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), readyzTimeout)
	defer cancel()

	checks := make(map[string]string, 1)
	ready := true

	if err := h.pinger.Ping(ctx); err != nil {
		checks["postgres"] = "fail"
		ready = false
	} else {
		checks["postgres"] = "ok"
	}

	if ready {
		WriteJSON(w, http.StatusOK, ReadyResponse{Status: "ready"})
		return
	}

	WriteJSON(w, http.StatusServiceUnavailable, ReadyResponse{
		Status: "not ready",
		Checks: checks,
	})
}
