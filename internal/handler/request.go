package handler

import (
	"github.com/canonical/identity-saml-provider/internal/domain"
)

// RegisterSPRequest is the JSON DTO for service provider registration.
type RegisterSPRequest struct {
	EntityID         string                   `json:"entity_id"`
	ACSURL           string                   `json:"acs_url"`
	ACSBinding       string                   `json:"acs_binding"`
	AttributeMapping *domain.AttributeMapping `json:"attribute_mapping,omitempty"`
}

// Validate checks required fields, URL format, binding value, and
// optional attribute mapping. Returns *domain.ErrValidation on failure.
// Delegates to domain.ServiceProvider.Validate() for business rule checks.
func (r *RegisterSPRequest) Validate() error {
	sp := r.ToDomain()
	return sp.Validate()
}

// ToDomain converts the DTO to a domain.ServiceProvider.
func (r *RegisterSPRequest) ToDomain() *domain.ServiceProvider {
	return &domain.ServiceProvider{
		EntityID:         r.EntityID,
		ACSURL:           r.ACSURL,
		ACSBinding:       r.ACSBinding,
		AttributeMapping: r.AttributeMapping,
	}
}
