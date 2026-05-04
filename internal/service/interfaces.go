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
}

// MappingService handles per-SP attribute mapping logic.
// This encapsulates the current applyAttributeMapping / buildInternalModel
// functions from provider/mapping.go into a testable service.
type MappingService interface {
	// ApplyMapping applies per-SP attribute mapping to a session.
	// If the SP has no mapping configured, the session is returned unmodified.
	// The entityID is used to look up the SP's mapping configuration.
	ApplyMapping(ctx context.Context, session *domain.Session, entityID string) (*domain.Session, error)
}

// OIDCService handles OIDC authentication flows with the identity provider.
type OIDCService interface {
	AuthCodeURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*OIDCClaims, error)
}

// PendingRequestService manages in-flight SAML AuthnRequests awaiting
// OIDC authentication completion.
type PendingRequestService interface {
	Store(ctx context.Context, req *domain.PendingAuthnRequest) error
	Retrieve(ctx context.Context, requestID string) (*domain.PendingAuthnRequest, error)
}

// OIDCClaims represents user claims extracted from an OIDC ID token.
type OIDCClaims struct {
	Sub       string
	Email     string
	Name      string
	Groups    []string
	RawClaims map[string]interface{} // All claims from the OIDC ID token (for per-SP mapping)
}

// OIDCTokenVerifier abstracts OIDC ID token verification for testability.
type OIDCTokenVerifier interface {
	Verify(ctx context.Context, rawIDToken string) (OIDCIDToken, error)
}

// OIDCIDToken abstracts the verified ID token for claims extraction.
type OIDCIDToken interface {
	Claims(v interface{}) error
}
