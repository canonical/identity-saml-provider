package service

import (
	"context"
	"fmt"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
)

type oidcService struct {
	hydra  HydraClient
	logger logging.Logger
}

// NewOIDCService creates a new OIDCService backed by the given HydraClient.
func NewOIDCService(hydra HydraClient, logger logging.Logger) OIDCService {
	return &oidcService{
		hydra:  hydra,
		logger: logger,
	}
}

func (s *oidcService) AuthCodeURL(state string) string {
	return s.hydra.AuthCodeURL(state)
}

func (s *oidcService) ExchangeCode(ctx context.Context, code string) (*OIDCClaims, error) {
	logger := logging.FromContext(ctx, s.logger)

	idToken, err := s.hydra.ExchangeCode(ctx, code)
	if err != nil {
		logger.Errorw("Token exchange failed", "error", err)
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
