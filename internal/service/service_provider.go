package service

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/codes"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/repository"
	"github.com/canonical/identity-saml-provider/internal/tracing"
)

type serviceProviderService struct {
	repo   repository.ServiceProviderRepository
	logger logging.Logger
	tracer tracing.TracingInterface
}

// NewServiceProviderService creates a new ServiceProviderService.
func NewServiceProviderService(repo repository.ServiceProviderRepository, logger logging.Logger, tracer tracing.TracingInterface) ServiceProviderService {
	return &serviceProviderService{repo: repo, logger: logger, tracer: tracer}
}

func (s *serviceProviderService) Register(ctx context.Context, sp *domain.ServiceProvider) error {
	ctx, span := s.tracer.Start(ctx, "service.sp.register")
	defer span.End()

	logger := logging.FromContext(ctx, s.logger)

	// Validate attribute mapping if present
	if sp.AttributeMapping != nil {
		if err := sp.AttributeMapping.Validate(); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
	}

	if err := s.repo.Save(ctx, sp); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.Errorw("Failed to register service provider", "entityID", sp.EntityID, "error", err)
		return fmt.Errorf("register service provider: %w", err)
	}

	logger.Infow("Service provider registered", "entityID", sp.EntityID)
	return nil
}

func (s *serviceProviderService) GetByEntityID(ctx context.Context, entityID string) (*domain.ServiceProvider, error) {
	ctx, span := s.tracer.Start(ctx, "service.sp.get_by_entity_id")
	defer span.End()

	sp, err := s.repo.GetByEntityID(ctx, entityID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err // propagates *domain.ErrNotFound or infrastructure error
	}
	return sp, nil
}
