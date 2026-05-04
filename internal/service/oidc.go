package service

import (
	"context"
	"fmt"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"golang.org/x/oauth2"
)

type oidcService struct {
	oauth2Config *oauth2.Config
	verifier     OIDCTokenVerifier
	logger       logging.Logger
}

// NewOIDCService creates a new OIDCService with the given OAuth2 config and token verifier.
func NewOIDCService(oauth2Config *oauth2.Config, verifier OIDCTokenVerifier, logger logging.Logger) OIDCService {
	return &oidcService{
		oauth2Config: oauth2Config,
		verifier:     verifier,
		logger:       logger,
	}
}

func (s *oidcService) AuthCodeURL(state string) string {
	return s.oauth2Config.AuthCodeURL(state)
}

func (s *oidcService) ExchangeCode(ctx context.Context, code string) (*OIDCClaims, error) {
	// Exchange the authorization code for tokens
	token, err := s.oauth2Config.Exchange(ctx, code)
	if err != nil {
		s.logger.Errorw("Token exchange failed", "error", err)
		return nil, &domain.ErrUpstream{Service: "hydra", Err: fmt.Errorf("token exchange: %w", err)}
	}

	// Extract the raw ID token from the OAuth2 token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, &domain.ErrUpstream{Service: "hydra", Err: fmt.Errorf("no id_token in token response")}
	}

	// Verify the ID token
	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		s.logger.Errorw("ID token verification failed", "error", err)
		return nil, &domain.ErrAuthentication{Reason: fmt.Sprintf("invalid ID token: %v", err)}
	}

	// Extract standard claims
	var standardClaims struct {
		Sub    string   `json:"sub"`
		Email  string   `json:"email"`
		Name   string   `json:"name"`
		Groups []string `json:"groups"`
	}
	if err := idToken.Claims(&standardClaims); err != nil {
		return nil, &domain.ErrAuthentication{Reason: fmt.Sprintf("failed to parse claims: %v", err)}
	}

	// Extract all raw claims for per-SP mapping
	var rawClaims map[string]interface{}
	if err := idToken.Claims(&rawClaims); err != nil {
		s.logger.Warnw("Failed to extract raw claims", "error", err)
		// Non-fatal: proceed without raw claims
	}

	claims := &OIDCClaims{
		Sub:       standardClaims.Sub,
		Email:     standardClaims.Email,
		Name:      standardClaims.Name,
		Groups:    standardClaims.Groups,
		RawClaims: rawClaims,
	}

	s.logger.Infow("OIDC code exchange successful", "sub", claims.Sub, "email", claims.Email)
	return claims, nil
}
