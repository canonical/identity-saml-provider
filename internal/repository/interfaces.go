package repository

import (
	"context"

	"github.com/canonical/identity-saml-provider/internal/domain"
)

//go:generate mockgen -destination=../../mocks/mock_session_repository.go -package=mocks . SessionRepository
//go:generate mockgen -destination=../../mocks/mock_service_provider_repository.go -package=mocks . ServiceProviderRepository
//go:generate mockgen -destination=../../mocks/mock_pending_request_repository.go -package=mocks . PendingRequestRepository

// SessionRepository manages session persistence.
type SessionRepository interface {
	Save(ctx context.Context, session *domain.Session) error
	GetByID(ctx context.Context, id string) (*domain.Session, error)
	DeleteExpired(ctx context.Context) (int64, error)
}

// ServiceProviderRepository manages service provider persistence.
type ServiceProviderRepository interface {
	Save(ctx context.Context, sp *domain.ServiceProvider) error
	GetByEntityID(ctx context.Context, entityID string) (*domain.ServiceProvider, error)
	GetAttributeMapping(ctx context.Context, entityID string) (*domain.AttributeMapping, error)
}

// PendingRequestRepository manages in-flight SAML AuthnRequests.
// This replaces the in-memory map in the current Server struct.
type PendingRequestRepository interface {
	Save(ctx context.Context, req *domain.PendingAuthnRequest) error
	GetAndDelete(ctx context.Context, requestID string) (*domain.PendingAuthnRequest, error)
	DeleteExpired(ctx context.Context) (int64, error)
}
