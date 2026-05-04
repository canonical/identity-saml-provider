package service

import (
	"context"
	"strings"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/repository"
)

type mappingService struct {
	spRepo repository.ServiceProviderRepository
	logger logging.Logger
}

// NewMappingService creates a new MappingService.
func NewMappingService(spRepo repository.ServiceProviderRepository, logger logging.Logger) MappingService {
	return &mappingService{spRepo: spRepo, logger: logger}
}

func (s *mappingService) ApplyMapping(ctx context.Context, session *domain.Session, entityID string) (*domain.Session, error) {
	mapping, err := s.spRepo.GetAttributeMapping(ctx, entityID)
	if err != nil {
		s.logger.Errorw("Error retrieving attribute mapping", "entityID", entityID, "error", err)
		return session, nil // graceful degradation: return unmapped session
	}
	if mapping == nil {
		return session, nil // no mapping configured, return as-is
	}

	s.logger.Infow("Applying per-SP attribute mapping", "entityID", entityID)

	// Build internal user model from session fields and raw OIDC claims
	internalModel := buildInternalModel(session, mapping.OIDCClaims, session.RawOIDCClaims)

	// Apply transforms
	if mapping.Options.LowercaseEmail {
		if v, ok := internalModel["email"]; ok {
			internalModel["email"] = strings.ToLower(v)
		}
	}

	// Create a mapped copy of the session
	mapped := *session
	// Deep copy slice fields to avoid shared references
	if len(session.Groups) > 0 {
		mapped.Groups = make([]string, len(session.Groups))
		copy(mapped.Groups, session.Groups)
	}
	if len(session.CustomAttributes) > 0 {
		mapped.CustomAttributes = make([]domain.Attribute, len(session.CustomAttributes))
		copy(mapped.CustomAttributes, session.CustomAttributes)
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
		var customAttrs []domain.Attribute
		for internalField, samlAttrName := range mapping.SAMLAttributes {
			value, ok := internalModel[internalField]
			if !ok || value == "" {
				continue
			}

			// Check if this is a multi-valued field (stored as null-separated)
			if strings.Contains(value, "\x00") {
				values := strings.Split(value, "\x00")
				var attrValues []domain.AttributeValue
				for _, g := range values {
					if g != "" {
						attrValues = append(attrValues, domain.AttributeValue{
							Type:  "xs:string",
							Value: g,
						})
					}
				}
				if len(attrValues) > 0 {
					customAttrs = append(customAttrs, domain.Attribute{
						FriendlyName: samlAttrName,
						Name:         samlAttrName,
						NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
						Values:       attrValues,
					})
				}
				continue
			}

			customAttrs = append(customAttrs, domain.Attribute{
				FriendlyName: samlAttrName,
				Name:         samlAttrName,
				NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
				Values: []domain.AttributeValue{
					{Type: "xs:string", Value: value},
				},
			})
		}

		// Preserve any existing custom attributes and append the mapped ones
		mapped.CustomAttributes = append(mapped.CustomAttributes, customAttrs...)
	}

	return &mapped, nil
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

// buildInternalModel constructs a map of internal field names to values
// from the session and raw OIDC claims, using the OIDC claims mapping if provided.
func buildInternalModel(session *domain.Session, oidcClaims map[string]string, rawClaims map[string]interface{}) map[string]string {
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

	// Build OIDC values from session fields, falling back where available.
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
