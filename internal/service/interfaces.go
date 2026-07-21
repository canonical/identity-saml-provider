// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package service

import (
	"context"

	"github.com/canonical/identity-saml-provider/internal/domain"
)

//go:generate mockgen -destination=../../mocks/mock_session_service.go -package=mocks . SessionService
//go:generate mockgen -destination=../../mocks/mock_service_provider_service.go -package=mocks . ServiceProviderService
//go:generate mockgen -destination=../../mocks/mock_mapping_service.go -package=mocks . MappingService
//go:generate mockgen -destination=../../mocks/mock_oidc_service.go -package=mocks . OIDCService
//go:generate mockgen -destination=../../mocks/mock_pending_request_service.go -package=mocks . PendingRequestService
//go:generate mockgen -destination=../../mocks/mock_hydra_client.go -package=mocks . HydraClient

// SessionService manages user session lifecycle.
type SessionService interface {
	CreateFromOIDC(ctx context.Context, claims *OIDCClaims) (*domain.Session, error)
	GetByID(ctx context.Context, id string) (*domain.Session, error)
	CleanupExpired(ctx context.Context) (int64, error)
}

// ServiceProviderService manages SAML service provider registration and lookup.
type ServiceProviderService interface {
	Register(ctx context.Context, sp *domain.ServiceProvider) error
	GetByEntityID(ctx context.Context, entityID string) (*domain.ServiceProvider, error)

	// UpdateAttributeMapping replaces the SP's attribute mapping with
	// the validated mapping argument. A nil mapping is rejected with
	// *domain.ErrValidation; use ClearAttributeMapping to remove a
	// mapping. *domain.ErrNotFound is returned if the SP is unknown.
	UpdateAttributeMapping(ctx context.Context, entityID string, mapping *domain.AttributeMapping) error

	// ClearAttributeMapping removes the SP's attribute mapping,
	// reverting the SP to default (unmapped) assertion behaviour.
	// *domain.ErrNotFound is returned if the SP is unknown.
	ClearAttributeMapping(ctx context.Context, entityID string) error
}

// MappingService applies per-SP attribute mapping to a session,
// translating OIDC claims into the SAML NameID and assertion
// attributes the configured service provider expects.
type MappingService interface {
	// ApplyMapping applies per-SP attribute mapping to a session.
	// If the SP has no mapping configured, the session is returned unmodified.
	// The entityID is used to look up the SP's mapping configuration.
	ApplyMapping(ctx context.Context, session *domain.Session, entityID string) (*domain.Session, error)
}

// OIDCService handles OIDC authentication flows with the identity provider.
type OIDCService interface {
	AuthCodeURL(state, nonce string) string
	ExchangeCode(ctx context.Context, code, expectedNonce string) (*OIDCClaims, error)
}

// PendingRequestService manages in-flight SAML AuthnRequests awaiting
// OIDC authentication completion.
type PendingRequestService interface {
	Store(ctx context.Context, req *domain.PendingAuthnRequest) error
	Retrieve(ctx context.Context, requestID string) (*domain.PendingAuthnRequest, error)
	CleanupExpired(ctx context.Context, limit int) (int64, error)
}

// HydraClient abstracts the Hydra OIDC infrastructure for auth URL
// generation and token exchange.
type HydraClient interface {
	AuthCodeURL(state, nonce string) string
	ExchangeCode(ctx context.Context, code, expectedNonce string) (*domain.IDToken, error)
}

// OIDCClaims represents user claims extracted from an OIDC ID token.
type OIDCClaims struct {
	Sub       string
	Email     string
	Name      string
	Groups    []string
	RawClaims map[string]interface{} // All claims from the OIDC ID token (for per-SP mapping)
}
