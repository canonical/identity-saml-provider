package handler

import (
	"net/url"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/crewjam/saml"
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
func (r *RegisterSPRequest) Validate() error {
	if r.EntityID == "" {
		return &domain.ErrValidation{Field: "entity_id", Message: "required"}
	}
	if r.ACSURL == "" {
		return &domain.ErrValidation{Field: "acs_url", Message: "required"}
	}

	// Validate ACS URL format
	acsURL, err := url.Parse(r.ACSURL)
	if err != nil || acsURL.Scheme == "" || acsURL.Host == "" {
		return &domain.ErrValidation{Field: "acs_url", Message: "must be a valid URL with scheme and host"}
	}
	if acsURL.Scheme != "http" && acsURL.Scheme != "https" {
		return &domain.ErrValidation{Field: "acs_url", Message: "scheme must be http or https"}
	}

	// Validate binding
	validBindings := map[string]bool{
		saml.HTTPPostBinding:     true,
		saml.HTTPRedirectBinding: true,
	}
	if r.ACSBinding == "" {
		r.ACSBinding = saml.HTTPPostBinding // default
	} else if !validBindings[r.ACSBinding] {
		return &domain.ErrValidation{Field: "acs_binding", Message: "must be HTTP-POST or HTTP-Redirect"}
	}

	// Validate attribute mapping if present
	if r.AttributeMapping != nil {
		if err := r.AttributeMapping.Validate(); err != nil {
			return err
		}
	}

	return nil
}
