package provider

import (
	"strings"

	"github.com/crewjam/saml"
)

// AttributeMapping defines the per-SP attribute mapping configuration.
// It specifies how OIDC claims are mapped to SAML attributes for a given service provider.
type AttributeMapping struct {
	// NameIDFormat specifies the SAML NameID format for this SP.
	// Accepted values: "persistent", "transient", "emailAddress", or a full URN.
	// Defaults to "transient" if not specified.
	NameIDFormat string `json:"nameid_format,omitempty"`

	// SAMLAttributes maps internal field names to SAML attribute names.
	// For example: {"subject": "uid", "email": "mail", "name": "cn"}
	SAMLAttributes map[string]string `json:"saml_attributes,omitempty"`

	// OIDCClaims maps OIDC claim names to internal field names.
	// For example: {"sub": "subject", "email": "email", "name": "name", "groups": "groups"}
	OIDCClaims map[string]string `json:"oidc_claims,omitempty"`

	// Options contains optional transform settings.
	Options MappingOptions `json:"options,omitempty"`
}

// MappingOptions defines optional transformations applied during attribute mapping.
type MappingOptions struct {
	// LowercaseEmail lowercases the email attribute value before mapping.
	LowercaseEmail bool `json:"lowercase_email,omitempty"`
}

// nameIDFormatToURN converts a short NameID format name to its full SAML URN.
func nameIDFormatToURN(format string) string {
	switch strings.ToLower(format) {
	case "persistent":
		return "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"
	case "transient":
		return "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"
	case "emailaddress", "email":
		return "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
	case "unspecified":
		return "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified"
	default:
		if strings.HasPrefix(format, "urn:") {
			return format
		}
		return "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"
	}
}

// applyAttributeMapping applies a per-SP attribute mapping to a session.
// It returns a modified copy of the session with the mapped attributes.
// If mapping is nil, the session is returned unmodified.
// rawClaims contains all claims extracted from the OIDC ID token, allowing
// the mapping to use claims beyond the standard session fields.
func applyAttributeMapping(session *saml.Session, mapping *AttributeMapping, rawClaims map[string]interface{}) *saml.Session {
	if mapping == nil {
		return session
	}

	// Create a copy of the session to avoid modifying the stored version
	mapped := *session
	// Deep copy slice fields to avoid shared references
	if len(session.Groups) > 0 {
		mapped.Groups = make([]string, len(session.Groups))
		copy(mapped.Groups, session.Groups)
	}
	if len(session.CustomAttributes) > 0 {
		mapped.CustomAttributes = make([]saml.Attribute, len(session.CustomAttributes))
		copy(mapped.CustomAttributes, session.CustomAttributes)
	}

	// Build the internal user model from session fields and raw OIDC claims.
	// The OIDC claims mapping determines which OIDC claim populates which internal field.
	// When raw claims are available, they allow mapping arbitrary OIDC claims
	// beyond the standard session fields (email, sub, name, groups).
	internalModel := buildInternalModel(session, mapping.OIDCClaims, rawClaims)

	// Apply transforms
	if mapping.Options.LowercaseEmail {
		if v, ok := internalModel["email"]; ok {
			internalModel["email"] = strings.ToLower(v)
		}
	}

	// Set NameID based on format
	if mapping.NameIDFormat != "" {
		mapped.NameIDFormat = nameIDFormatToURN(mapping.NameIDFormat)
		mapped.NameID = getNameIDValue(internalModel, mapping.NameIDFormat)
	}

	// If SAML attributes are configured, use custom attributes instead of built-in fields
	if len(mapping.SAMLAttributes) > 0 {
		// Clear built-in fields to prevent default attribute generation by the library
		mapped.UserEmail = ""
		mapped.UserCommonName = ""
		mapped.UserName = ""
		mapped.UserSurname = ""
		mapped.UserGivenName = ""
		mapped.UserScopedAffiliation = ""
		mapped.Groups = nil

		// Build custom SAML attributes from the mapping
		var customAttrs []saml.Attribute
		for internalField, samlAttrName := range mapping.SAMLAttributes {
			value, ok := internalModel[internalField]
			if !ok || value == "" {
				continue
			}

			// Check if this is the groups field (multi-valued, stored as null-separated)
			if strings.Contains(value, "\x00") {
				values := strings.Split(value, "\x00")
				var attrValues []saml.AttributeValue
				for _, g := range values {
					if g != "" {
						attrValues = append(attrValues, saml.AttributeValue{
							Type:  "xs:string",
							Value: g,
						})
					}
				}
				if len(attrValues) > 0 {
					customAttrs = append(customAttrs, saml.Attribute{
						FriendlyName: samlAttrName,
						Name:         samlAttrName,
						NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
						Values:       attrValues,
					})
				}
				continue
			}

			customAttrs = append(customAttrs, saml.Attribute{
				FriendlyName: samlAttrName,
				Name:         samlAttrName,
				NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
				Values: []saml.AttributeValue{
					{Type: "xs:string", Value: value},
				},
			})
		}

		// Preserve any existing custom attributes and append the mapped ones
		mapped.CustomAttributes = append(mapped.CustomAttributes, customAttrs...)
	}

	return &mapped
}

