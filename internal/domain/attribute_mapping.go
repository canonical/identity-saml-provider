package domain

import "strings"

// AttributeMapping defines the per-SP attribute mapping configuration.
// It is stored as JSONB in the service_providers table.
type AttributeMapping struct {
	// NameIDFormat specifies the SAML NameID format for this SP.
	// Accepted values: "persistent", "transient", "emailAddress",
	// "email", "unspecified", or a full URN (e.g.
	// "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent").
	// Defaults to "transient" if not specified.
	NameIDFormat string `json:"nameid_format,omitempty"`

	// SAMLAttributes maps internal field names to SAML attribute names.
	// For example: {"subject": "uid", "email": "mail", "name": "cn"}.
	SAMLAttributes map[string]string `json:"saml_attributes,omitempty"`

	// OIDCClaims maps OIDC claim names to internal field names.
	// For example: {"sub": "subject", "email": "email", "name": "name"}.
	OIDCClaims map[string]string `json:"oidc_claims,omitempty"`

	// Options contains optional transform settings.
	Options MappingOptions `json:"options,omitempty"`
}

// MappingOptions defines optional transformations applied during
// attribute mapping.
type MappingOptions struct {
	// LowercaseEmail lowercases the email attribute value before mapping.
	LowercaseEmail bool `json:"lowercase_email,omitempty"`
}

// Validate checks the mapping configuration for invalid values.
// A nil receiver is considered valid (no mapping configured).
func (m *AttributeMapping) Validate() error {
	if m == nil {
		return nil
	}

	validFormats := map[string]bool{
		"":             true,
		"persistent":   true,
		"transient":    true,
		"emailAddress": true,
		"email":        true,
		"unspecified":  true,
	}

	if !validFormats[m.NameIDFormat] && !isURN(m.NameIDFormat) {
		return &ErrValidation{
			Field:   "nameid_format",
			Message: "must be one of: persistent, transient, emailAddress, unspecified, or a valid URN",
		}
	}

	return nil
}

// isURN reports whether s is a valid URN string (starts with "urn:").
func isURN(s string) bool {
	return strings.HasPrefix(s, "urn:")
}
