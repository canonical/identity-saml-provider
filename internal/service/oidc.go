// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package service

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/codes"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/tracing"
)

type oidcService struct {
	hydra  HydraClient
	logger logging.Logger
	tracer tracing.TracingInterface
}

// NewOIDCService creates a new OIDCService backed by the given HydraClient.
func NewOIDCService(hydra HydraClient, logger logging.Logger, tracer tracing.TracingInterface) OIDCService {
	return &oidcService{
		hydra:  hydra,
		logger: logger,
		tracer: tracer,
	}
}

func (s *oidcService) AuthCodeURL(state, nonce string) string {
	return s.hydra.AuthCodeURL(state, nonce)
}

func (s *oidcService) ExchangeCode(ctx context.Context, code, expectedNonce string) (*OIDCClaims, error) {
	ctx, span := s.tracer.Start(ctx, "service.oidc.exchange_code")
	defer span.End()

	logger := logging.FromContext(ctx, s.logger)

	idToken, err := s.hydra.ExchangeCode(ctx, code, expectedNonce)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.Errorw("Token exchange failed", "error", err)

		// Preserve ErrAuthentication (e.g. nonce mismatch) so the
		// handler maps it to 403, not 502.
		var authErr *domain.ErrAuthentication
		if errors.As(err, &authErr) {
			return nil, err
		}

		return nil, &domain.ErrUpstream{
			Service: "hydra",
			Err:     fmt.Errorf("token exchange: %w", err),
		}
	}

	claims := &OIDCClaims{
		Sub:       idToken.Subject,
		Email:     claimString(idToken.Claims, "email"),
		Name:      claimString(idToken.Claims, "name"),
		Groups:    claimStringSlice(idToken.Claims, "groups"),
		RawClaims: idToken.Claims,
	}

	logger.Infow("OIDC code exchange successful", "sub", claims.Sub)
	logger.Debugw("OIDC claims detail", "sub", claims.Sub, "email", claims.Email)

	return claims, nil
}

// claimString extracts a string claim from the raw claims map.
// Returns empty string if missing or not a string.
func claimString(claims map[string]interface{}, key string) string {
	v, _ := claims[key].(string)
	return v
}

// claimStringSlice extracts a string slice claim from the raw claims
// map. Returns nil if missing or not a slice.
func claimStringSlice(claims map[string]interface{}, key string) []string {
	arr, ok := claims[key].([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
