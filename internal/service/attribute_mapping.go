// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/repository"
	"github.com/canonical/identity-saml-provider/internal/tracing"
)

type mappingService struct {
	spRepo        repository.ServiceProviderRepository
	persistentIDs repository.PersistentNameIDRepository
	logger        logging.Logger
	tracer        tracing.TracingInterface
}

// NewMappingService creates a new MappingService.
func NewMappingService(
	spRepo repository.ServiceProviderRepository,
	persistentIDs repository.PersistentNameIDRepository,
	logger logging.Logger,
	tracer tracing.TracingInterface,
) MappingService {
	return &mappingService{
		spRepo:        spRepo,
		persistentIDs: persistentIDs,
		logger:        logger,
		tracer:        tracer,
	}
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
		value, formatURN, resolveErr := s.resolveNameID(ctx, session, attrs, mapping.NameIDFormat, entityID)
		if resolveErr != nil {
			return nil, resolveErr
		}
		mapped.NameIDFormat = formatURN
		mapped.NameID = value
	}

	if len(mapping.SAMLAttributeMappings) > 0 {
		customAttrs := buildSAMLAttributes(attrs, mapping.SAMLAttributeMappings, logger, entityID)
		mapped.CustomAttributes = append(mapped.CustomAttributes, customAttrs...)
	}

	return &mapped, nil
}

// nameIDFormatToURN converts a NameID format identifier to its full
// SAML URN. Accepts both the short names ("persistent", "transient",
// "emailAddress"/"email", "unspecified") and full SAML URNs (returned
// unchanged). Unknown values fall back to the transient URN to
// preserve pre-change permissive behavior at the assertion boundary.
func nameIDFormatToURN(format string) string {
	switch normalizeNameIDFormat(format) {
	case nameIDFormatPersistent:
		return "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"
	case nameIDFormatTransient:
		return "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"
	case nameIDFormatEmail:
		return "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
	case nameIDFormatUnspecified:
		return "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified"
	default:
		if strings.HasPrefix(format, "urn:") {
			return format
		}
		return "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"
	}
}

// Canonical short-name identifiers used for NameID format dispatch.
// resolveNameID and nameIDFormatToURN both switch on these values
// after running the configured format through normalizeNameIDFormat.
const (
	nameIDFormatPersistent  = "persistent"
	nameIDFormatTransient   = "transient"
	nameIDFormatEmail       = "emailAddress"
	nameIDFormatUnspecified = "unspecified"
)

// normalizeNameIDFormat collapses both the short names and the full
// SAML URN spellings of each supported NameID format to a single
// canonical short name. Unrecognized values are returned unchanged so
// callers can decide whether to fall through to permissive handling.
func normalizeNameIDFormat(format string) string {
	switch strings.ToLower(format) {
	case "persistent",
		"urn:oasis:names:tc:saml:2.0:nameid-format:persistent":
		return nameIDFormatPersistent
	case "transient",
		"urn:oasis:names:tc:saml:2.0:nameid-format:transient":
		return nameIDFormatTransient
	case "emailaddress", "email",
		"urn:oasis:names:tc:saml:1.1:nameid-format:emailaddress":
		return nameIDFormatEmail
	case "unspecified",
		"urn:oasis:names:tc:saml:1.1:nameid-format:unspecified":
		return nameIDFormatUnspecified
	default:
		return format
	}
}

