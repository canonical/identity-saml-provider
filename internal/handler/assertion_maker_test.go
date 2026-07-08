// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package handler_test

import (
	"context"
	"crypto/x509"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"go.uber.org/mock/gomock"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/handler"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/mocks"
)

func TestSPAssertionMaker_MakeAssertion_Unmapped(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockSPService := mocks.NewMockServiceProviderService(ctrl)
	logger := logging.NewNopLogger()

	maker := handler.NewSPAssertionMaker(mockSPService, logger)

	entityID := "sp-unmapped"
	mockSPService.EXPECT().GetByEntityID(gomock.Any(), entityID).Return(&domain.ServiceProvider{
		EntityID:         entityID,
		AttributeMapping: nil,
	}, nil)

	req := &saml.IdpAuthnRequest{
		HTTPRequest: &http.Request{},
		IDP: &saml.IdentityProvider{
			MetadataURL: url.URL{Scheme: "http", Host: "idp"},
			Certificate: &x509.Certificate{},
		},
		ServiceProviderMetadata: &saml.EntityDescriptor{
			EntityID: entityID,
		},
		Request: saml.AuthnRequest{
			IssueInstant: time.Now(),
		},
		SPSSODescriptor: &saml.SPSSODescriptor{},
		ACSEndpoint:     &saml.IndexedEndpoint{Location: "http://acs"},
		Now:             time.Now(),
	}

	session := &saml.Session{
		NameID:    "user@test.com",
		UserEmail: "user@test.com",
	}

	err := maker.MakeAssertion(req, session)
	if err != nil {
		t.Fatalf("MakeAssertion failed: %v", err)
	}

	if req.Assertion == nil {
		t.Fatal("Assertion was not created")
	}

	// Verify delegation occurred (default logic sets email if UserEmail is set).
	foundEmail := false
	for _, statement := range req.Assertion.AttributeStatements {
		for _, attr := range statement.Attributes {
			if attr.FriendlyName == "mail" && attr.Values[0].Value == "user@test.com" {
				foundEmail = true
			}
		}
	}

	if !foundEmail {
		t.Error("Expected default maker to populate mail attribute")
	}
}

func TestSPAssertionMaker_MakeAssertion_Mapped(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockSPService := mocks.NewMockServiceProviderService(ctrl)
	logger := logging.NewNopLogger()

	maker := handler.NewSPAssertionMaker(mockSPService, logger)

	entityID := "sp-mapped"
	mockSPService.EXPECT().GetByEntityID(gomock.Any(), entityID).Return(&domain.ServiceProvider{
		EntityID: entityID,
		AttributeMapping: &domain.AttributeMapping{
			SAMLAttributeMappings: map[string]domain.SAMLAttributeDef{
				"email": {Name: "mapped-email"},
			},
		},
	}, nil)

	req := &saml.IdpAuthnRequest{
		HTTPRequest: &http.Request{},
		IDP: &saml.IdentityProvider{
			MetadataURL: url.URL{Scheme: "http", Host: "idp"},
			Certificate: &x509.Certificate{},
		},
		ServiceProviderMetadata: &saml.EntityDescriptor{
			EntityID: entityID,
		},
		Request: saml.AuthnRequest{
			IssueInstant: time.Now(),
		},
		SPSSODescriptor: &saml.SPSSODescriptor{},
		ACSEndpoint:     &saml.IndexedEndpoint{Location: "http://acs"},
		Now:             time.Now(),
	}

	// For a mapped SP, the applyMapping step (which happens before MakeAssertion)
	// would have populated CustomAttributes. Here we just mock what the session would look like.
	session := &saml.Session{
		NameID:    "user@test.com",
		UserEmail: "user@test.com", // This shouldn't leak
		CustomAttributes: []saml.Attribute{
			{
				Name: "mapped-email",
				Values: []saml.AttributeValue{
					{Type: "xs:string", Value: "custom@test.com"},
				},
			},
		},
	}

	err := maker.MakeAssertion(req, session)
	if err != nil {
		t.Fatalf("MakeAssertion failed: %v", err)
	}

	if req.Assertion == nil {
		t.Fatal("Assertion was not created")
	}

	// Verify no leakage and custom attributes present.
	foundCustom := false
	foundDefaultMail := false
	for _, statement := range req.Assertion.AttributeStatements {
		for _, attr := range statement.Attributes {
			if attr.Name == "mapped-email" && attr.Values[0].Value == "custom@test.com" {
				foundCustom = true
			}
			if attr.FriendlyName == "mail" {
				foundDefaultMail = true
			}
		}
	}

	if !foundCustom {
		t.Error("Expected mapped-email attribute")
	}
	if foundDefaultMail {
		t.Error("Expected no default attributes to leak")
	}
}

func TestSPAssertionMaker_MakeAssertion_LookupFail(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockSPService := mocks.NewMockServiceProviderService(ctrl)
	logger := logging.NewNopLogger()

	maker := handler.NewSPAssertionMaker(mockSPService, logger)

	entityID := "sp-error"
	mockSPService.EXPECT().GetByEntityID(gomock.Any(), entityID).Return(nil, context.DeadlineExceeded)

	req := &saml.IdpAuthnRequest{
		HTTPRequest: &http.Request{},
		ServiceProviderMetadata: &saml.EntityDescriptor{
			EntityID: entityID,
		},
	}
	session := &saml.Session{}

	err := maker.MakeAssertion(req, session)
	if err == nil {
		t.Fatal("Expected error on SP lookup failure, got nil")
	}
}
