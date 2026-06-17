// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import (
	"fmt"
	"strings"
)

// DefaultNameFormat is the SAML attribute NameFormat applied when a
// SAMLAttributeDef does not specify one explicitly.
const DefaultNameFormat = "urn:oasis:names:tc:SAML:2.0:attrname-format:uri"

// SAMLAttributeDef defines how an internal user field is emitted as a
// SAML attribute in the assertion.
type SAMLAttributeDef struct {
	// Name is the SAML attribute Name (required). Typically, an OID
	// (e.g. "urn:oid:0.9.2342.19200300.100.1.3") or an LDAP-style
	// short name (e.g. "mail").
	Name string `json:"name"`

	// FriendlyName is the optional SAML FriendlyName. When empty, the
	// emitted SAML attribute SHALL omit the FriendlyName XML attribute.
	FriendlyName string `json:"friendly_name,omitempty"`

	// NameFormat is the SAML attribute NameFormat URI. When empty,
	// EffectiveNameFormat falls back to DefaultNameFormat.
	NameFormat string `json:"name_format,omitempty"`
}

// EffectiveNameFormat returns the configured NameFormat, falling back
// to DefaultNameFormat when the configured value is empty.
func (d SAMLAttributeDef) EffectiveNameFormat() string {
	if d.NameFormat != "" {
		return d.NameFormat
	}
	return DefaultNameFormat
}

// UserAttributes is the internal, typed representation of a user used
// between OIDC claim extraction and SAML attribute emission.
//
// Well-known fields are stored directly. Non-well-known fields produced
// by custom OIDCClaimMappings entries are stored in Custom.
type UserAttributes struct {
	Subject string            `json:"subject,omitempty"`
	Email   string            `json:"email,omitempty"`
	Name    string            `json:"name,omitempty"`
	Groups  []string          `json:"groups,omitempty"`
	Custom  map[string]string `json:"custom,omitempty"`
}

// GetField returns the value of a well-known single-valued field by
// name, falling back to the Custom map for unknown names. Returns the
// empty string when no value is present. The multi-valued "groups"
// field is not accessible via GetField; callers must read
// UserAttributes.Groups directly.
func (u *UserAttributes) GetField(name string) string {
	if u == nil {
		return ""
	}
	switch name {
	case "subject":
		return u.Subject
	case "email":
		return u.Email
	case "name":
		return u.Name
	default:
		return u.Custom[name]
	}
}

// AttributeMapping defines the per-SP attribute mapping configuration.
// It is stored as JSONB in the service_providers table.
type AttributeMapping struct {
	// NameIDFormat specifies the SAML NameID format for this SP.
	// Accepted values: "persistent", "transient", "emailAddress",
	// "email", "unspecified", or a full URN (e.g.
	// "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent").
	// Defaults to "transient" if not specified.
	NameIDFormat string `json:"nameid_format,omitempty"`

	// SAMLAttributeMappings maps internal field names to SAML
	// attribute definitions. The well-known internal field names are
	// "subject", "email", "name", and "groups"; other keys are
	// populated via OIDCClaimMappings into UserAttributes.Custom.
	SAMLAttributeMappings map[string]SAMLAttributeDef `json:"saml_attribute_mappings,omitempty"`

	// OIDCClaimMappings maps OIDC claim names to internal field names.
	// For example: {"sub": "subject", "email": "email", "name": "name"}.
	// An empty map causes BuildUserAttributes to apply the default
	// mapping {"sub":"subject","email":"email","name":"name","groups":"groups"}.
	OIDCClaimMappings map[string]string `json:"oidc_claim_mappings,omitempty"`

	// Options contains optional transform settings.
	Options MappingOptions `json:"options,omitempty"`
}

// MappingOptions defines optional transformations applied during
// attribute mapping.
type MappingOptions struct {
	// LowercaseEmail lowercases the email attribute value before
	// SAML attributes are built.
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
			Message: "must be one of: persistent, transient, emailAddress, email, unspecified, or a valid URN",
		}
	}

	for field, def := range m.SAMLAttributeMappings {
		if def.Name == "" {
			return &ErrValidation{
				Field:   fmt.Sprintf("saml_attribute_mappings.%s.name", field),
				Message: "SAML attribute name is required",
			}
		}
	}

	return nil
}

// isURN reports whether s is a valid URN string (starts with "urn:").
func isURN(s string) bool {
	return strings.HasPrefix(s, "urn:")
}
