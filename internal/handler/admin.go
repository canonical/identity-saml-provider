// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package handler

import (
	"encoding/json"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
)

// GetServiceProviderResponse is the JSON response body for
// GET /admin/service-providers. AttributeMapping is omitted when the
// SP has no mapping configured (the JSONB column is NULL).
type GetServiceProviderResponse struct {
	EntityID         string                   `json:"entity_id"`
	ACSURL           string                   `json:"acs_url"`
	ACSBinding       string                   `json:"acs_binding"`
	AttributeMapping *domain.AttributeMapping `json:"attribute_mapping,omitempty"`
}

func spToResponse(sp *domain.ServiceProvider) *GetServiceProviderResponse {
	return &GetServiceProviderResponse{
		EntityID:         sp.EntityID,
		ACSURL:           sp.ACSURL,
		ACSBinding:       sp.ACSBinding,
		AttributeMapping: sp.AttributeMapping,
	}
}

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

// HandleGetServiceProvider handles GET /admin/service-providers,
// returning the SP identified by the `entity_id` query parameter.
func (h *Handlers) HandleGetServiceProvider(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)
	logger := logging.FromContext(ctx, h.logger)

	entityID := r.URL.Query().Get("entity_id")
	if entityID == "" {
		WriteJSON(w, http.StatusBadRequest, APIError{
			Status:  http.StatusBadRequest,
			Message: "entity_id query parameter is required",
		})
		return
	}

	span.SetAttributes(attribute.String("handler.admin.entity_id", entityID))

	sp, err := h.serviceProviders.GetByEntityID(ctx, entityID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "get service provider failed")
		logger.Errorw("Failed to get service provider", "entityID", entityID, "error", err)
		WriteError(w, err)
		return
	}

	logger.Infow("Service provider retrieved", "entityID", entityID, "operation", "get")
	WriteJSON(w, http.StatusOK, spToResponse(sp))
}

// HandleUpdateAttributeMapping handles
// PUT /admin/service-providers/attribute-mapping. It fully replaces
// the addressed SP's attribute mapping with the JSON body.
func (h *Handlers) HandleUpdateAttributeMapping(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)
	logger := logging.FromContext(ctx, h.logger)

	entityID := r.URL.Query().Get("entity_id")
	if entityID == "" {
		WriteJSON(w, http.StatusBadRequest, APIError{
			Status:  http.StatusBadRequest,
			Message: "entity_id query parameter is required",
		})
		return
	}

	span.SetAttributes(attribute.String("handler.admin.entity_id", entityID))

	var mapping domain.AttributeMapping
	if err := json.NewDecoder(r.Body).Decode(&mapping); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid JSON")
		WriteJSON(w, http.StatusBadRequest, APIError{
			Status:  http.StatusBadRequest,
			Message: "invalid JSON",
		})
		return
	}

	if err := h.serviceProviders.UpdateAttributeMapping(ctx, entityID, &mapping); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "update attribute mapping failed")
		logger.Errorw("Failed to update attribute mapping", "entityID", entityID, "error", err)
		WriteError(w, err)
		return
	}

	logger.Infow("Attribute mapping updated", "entityID", entityID, "operation", "update")
	WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Attribute mapping updated",
	})
}

// HandleDeleteAttributeMapping handles
// DELETE /admin/service-providers/attribute-mapping. It clears the
// SP's attribute mapping, reverting it to unmapped behaviour.
func (h *Handlers) HandleDeleteAttributeMapping(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)
	logger := logging.FromContext(ctx, h.logger)

	entityID := r.URL.Query().Get("entity_id")
	if entityID == "" {
		WriteJSON(w, http.StatusBadRequest, APIError{
			Status:  http.StatusBadRequest,
			Message: "entity_id query parameter is required",
		})
		return
	}

	span.SetAttributes(attribute.String("handler.admin.entity_id", entityID))

	if err := h.serviceProviders.ClearAttributeMapping(ctx, entityID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "clear attribute mapping failed")
		logger.Errorw("Failed to clear attribute mapping", "entityID", entityID, "error", err)
		WriteError(w, err)
		return
	}

	logger.Infow("Attribute mapping cleared", "entityID", entityID, "operation", "clear")
	WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Attribute mapping cleared",
	})
}
