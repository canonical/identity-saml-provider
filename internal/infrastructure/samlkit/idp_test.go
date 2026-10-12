// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package samlkit_test

import (
	"strings"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/infrastructure/samlkit"
	"github.com/canonical/identity-saml-provider/internal/logging"
	dsig "github.com/russellhaering/goxmldsig"
	"go.uber.org/zap/zaptest"
)

func TestNewIdentityProvider_Success(t *testing.T) {
	certPath, keyPath := generateTestRSAKeyPair(t)
	zapLogger := logging.NewZapLogger(zaptest.NewLogger(t).Sugar())

	cfg := samlkit.Config{
		BridgeBaseURL:   "http://localhost:8082",
		CertPath:        certPath,
		KeyPath:         keyPath,
		SignatureMethod: dsig.RSASHA384SignatureMethod,
	}

	idp, err := samlkit.NewIdentityProvider(cfg, zapLogger)
	if err != nil {
		t.Fatalf("NewIdentityProvider() unexpected error: %v", err)
	}

	if idp == nil {
		t.Fatal("NewIdentityProvider() returned nil")
	}

	if idp.Key == nil {
		t.Error("idp.Key is nil")
	}
	if idp.Certificate == nil {
		t.Error("idp.Certificate is nil")
	}
	if idp.SignatureMethod != dsig.RSASHA384SignatureMethod {
		t.Errorf("idp.SignatureMethod = %q, want %q", idp.SignatureMethod, dsig.RSASHA384SignatureMethod)
	}
	if got := idp.SSOURL.String(); got != "http://localhost:8082/saml/sso" {
		t.Errorf("idp.SSOURL = %q, want %q", got, "http://localhost:8082/saml/sso")
	}
	if got := idp.MetadataURL.String(); got != "http://localhost:8082/saml/metadata" {
		t.Errorf("idp.MetadataURL = %q, want %q", got, "http://localhost:8082/saml/metadata")
	}
}

func TestNewIdentityProvider_DefaultSignatureMethod(t *testing.T) {
	certPath, keyPath := generateTestRSAKeyPair(t)
	zapLogger := logging.NewZapLogger(zaptest.NewLogger(t).Sugar())

	cfg := samlkit.Config{
		BridgeBaseURL:   "http://localhost:8082",
		CertPath:        certPath,
		KeyPath:         keyPath,
		SignatureMethod: "",
	}

	idp, err := samlkit.NewIdentityProvider(cfg, zapLogger)
	if err != nil {
		t.Fatalf("NewIdentityProvider() unexpected error: %v", err)
	}

	if idp.SignatureMethod != dsig.RSASHA256SignatureMethod {
		t.Errorf("idp.SignatureMethod = %q, want %q", idp.SignatureMethod, dsig.RSASHA256SignatureMethod)
	}
}

func TestNewIdentityProvider_SupportedSignatureMethods(t *testing.T) {
	methods := []string{
		dsig.RSASHA256SignatureMethod,
		dsig.RSASHA384SignatureMethod,
		dsig.RSASHA512SignatureMethod,
		dsig.RSASHA1SignatureMethod,
	}

	certPath, keyPath := generateTestRSAKeyPair(t)
	zapLogger := logging.NewZapLogger(zaptest.NewLogger(t).Sugar())

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			cfg := samlkit.Config{
				BridgeBaseURL:   "http://localhost:8082",
				CertPath:        certPath,
				KeyPath:         keyPath,
				SignatureMethod: method,
			}

			idp, err := samlkit.NewIdentityProvider(cfg, zapLogger)
			if err != nil {
				t.Fatalf("NewIdentityProvider() unexpected error: %v", err)
			}
			if idp.SignatureMethod != method {
				t.Errorf("idp.SignatureMethod = %q, want %q", idp.SignatureMethod, method)
			}
		})
	}
}

func TestNewIdentityProvider_InvalidKeyPair(t *testing.T) {
	zapLogger := logging.NewZapLogger(zaptest.NewLogger(t).Sugar())

	cfg := samlkit.Config{
		BridgeBaseURL: "http://localhost:8082",
		CertPath:      "/nonexistent/cert.pem",
		KeyPath:       "/nonexistent/key.pem",
	}

	_, err := samlkit.NewIdentityProvider(cfg, zapLogger)
	if err == nil {
		t.Fatal("NewIdentityProvider() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "load SAML key pair") {
		t.Errorf("error = %q, want substring 'load SAML key pair'", err.Error())
	}
}

func TestNewIdentityProvider_InvalidURL(t *testing.T) {
	certPath, keyPath := generateTestRSAKeyPair(t)
	zapLogger := logging.NewZapLogger(zaptest.NewLogger(t).Sugar())

	cfg := samlkit.Config{
		BridgeBaseURL: "http://localhost:8082\x7f",
		CertPath:      certPath,
		KeyPath:       keyPath,
	}

	_, err := samlkit.NewIdentityProvider(cfg, zapLogger)
	if err == nil {
		t.Fatal("NewIdentityProvider() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parse SSO URL") {
		t.Errorf("error = %q, want substring 'parse SSO URL'", err.Error())
	}
}
