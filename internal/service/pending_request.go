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
	if err := s.repo.Save(ctx, req); err != nil {
		s.logger.Errorw("Failed to store pending request", "requestID", req.RequestID, "error", err)
		return err
	}
	s.logger.Debugw("Pending request stored", "requestID", req.RequestID)
	return nil
}

func (s *pendingRequestService) Retrieve(ctx context.Context, requestID string) (*domain.PendingAuthnRequest, error) {
	req, err := s.repo.GetAndDelete(ctx, requestID)
	if err != nil {
		return nil, err // propagates *domain.ErrNotFound
	}
	s.logger.Debugw("Pending request retrieved", "requestID", requestID)
	return req, nil
}
