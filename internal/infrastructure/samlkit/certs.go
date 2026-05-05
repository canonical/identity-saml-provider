package samlkit

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
)

// KeyPair holds the parsed X.509 certificate and RSA private key for SAML signing.
type KeyPair struct {
	Certificate *x509.Certificate
	PrivateKey  *rsa.PrivateKey
}

// LoadKeyPair loads a TLS certificate and key from the given file paths,
// parses the X.509 certificate, and extracts the RSA private key.
// It returns descriptive errors for missing or invalid files.
func LoadKeyPair(certPath, keyPath string) (*KeyPair, error) {
	if certPath == "" {
		return nil, fmt.Errorf("certificate path must not be empty")
	}
	if keyPath == "" {
		return nil, fmt.Errorf("key path must not be empty")
	}

	tlsKeyPair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load key pair (cert=%q, key=%q): %w", certPath, keyPath, err)
	}

	if len(tlsKeyPair.Certificate) == 0 {
		return nil, fmt.Errorf("certificate file %q contains no certificates", certPath)
	}

	x509Cert, err := x509.ParseCertificate(tlsKeyPair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse X.509 certificate from %q: %w", certPath, err)
	}

	rsaKey, ok := tlsKeyPair.PrivateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key in %q is not RSA (SAML requires RSA keys)", keyPath)
	}

	return &KeyPair{
		Certificate: x509Cert,
		PrivateKey:  rsaKey,
	}, nil
}