// buildInternalModel constructs a map of internal field names to values
// from the session and raw OIDC claims, using the OIDC claims mapping if provided.
// When rawClaims is available, claim values are taken directly from the OIDC token,
// allowing mapping of arbitrary claims beyond the standard session fields.
func buildInternalModel(session *saml.Session, oidcClaims map[string]string, rawClaims map[string]interface{}) map[string]string {
	// Default OIDC-to-internal mapping
	oidcToInternal := map[string]string{
		"sub":    "subject",
		"email":  "email",
		"name":   "name",
		"groups": "groups",
	}

	// Override with custom mapping if provided
	if len(oidcClaims) > 0 {
		oidcToInternal = oidcClaims
	}

	// Build OIDC values from raw claims if available, falling back to session fields.
	// Raw claims allow access to arbitrary OIDC token claims (e.g., preferred_username,
	// given_name, family_name) that are not stored in standard session fields.
	oidcValues := map[string]string{
		"sub":   session.UserName,
		"email": session.UserEmail,
		"name":  session.UserCommonName,
	}

	// Groups are multi-valued, encode as null-separated string
	if len(session.Groups) > 0 {
		oidcValues["groups"] = strings.Join(session.Groups, "\x00")
	}

	// If raw claims are available, overlay with values from the OIDC token.
	// This lets us access any claim from the token, not just the 4 hardcoded ones.
	if len(rawClaims) > 0 {
		for oidcClaim := range oidcToInternal {
			if rawVal, ok := rawClaims[oidcClaim]; ok {
				switch v := rawVal.(type) {
				case string:
					oidcValues[oidcClaim] = v
				case []interface{}:
					// Multi-valued claim (e.g., groups)
					var parts []string
					for _, item := range v {
						if s, ok := item.(string); ok {
							parts = append(parts, s)
						}
					}
					if len(parts) > 0 {
						oidcValues[oidcClaim] = strings.Join(parts, "\x00")
					}
				}
			}
		}
	}

	// Build internal model
	model := make(map[string]string)
	for oidcClaim, internalField := range oidcToInternal {
		if value, ok := oidcValues[oidcClaim]; ok {
			model[internalField] = value
		}
	}

	return model
}

// getNameIDValue returns the NameID value based on the configured format
// and the internal user model.
func getNameIDValue(model map[string]string, format string) string {
	switch strings.ToLower(format) {
	case "persistent":
		if v, ok := model["subject"]; ok && v != "" {
			return v
		}
		// Fall back to email for persistent if no subject
		if v, ok := model["email"]; ok && v != "" {
			return v
		}
		return ""
	case "emailaddress", "email":
		if v, ok := model["email"]; ok && v != "" {
			return v
		}
		return ""
	default:
		// For transient/unspecified/unknown formats, use email then subject
		if v, ok := model["email"]; ok && v != "" {
			return v
		}
		if v, ok := model["subject"]; ok && v != "" {
			return v
		}
		return ""
	}
}
