package samlkit

import (
	"fmt"
	"net/url"

	"github.com/crewjam/saml"
	"go.uber.org/zap"
)

// Config holds the configuration needed to create a SAML Identity Provider.
type Config struct {
	BridgeBaseURL string
	CertPath      string
	KeyPath       string
}

// NewIdentityProvider creates a *saml.IdentityProvider from the given config
// and zap logger. It loads the SAML signing key pair and configures the IdP
// URLs. The caller must set SessionProvider and ServiceProviderProvider on the
// returned IdP after wiring services.
func NewIdentityProvider(cfg Config, zapLogger *zap.Logger) (*saml.IdentityProvider, error) {
	kp, err := LoadKeyPair(cfg.CertPath, cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("load SAML key pair: %w", err)
	}

	ssoURL, err := url.Parse(cfg.BridgeBaseURL + "/saml/sso")
	if err != nil {
		return nil, fmt.Errorf("parse SSO URL: %w", err)
	}

	metadataURL, err := url.Parse(cfg.BridgeBaseURL + "/saml/metadata")
	if err != nil {
		return nil, fmt.Errorf("parse metadata URL: %w", err)
	}

	idp := &saml.IdentityProvider{
		Key:         kp.PrivateKey,
		Certificate: kp.Certificate,
		Logger:      NewZapLoggerAdapter(zapLogger),
		SSOURL:      *ssoURL,
		MetadataURL: *metadataURL,
	}

	return idp, nil
}
