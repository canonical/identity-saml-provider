// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import (
	"fmt"
	"sort"
	"strings"
)

// DefaultNameFormat is the SAML attribute NameFormat applied when a
// SAMLAttributeDef does not specify one explicitly.
const DefaultNameFormat = "urn:oasis:names:tc:SAML:2.0:attrname-format:uri"

const (
	// PersistentTypePublic directly emits the canonical upstream OIDC sub claim.
	PersistentTypePublic = "public"

	// PersistentTypePairwise generates and persists a distinct UUID per SP.
	PersistentTypePairwise = "pairwise"
)

// WellKnownField is one entry in the canonical set of internal
// user-attribute fields. It carries the typed dispatch (read, write,
// session fallback) so callers never need to switch on the field name.
//
// Single-valued entries populate GetString, SetString, and (optionally)
// SessionFallback. Multi-valued entries set Multi=true and populate
// GetSlice, SetSlice, and (optionally) SessionSliceFallback; their
// scalar callbacks are nil.
type WellKnownField struct {
	Name  string
	Multi bool

	// Nil for Multi fields.
	GetString func(*UserAttributes) string
	// Nil for Multi fields.
	SetString func(*UserAttributes, string)
	// Returns "" when no equivalent session field exists. Nil for Multi fields.
	SessionFallback func(*Session) string

	// Nil for non-Multi fields. Returns the field's slice by reference
	// for iteration; callers MUST NOT mutate the returned slice.
	GetSlice func(*UserAttributes) []string
	// Nil for non-Multi fields.
	SetSlice func(*UserAttributes, []string)
	// Returns a defensive copy owned by the caller, or nil when no
	// equivalent exists. Nil for non-Multi fields.
	SessionSliceFallback func(*Session) []string
}

// wellKnownFields is the single source of truth for the bridge's
// canonical internal user-attribute fields. Adding a new well-known
// field requires exactly two edits: a struct field on UserAttributes
// and a new entry here.
var wellKnownFields = []WellKnownField{
	{
		Name:            "subject",
		GetString:       func(u *UserAttributes) string { return u.Subject },
		SetString:       func(u *UserAttributes, v string) { u.Subject = v },
		SessionFallback: func(s *Session) string { return s.UserName },
	},
	{
		Name:            "email",
		GetString:       func(u *UserAttributes) string { return u.Email },
		SetString:       func(u *UserAttributes, v string) { u.Email = v },
		SessionFallback: func(s *Session) string { return s.UserEmail },
	},
	{
		Name:            "name",
		GetString:       func(u *UserAttributes) string { return u.Name },
		SetString:       func(u *UserAttributes, v string) { u.Name = v },
		SessionFallback: func(s *Session) string { return s.UserCommonName },
	},
	{
		Name:     "groups",
		Multi:    true,
		GetSlice: func(u *UserAttributes) []string { return u.Groups },
		SetSlice: func(u *UserAttributes, v []string) { u.Groups = v },
		SessionSliceFallback: func(s *Session) []string {
			if len(s.Groups) == 0 {
				return nil
			}
			out := make([]string, len(s.Groups))
			copy(out, s.Groups)
			return out
		},
	},
}

// wellKnownByName indexes wellKnownFields for O(1) lookup. Values are
// indices into wellKnownFields so callers never receive a pointer that
// could be used to mutate the registry.
var wellKnownByName = func() map[string]int {
	m := make(map[string]int, len(wellKnownFields))
	for i, f := range wellKnownFields {
		m[f.Name] = i
	}
	return m
}()

// IsWellKnownField reports whether name identifies a canonical
// internal user-attribute field.
func IsWellKnownField(name string) bool {
	_, ok := wellKnownByName[name]
	return ok
}

// LookupWellKnownField returns a copy of the registry entry for name.
// The second return value reports whether name is well-known. Returning
// a value (not a pointer) prevents callers from mutating the shared
// registry.
func LookupWellKnownField(name string) (WellKnownField, bool) {
	i, ok := wellKnownByName[name]
	if !ok {
		return WellKnownField{}, false
	}
	return wellKnownFields[i], true
}

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

// GetField returns the value of a well-known single-valued field, or
// the Custom entry for unknown names. The multi-valued well-known fields
// (like "groups") are not accessible here; callers should read the
// slice field directly or use LookupWellKnownField for dispatch.
func (u *UserAttributes) GetField(name string) string {
	if u == nil {
		return ""
	}
	if i, ok := wellKnownByName[name]; ok {
		if f := wellKnownFields[i]; f.GetString != nil {
			return f.GetString(u)
		}
	}
	return u.Custom[name]
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

	// PersistentType specifies the persistent NameID mode ("public" or "pairwise").
	// Defaults to "public" when empty.
	PersistentType string `json:"persistent_type,omitempty"`

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

	if m.PersistentType != "" {
		if m.PersistentType != PersistentTypePublic && m.PersistentType != PersistentTypePairwise {
			return &ErrValidation{
				Field:   "persistent_type",
				Message: "must be one of: public, pairwise",
			}
		}
		if !isPersistentFormat(m.NameIDFormat) {
			return &ErrValidation{
				Field:   "persistent_type",
				Message: "persistent_type is only valid when nameid_format is persistent",
			}
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

	// Cross-map resolvability: every saml_attribute_mappings key
	// MUST either be a well-known internal field or appear as a
	// target value in oidc_claim_mappings. Otherwise the SAML
	// attribute would never be populated and is rejected at
	// configuration time. Failing keys are sorted so the error is
	// deterministic across Go's randomised map iteration.
	if len(m.SAMLAttributeMappings) > 0 {
		oidcTargets := make(map[string]struct{}, len(m.OIDCClaimMappings))
		for _, target := range m.OIDCClaimMappings {
			oidcTargets[target] = struct{}{}
		}

		var unresolvable []string
		for field := range m.SAMLAttributeMappings {
			if IsWellKnownField(field) {
				continue
			}
			if _, ok := oidcTargets[field]; ok {
				continue
			}
			unresolvable = append(unresolvable, field)
		}

		if len(unresolvable) > 0 {
			sort.Strings(unresolvable)
			return &ErrValidation{
				Field: fmt.Sprintf("saml_attribute_mappings.%s", unresolvable[0]),
				Message: "internal field must be a well-known field " +
					"(subject, email, name, groups) or a target value in oidc_claim_mappings",
			}
		}
	}

	return nil
}

// isURN reports whether s is a valid URN string (starts with "urn:").
func isURN(s string) bool {
	return strings.HasPrefix(s, "urn:")
}

// isPersistentFormat reports whether format represents a persistent NameID format.
// An empty format defaults to persistent.
func isPersistentFormat(format string) bool {
	return format == "" || format == "persistent" || format == "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"
}
