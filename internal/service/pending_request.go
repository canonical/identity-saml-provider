package service

import (
	"context"

	"go.opentelemetry.io/otel/codes"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/repository"
	"github.com/canonical/identity-saml-provider/internal/tracing"
)

type pendingRequestService struct {
	repo   repository.PendingRequestRepository
	logger logging.Logger
	tracer tracing.TracingInterface
}

// NewPendingRequestService creates a new PendingRequestService.
func NewPendingRequestService(repo repository.PendingRequestRepository, logger logging.Logger, tracer tracing.TracingInterface) PendingRequestService {
	return &pendingRequestService{repo: repo, logger: logger, tracer: tracer}
}

func (s *pendingRequestService) Store(ctx context.Context, req *domain.PendingAuthnRequest) error {
	ctx, span := s.tracer.Start(ctx, "service.pending.store")
	defer span.End()

	logger := logging.FromContext(ctx, s.logger)

	if err := s.repo.Save(ctx, req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.Errorw("Failed to store pending request", "requestID", req.RequestID, "error", err)
		return err
	}
	logger.Debugw("Pending request stored", "requestID", req.RequestID)
	return nil
}

func (s *pendingRequestService) Retrieve(ctx context.Context, requestID string) (*domain.PendingAuthnRequest, error) {
	ctx, span := s.tracer.Start(ctx, "service.pending.retrieve")
	defer span.End()

	logger := logging.FromContext(ctx, s.logger)

	req, err := s.repo.GetAndDelete(ctx, requestID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err // propagates *domain.ErrNotFound
	}
	logger.Debugw("Pending request retrieved", "requestID", requestID)
	return req, nil
}
