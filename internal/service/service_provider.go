package service

import (
	"context"
	"fmt"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/repository"
)

type serviceProviderService struct {
	repo   repository.ServiceProviderRepository
	logger logging.Logger
}

// NewServiceProviderService creates a new ServiceProviderService.
func NewServiceProviderService(repo repository.ServiceProviderRepository, logger logging.Logger) ServiceProviderService {
	return &serviceProviderService{repo: repo, logger: logger}
}

func (s *serviceProviderService) Register(ctx context.Context, sp *domain.ServiceProvider) error {
	// Validate attribute mapping if present
	if sp.AttributeMapping != nil {
		if err := sp.AttributeMapping.Validate(); err != nil {
			return err
		}
	}

	if err := s.repo.Save(ctx, sp); err != nil {
		s.logger.Errorw("Failed to register service provider", "entityID", sp.EntityID, "error", err)
		return fmt.Errorf("register service provider: %w", err)
	}

	s.logger.Infow("Service provider registered", "entityID", sp.EntityID)
	return nil
}

func (s *serviceProviderService) GetByEntityID(ctx context.Context, entityID string) (*domain.ServiceProvider, error) {
	sp, err := s.repo.GetByEntityID(ctx, entityID)
	if err != nil {
		return nil, err // propagates *domain.ErrNotFound or infrastructure error
	}
	return sp, nil
}
