package service

import (
	"context"
	"fmt"
	"time"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/repository"
)

type sessionService struct {
	repo   repository.SessionRepository
	logger logging.Logger
}

// NewSessionService creates a new SessionService backed by the given repository.
func NewSessionService(repo repository.SessionRepository, logger logging.Logger) SessionService {
	return &sessionService{repo: repo, logger: logger}
}

func (s *sessionService) CreateFromOIDC(ctx context.Context, claims *OIDCClaims) (*domain.Session, error) {
	if claims.Email == "" {
		return nil, &domain.ErrValidation{Field: "email", Message: "email claim is required"}
	}

	// Use the name claim if available, otherwise fall back to email
	displayName := claims.Email
	if claims.Name != "" {
		displayName = claims.Name
	}

	sessionID := fmt.Sprintf("_%d", time.Now().UnixNano())
	session := &domain.Session{
		ID:             sessionID,
		CreateTime:     time.Now(),
		ExpireTime:     time.Now().Add(10 * time.Minute),
		Index:          sessionID,
		NameID:         claims.Email,
		UserEmail:      claims.Email,
		UserCommonName: displayName,
		UserName:       claims.Sub, // Store OIDC subject for attribute mapping
		Groups:         claims.Groups,
		RawOIDCClaims:  claims.RawClaims, // Store all claims for per-SP mapping
	}

	if err := s.repo.Save(ctx, session); err != nil {
		s.logger.Errorw("Failed to save session", "sessionID", sessionID, "error", err)
		return nil, fmt.Errorf("save session: %w", err)
	}

	s.logger.Infow("Session created", "sessionID", sessionID, "email", claims.Email)
	return session, nil
}

func (s *sessionService) GetByID(ctx context.Context, id string) (*domain.Session, error) {
	session, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err // propagates *domain.ErrNotFound or infrastructure error
	}
	return session, nil
}

func (s *sessionService) CleanupExpired(ctx context.Context) (int64, error) {
	count, err := s.repo.DeleteExpired(ctx)
	if err != nil {
		s.logger.Errorw("Failed to cleanup expired sessions", "error", err)
		return 0, err
	}
	if count > 0 {
		s.logger.Infow("Cleaned up expired sessions", "count", count)
	}
	return count, nil
}
