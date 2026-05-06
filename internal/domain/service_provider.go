package domain

import (
	"net/url"
)

// ServiceProvider represents a registered SAML Service Provider.
type ServiceProvider struct {
	EntityID         string
	ACSURL           string
	ACSBinding       string
	AttributeMapping *AttributeMapping // per-SP attribute mapping config (nil = use defaults)
}

// defaultACSBinding is the default ACS binding type for SAML service providers.
const defaultACSBinding = "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"

// validBindings enumerates the supported SAML ACS binding types.
var validBindings = map[string]bool{
	"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST":     true,
	"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect": true,
}

// Validate checks required fields, URL format, binding value, and
// optional attribute mapping. Returns *ErrValidation on failure.
// It also applies defaults (e.g. ACS binding) when appropriate.
func (sp *ServiceProvider) Validate() error {
	if sp.EntityID == "" {
		return &ErrValidation{Field: "entity_id", Message: "required"}
	}
	if sp.ACSURL == "" {
		return &ErrValidation{Field: "acs_url", Message: "required"}
	}

	// Validate ACS URL format.
	acsURL, err := url.Parse(sp.ACSURL)
	if err != nil || acsURL.Scheme == "" || acsURL.Host == "" {
		return &ErrValidation{Field: "acs_url", Message: "must be a valid URL with scheme and host"}
	}
	if acsURL.Scheme != "http" && acsURL.Scheme != "https" {
		return &ErrValidation{Field: "acs_url", Message: "scheme must be http or https"}
	}

	// Apply default binding and validate.
	if sp.ACSBinding == "" {
		sp.ACSBinding = defaultACSBinding
	} else if !validBindings[sp.ACSBinding] {
		return &ErrValidation{Field: "acs_binding", Message: "must be HTTP-POST or HTTP-Redirect"}
	}

	// Validate attribute mapping if present.
	if sp.AttributeMapping != nil {
		if err := sp.AttributeMapping.Validate(); err != nil {
			return err
		}
	}

	return nil
}
