package handler

import (
	"encoding/json"
	"net/http"

	"github.com/canonical/identity-saml-provider/internal/domain"
)

// HandleRegisterServiceProvider handles POST /admin/service-providers.
func (h *Handlers) HandleRegisterServiceProvider(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "handler.register_service_provider")
	defer span.End()

	var req RegisterSPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, APIError{Status: http.StatusBadRequest, Message: "invalid JSON"})
		return
	}

	if err := req.Validate(); err != nil {
		WriteError(w, err)
		return
	}

	sp := &domain.ServiceProvider{
		EntityID:         req.EntityID,
		ACSURL:           req.ACSURL,
		ACSBinding:       req.ACSBinding,
		AttributeMapping: req.AttributeMapping,
	}

	if err := h.serviceProviders.Register(ctx, sp); err != nil {
		h.logger.Errorw("Failed to register SP", "entityID", req.EntityID, "error", err)
		WriteError(w, err)
		return
	}

	h.logger.Infow("Service provider registered", "entityID", req.EntityID)
	WriteJSON(w, http.StatusCreated, RegisterSPResponse{
		Status:   "success",
		Message:  "Service provider registered",
		EntityID: req.EntityID,
	})
}
