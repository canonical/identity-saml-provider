package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/canonical/identity-saml-provider/internal/domain"
)

// APIError is the standard JSON error response body.
type APIError struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// RegisterSPResponse is returned on successful service provider registration.
type RegisterSPResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	EntityID string `json:"entity_id"`
}

// WriteJSON encodes v as JSON and writes it to w with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError maps domain errors to HTTP status codes and writes a JSON error response.
func WriteError(w http.ResponseWriter, err error) {
	var notFound *domain.ErrNotFound
	var validation *domain.ErrValidation
	var conflict *domain.ErrConflict
	var authErr *domain.ErrAuthentication
	var upstream *domain.ErrUpstream

	switch {
	case errors.As(err, &notFound):
		WriteJSON(w, http.StatusNotFound, APIError{Status: http.StatusNotFound, Message: notFound.Error()})
	case errors.As(err, &validation):
		WriteJSON(w, http.StatusBadRequest, APIError{Status: http.StatusBadRequest, Message: validation.Error()})
	case errors.As(err, &conflict):
		WriteJSON(w, http.StatusConflict, APIError{Status: http.StatusConflict, Message: conflict.Error()})
	case errors.As(err, &authErr):
		WriteJSON(w, http.StatusForbidden, APIError{Status: http.StatusForbidden, Message: authErr.Error()})
	case errors.As(err, &upstream):
		WriteJSON(w, http.StatusBadGateway, APIError{Status: http.StatusBadGateway, Message: "upstream service error"})
	default:
		WriteJSON(w, http.StatusInternalServerError, APIError{Status: http.StatusInternalServerError, Message: "internal error"})
	}
}
