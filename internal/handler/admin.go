// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package handler

import (
	"encoding/json"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// HandleRegisterServiceProvider handles POST /admin/service-providers.
func (h *Handlers) HandleRegisterServiceProvider(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	var req RegisterSPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid JSON")
		WriteJSON(w, http.StatusBadRequest, APIError{Status: http.StatusBadRequest, Message: "invalid JSON"})
		return
	}

	span.SetAttributes(
		attribute.String("handler.admin.entity_id", req.EntityID),
	)

	sp := req.ToDomain()

	if err := sp.Validate(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "validation failed")
		WriteError(w, err)
		return
	}

	if err := h.serviceProviders.Register(ctx, sp); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "registration failed")
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, RegisterSPResponse{
		Status:   "success",
		Message:  "Service provider registered",
		EntityID: req.EntityID,
	})
}
