package service

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
)

// idTokenVerifierAdapter adapts *oidc.IDTokenVerifier to the OIDCTokenVerifier interface.
type idTokenVerifierAdapter struct {
	verifier *oidc.IDTokenVerifier
}

// NewIDTokenVerifierAdapter wraps an *oidc.IDTokenVerifier so that it
// satisfies the OIDCTokenVerifier interface used by the service layer.
func NewIDTokenVerifierAdapter(v *oidc.IDTokenVerifier) OIDCTokenVerifier {
	return &idTokenVerifierAdapter{verifier: v}
}

func (a *idTokenVerifierAdapter) Verify(ctx context.Context, rawIDToken string) (OIDCIDToken, error) {
	return a.verifier.Verify(ctx, rawIDToken)
}
