// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package service

import (
	"context"
	"strings"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/repository"
	"github.com/canonical/identity-saml-provider/internal/tracing"
)

type mappingService struct {
	spRepo repository.ServiceProviderRepository
	logger logging.Logger
	tracer tracing.TracingInterface
}

// NewMappingService creates a new MappingService.
func NewMappingService(spRepo repository.ServiceProviderRepository, logger logging.Logger, tracer tracing.TracingInterface) MappingService {
	return &mappingService{spRepo: spRepo, logger: logger, tracer: tracer}
}

func (s *mappingService) ApplyMapping(ctx context.Context, session *domain.Session, entityID string) (*domain.Session, error) {
	ctx, span := s.tracer.Start(ctx, "service.mapping.apply_mapping")
	defer span.End()

	logger := logging.FromContext(ctx, s.logger)

	mapping, err := s.spRepo.GetAttributeMapping(ctx, entityID)
	if err != nil {
		logger.Errorw("Error retrieving attribute mapping", "entityID", entityID, "error", err)
		return session, nil // graceful degradation: return unmapped session
	}
	if mapping == nil {
		return session, nil // no mapping configured, return as-is
	}

	logger.Debugw("Applying per-SP attribute mapping", "entityID", entityID)

	// Build the typed internal user model from session fields and raw OIDC claims.
	attrs := BuildUserAttributes(session, mapping.OIDCClaimMappings, session.RawOIDCClaims)

	// Apply transforms.
	if mapping.Options.LowercaseEmail {
		attrs.Email = strings.ToLower(attrs.Email)
	}

	// Create a mapped copy of the session.
	mapped := *session
	// Deep copy slice fields to avoid shared references.
	if len(session.Groups) > 0 {
		mapped.Groups = make([]string, len(session.Groups))
		copy(mapped.Groups, session.Groups)
	}
	if len(session.CustomAttributes) > 0 {
		mapped.CustomAttributes = make([]domain.Attribute, len(session.CustomAttributes))
		copy(mapped.CustomAttributes, session.CustomAttributes)
	}

	// Set NameID based on the configured format.
	if mapping.NameIDFormat != "" {
		mapped.NameIDFormat = nameIDFormatToURN(mapping.NameIDFormat)
		mapped.NameID = getNameIDValue(attrs, mapping.NameIDFormat)
	}

	// When SAML attribute mappings are configured, suppress the SAML
	// library's default attribute emission and append the custom ones.
	if len(mapping.SAMLAttributeMappings) > 0 {
		mapped.UserEmail = ""
		mapped.UserCommonName = ""
		mapped.UserName = ""
		mapped.UserSurname = ""
		mapped.UserGivenName = ""
		mapped.UserScopedAffiliation = ""
		mapped.Groups = nil

		customAttrs := buildSAMLAttributes(attrs, mapping.SAMLAttributeMappings, logger, entityID)
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

// BuildUserAttributes constructs a typed UserAttributes value from the
// session fields and raw OIDC claims using the SP's OIDCClaimMappings.
// When oidcClaimMappings is empty, the default mapping
// {"sub":"subject","email":"email","name":"name","groups":"groups"} is
// applied. Single-valued claims populate the matching well-known field
// (Subject, Email, Name) or the Custom map when the internal field is
// not well-known. The "groups" internal field is populated as []string
// from a multi-valued OIDC claim. Claims missing from the raw OIDC
// token fall back to the equivalent session field when one exists.
func BuildUserAttributes(session *domain.Session, oidcClaimMappings map[string]string, rawClaims map[string]interface{}) *domain.UserAttributes {
	// Default OIDC-to-internal mapping.
	oidcToInternal := map[string]string{
		"sub":    "subject",
		"email":  "email",
		"name":   "name",
		"groups": "groups",
	}
	if len(oidcClaimMappings) > 0 {
		oidcToInternal = oidcClaimMappings
	}

	attrs := &domain.UserAttributes{Custom: make(map[string]string)}

	for oidcClaim, internalField := range oidcToInternal {
		if internalField == "groups" {
			attrs.Groups = extractGroups(rawClaims, oidcClaim, session)
			continue
		}

		value := extractStringClaim(rawClaims, oidcClaim)
		if value == "" {
			value = sessionFieldFallback(session, internalField)
		}

		switch internalField {
		case "subject":
			attrs.Subject = value
		case "email":
			attrs.Email = value
		case "name":
			attrs.Name = value
		default:
			if value != "" {
				attrs.Custom[internalField] = value
			}
		}
	}

	return attrs
}

// extractStringClaim returns the string value of the given OIDC claim
// from rawClaims, or empty string if the claim is absent or not a
// string.
func extractStringClaim(rawClaims map[string]interface{}, claim string) string {
	if rawClaims == nil {
		return ""
	}
	if v, ok := rawClaims[claim]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// extractGroups returns the value of a multi-valued OIDC claim (a JSON
// array of strings) as a []string. Falls back to session.Groups when
// the claim is absent in rawClaims.
func extractGroups(rawClaims map[string]interface{}, claim string, session *domain.Session) []string {
	if rawClaims != nil {
		if v, ok := rawClaims[claim]; ok {
			if arr, ok := v.([]interface{}); ok {
				result := make([]string, 0, len(arr))
				for _, item := range arr {
					if s, ok := item.(string); ok {
						result = append(result, s)
					}
				}
				if len(result) > 0 {
					return result
				}
			}
		}
	}
	if session != nil && len(session.Groups) > 0 {
		out := make([]string, len(session.Groups))
		copy(out, session.Groups)
		return out
	}
	return nil
}

// sessionFieldFallback returns the canonical session field value for a
// well-known internal field, used when the configured OIDC claim is
// absent from the raw token. Custom (non-well-known) internal fields
// have no session-level equivalent and return the empty string.
func sessionFieldFallback(session *domain.Session, internalField string) string {
	if session == nil {
		return ""
	}
	switch internalField {
	case "subject":
		return session.UserName
	case "email":
		return session.UserEmail
	case "name":
		return session.UserCommonName
	}
	return ""
}

// buildSAMLAttributes converts UserAttributes into SAML Attribute
// values per the SP's SAMLAttributeMappings. Empty source values cause
// the corresponding attribute to be omitted from the assertion, with a
// DEBUG log capturing the omission.
func buildSAMLAttributes(attrs *domain.UserAttributes, samlMappings map[string]domain.SAMLAttributeDef, logger logging.Logger, entityID string) []domain.Attribute {
	if len(samlMappings) == 0 {
		return nil
	}

	result := make([]domain.Attribute, 0, len(samlMappings))

	for internalField, def := range samlMappings {
		if internalField == "groups" {
			if len(attrs.Groups) == 0 {
				logger.Debugw("Mapped SAML attribute omitted: no groups available",
					"entityID", entityID,
					"internalField", internalField,
					"samlAttrName", def.Name,
				)
				continue
			}
			values := make([]domain.AttributeValue, 0, len(attrs.Groups))
			for _, g := range attrs.Groups {
				values = append(values, domain.AttributeValue{Type: "xs:string", Value: g})
			}
			result = append(result, domain.Attribute{
				Name:         def.Name,
				FriendlyName: def.FriendlyName,
				NameFormat:   def.EffectiveNameFormat(),
				Values:       values,
			})
			continue
		}

		value := attrs.GetField(internalField)
		if value == "" {
			logger.Debugw("Mapped SAML attribute omitted: claim value empty",
				"entityID", entityID,
				"internalField", internalField,
				"samlAttrName", def.Name,
			)
			continue
		}

		result = append(result, domain.Attribute{
			Name:         def.Name,
			FriendlyName: def.FriendlyName,
			NameFormat:   def.EffectiveNameFormat(),
			Values:       []domain.AttributeValue{{Type: "xs:string", Value: value}},
		})
	}

	logger.Debugw("Built SAML attributes",
		"entityID", entityID,
		"mappedCount", len(result),
		"totalConfigured", len(samlMappings),
	)

	return result
}

// getNameIDValue returns the NameID value based on the configured
// format and the typed user model.
func getNameIDValue(attrs *domain.UserAttributes, format string) string {
	switch strings.ToLower(format) {
	case "persistent":
		if attrs.Subject != "" {
			return attrs.Subject
		}
		return attrs.Email
	case "emailaddress", "email":
		return attrs.Email
	default:
		if attrs.Email != "" {
			return attrs.Email
		}
		return attrs.Subject
	}
}
