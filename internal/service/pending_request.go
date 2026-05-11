package service

import (
	"context"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/repository"
)

type pendingRequestService struct {
	repo   repository.PendingRequestRepository
	logger logging.Logger
}

// NewPendingRequestService creates a new PendingRequestService.
func NewPendingRequestService(repo repository.PendingRequestRepository, logger logging.Logger) PendingRequestService {
	return &pendingRequestService{repo: repo, logger: logger}
}

func (s *pendingRequestService) Store(ctx context.Context, req *domain.PendingAuthnRequest) error {
	logger := logging.FromContext(ctx, s.logger)

	if err := s.repo.Save(ctx, req); err != nil {
		logger.Errorw("Failed to store pending request", "requestID", req.RequestID, "error", err)
		return err
	}
	logger.Debugw("Pending request stored", "requestID", req.RequestID)
	return nil
}

func (s *pendingRequestService) Retrieve(ctx context.Context, requestID string) (*domain.PendingAuthnRequest, error) {
	logger := logging.FromContext(ctx, s.logger)

	req, err := s.repo.GetAndDelete(ctx, requestID)
	if err != nil {
		return nil, err // propagates *domain.ErrNotFound
	}
	logger.Debugw("Pending request retrieved", "requestID", requestID)
	return req, nil
}