// BuildUserAttributes constructs a typed UserAttributes from session
// fields and raw OIDC claims using the SP's OIDCClaimMappings. When
// oidcClaimMappings is empty, the default mapping
// {"sub":"subject","email":"email","name":"name","groups":"groups"}
// is applied. Well-known internal fields dispatch through the
// domain.WellKnownField registry (typed setter + session fallback);
// unknown internal fields land in UserAttributes.Custom when the OIDC
// value is non-empty.
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
		field, known := domain.LookupWellKnownField(internalField)

		if known && field.Multi {
			groups := extractGroupsFromClaims(rawClaims, oidcClaim)
			if groups == nil && session != nil && field.SessionSliceFallback != nil {
				groups = field.SessionSliceFallback(session)
			}
			field.SetSlice(attrs, groups)
			continue
		}

		value := extractStringClaim(rawClaims, oidcClaim)
		if value == "" && session != nil && known && field.SessionFallback != nil {
			value = field.SessionFallback(session)
		}

		if known && field.SetString != nil {
			field.SetString(attrs, value)
			continue
		}
		if value != "" {
			attrs.Custom[internalField] = value
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

// extractGroupsFromClaims returns the OIDC claim value as a []string,
// or nil when the claim is absent, not a JSON array, or contains no
// string values. Session-level fallback is the caller's responsibility.
func extractGroupsFromClaims(rawClaims map[string]interface{}, claim string) []string {
	if rawClaims == nil {
		return nil
	}
	v, ok := rawClaims[claim]
	if !ok {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
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
		if f, known := domain.LookupWellKnownField(internalField); known && f.Multi {
			// Multi-valued well-known fields dispatch through the
			// registry so adding a new multi field does not require
			// editing this function. The registry contract guarantees
			// GetSlice is populated for Multi entries.
			values := f.GetSlice(attrs)
			if len(values) == 0 {
				logger.Debugw("Mapped SAML attribute omitted: no values available",
					"entityID", entityID,
					"internalField", internalField,
					"samlAttrName", def.Name,
				)
				continue
			}
			attrValues := make([]domain.AttributeValue, 0, len(values))
			for _, v := range values {
				attrValues = append(attrValues, domain.AttributeValue{Type: "xs:string", Value: v})
			}
			result = append(result, domain.Attribute{
				Name:         def.Name,
				FriendlyName: def.FriendlyName,
				NameFormat:   def.EffectiveNameFormat(),
				Values:       attrValues,
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

// resolveNameID returns the NameID value and full format URN for the
// configured nameid_format. The persistent and emailAddress branches
// fail closed on missing input rather than emit a non-conforming
// NameID. The canonical OIDC `sub` claim is extracted from session
// only on the persistent branch, where it is the stable lookup key.
func (s *mappingService) resolveNameID(
	ctx context.Context,
	session *domain.Session,
	attrs *domain.UserAttributes,
	nameIDFormat string,
	entityID string,
) (value string, formatURN string, err error) {
	logger := logging.FromContext(ctx, s.logger)
	formatURN = nameIDFormatToURN(nameIDFormat)

	logger.Debugw("Resolving NameID",
		"entityID", entityID,
		"format", nameIDFormat,
	)

	switch normalizeNameIDFormat(nameIDFormat) {
	case nameIDFormatPersistent:
		// Persistent NameIDs are keyed on the canonical OIDC sub,
		// never the mapped attrs.Subject (admins can remap that).
		canonicalSubject, ok := session.CanonicalSubject()
		if !ok {
			resErr := &domain.ErrNameIDResolution{
				EntityID: entityID,
				Format:   nameIDFormat,
				Reason:   "missing or empty OIDC sub claim in session",
			}
			logger.Errorw("Persistent NameID resolution failed",
				"entityID", entityID,
				"canonicalSubject", "",
				"error", resErr,
			)
			return "", formatURN, resErr
		}
		persistentID, repoErr := s.persistentIDs.GetOrCreate(ctx, entityID, canonicalSubject)
		if repoErr != nil {
			resErr := &domain.ErrNameIDResolution{
				EntityID: entityID,
				Format:   nameIDFormat,
				Reason:   "persistent NameID storage call failed",
				Err:      repoErr,
			}
			logger.Errorw("Persistent NameID resolution failed",
				"entityID", entityID,
				"canonicalSubject", canonicalSubject,
				"error", resErr,
			)
			return "", formatURN, resErr
		}
		logger.Infow("Resolved persistent NameID",
			"entityID", entityID,
			"canonicalSubject", canonicalSubject,
		)
		return persistentID, formatURN, nil

	case nameIDFormatTransient:
		return uuid.New().String(), formatURN, nil

	case nameIDFormatEmail:
		if attrs.Email == "" {
			resErr := &domain.ErrNameIDResolution{
				EntityID: entityID,
				Format:   nameIDFormat,
				Reason:   "emailAddress NameID requested but user email is empty",
			}
			logger.Errorw("Email NameID resolution failed",
				"entityID", entityID,
				"error", resErr,
			)
			return "", formatURN, resErr
		}
		return attrs.Email, formatURN, nil

	default:
		// Legacy permissive behavior for unspecified / unknown URN /
		// empty: email preferred, subject fallback.
		if attrs.Email != "" {
			return attrs.Email, formatURN, nil
		}
		return attrs.Subject, formatURN, nil
	}
}
