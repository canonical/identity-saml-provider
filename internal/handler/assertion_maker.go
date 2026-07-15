// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package handler

import (
	"crypto/rand"
	"fmt"

	"github.com/crewjam/saml"

	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/service"
)

// SPAssertionMaker implements the saml.AssertionMaker interface.
// It assumes full control over assertion construction for SPs with custom attribute mappings,
// while delegating to the saml.DefaultAssertionMaker for unmapped SPs.
type SPAssertionMaker struct {
	spService    service.ServiceProviderService
	logger       logging.Logger
	defaultMaker saml.DefaultAssertionMaker
}

// NewSPAssertionMaker creates a new SPAssertionMaker.
func NewSPAssertionMaker(
	spService service.ServiceProviderService,
	logger logging.Logger,
) *SPAssertionMaker {
	return &SPAssertionMaker{
		spService:    spService,
		logger:       logger,
		defaultMaker: saml.DefaultAssertionMaker{},
	}
}

// MakeAssertion constructs an assertion from the session and request and assigns it to req.Assertion.
func (m *SPAssertionMaker) MakeAssertion(req *saml.IdpAuthnRequest, session *saml.Session) error {
	ctx := req.HTTPRequest.Context()
	entityID := req.ServiceProviderMetadata.EntityID

	sp, err := m.spService.GetByEntityID(ctx, entityID)
	if err != nil {
		m.logger.Errorw("failed to look up service provider during assertion making", "entityID", entityID, "error", err)
		return fmt.Errorf("failed to look up service provider %q: %w", entityID, err)
	}

	if sp.AttributeMapping == nil || len(sp.AttributeMapping.SAMLAttributeMappings) == 0 {
		return m.defaultMaker.MakeAssertion(req, session)
	}

	return m.buildCustomAssertion(req, session)
}

func (m *SPAssertionMaker) buildCustomAssertion(req *saml.IdpAuthnRequest, session *saml.Session) error {
	// Generate a random ID for the assertion.
	idBytes := make([]byte, 20)
	if _, err := rand.Read(idBytes); err != nil {
		return fmt.Errorf("failed to generate assertion ID: %w", err)
	}

	notBefore := req.Now.Add(-1 * saml.MaxClockSkew)
	notOnOrAfterAfter := req.Now.Add(saml.MaxIssueDelay)
	if notBefore.Before(req.Request.IssueInstant) {
		notBefore = req.Request.IssueInstant
		notOnOrAfterAfter = notBefore.Add(saml.MaxIssueDelay)
	}

	nameIDFormat := "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"
	if session.NameIDFormat != "" {
		nameIDFormat = session.NameIDFormat
	}

	req.Assertion = &saml.Assertion{
		ID:           fmt.Sprintf("id-%x", idBytes),
		IssueInstant: req.Now,
		Version:      "2.0",
		Issuer: saml.Issuer{
			Format: "urn:oasis:names:tc:SAML:2.0:nameid-format:entity",
			Value:  req.IDP.Metadata().EntityID,
		},
		Subject: &saml.Subject{
			NameID: &saml.NameID{
				Format:          nameIDFormat,
				NameQualifier:   req.IDP.Metadata().EntityID,
				SPNameQualifier: req.ServiceProviderMetadata.EntityID,
				Value:           session.NameID,
			},
			SubjectConfirmations: []saml.SubjectConfirmation{
				{
					Method: "urn:oasis:names:tc:SAML:2.0:cm:bearer",
					SubjectConfirmationData: &saml.SubjectConfirmationData{
						Address:      req.HTTPRequest.RemoteAddr,
						InResponseTo: req.Request.ID,
						NotOnOrAfter: req.Now.Add(saml.MaxIssueDelay),
						Recipient:    req.ACSEndpoint.Location,
					},
				},
			},
		},
		Conditions: &saml.Conditions{
			NotBefore:    notBefore,
			NotOnOrAfter: notOnOrAfterAfter,
			AudienceRestrictions: []saml.AudienceRestriction{
				{
					Audience: saml.Audience{Value: req.ServiceProviderMetadata.EntityID},
				},
			},
		},
		AuthnStatements: []saml.AuthnStatement{
			{
				AuthnInstant: session.CreateTime,
				SessionIndex: session.Index,
				SubjectLocality: &saml.SubjectLocality{
					Address: req.HTTPRequest.RemoteAddr,
				},
				AuthnContext: saml.AuthnContext{
					AuthnContextClassRef: &saml.AuthnContextClassRef{
						Value: "urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport",
					},
				},
			},
		},
		AttributeStatements: []saml.AttributeStatement{
			{
				Attributes: session.CustomAttributes,
			},
		},
	}

	return nil
}
