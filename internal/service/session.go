// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/codes"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/repository"
	"github.com/canonical/identity-saml-provider/internal/tracing"
)

type sessionService struct {
	repo   repository.SessionRepository
	logger logging.Logger
	tracer tracing.TracingInterface
}

// NewSessionService creates a new SessionService backed by the given repository.
func NewSessionService(repo repository.SessionRepository, logger logging.Logger, tracer tracing.TracingInterface) SessionService {
	return &sessionService{repo: repo, logger: logger, tracer: tracer}
}

// generateSessionID produces a cryptographically secure session ID with
// 256 bits of entropy. The "_" prefix ensures compliance with the XML
// xs:NCName type used by the SAML SessionIndex attribute.
func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session ID: %w", err)
	}
	return "_" + base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *sessionService) CreateFromOIDC(ctx context.Context, claims *OIDCClaims) (*domain.Session, error) {
	ctx, span := s.tracer.Start(ctx, "service.session.create_from_oidc")
	defer span.End()

	logger := logging.FromContext(ctx, s.logger)

	if claims.Email == "" {
		err := &domain.ErrValidation{Field: "email", Message: "email claim is required"}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Use the name claim if available, otherwise fall back to email
	displayName := claims.Email
	if claims.Name != "" {
		displayName = claims.Name
	}

	sessionID, err := generateSessionID()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.Errorw("Failed to generate session ID", "error", err)
		return nil, fmt.Errorf("create session: %w", err)
	}
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

	// Log only a prefix of the session ID to avoid leaking the full
	// bearer token into log aggregators (OWASP, CWE-532).
	logID := sessionID[:9] // "_" + first 8 chars of base64url

	if err := s.repo.Save(ctx, session); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.Errorw("Failed to save session", "sessionID", logID, "error", err)
		return nil, fmt.Errorf("save session: %w", err)
	}

	logger.Infow("Session created", "sessionID", logID)
	logger.Debugw("Session detail", "sessionID", logID, "email", claims.Email)
	return session, nil
}

func (s *sessionService) GetByID(ctx context.Context, id string) (*domain.Session, error) {
	ctx, span := s.tracer.Start(ctx, "service.session.get_by_id")
	defer span.End()

	session, err := s.repo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err // propagates *domain.ErrNotFound or infrastructure error
	}
	return session, nil
}

func (s *sessionService) CleanupExpired(ctx context.Context) (int64, error) {
	ctx, span := s.tracer.Start(ctx, "service.session.cleanup_expired")
	defer span.End()

	logger := logging.FromContext(ctx, s.logger)

	count, err := s.repo.DeleteExpired(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.Errorw("Failed to cleanup expired sessions", "error", err)
		return 0, err
	}
	if count > 0 {
		logger.Infow("Cleaned up expired sessions", "count", count)
	}
	return count, nil
}
