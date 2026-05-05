package samlkit_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/canonical/identity-saml-provider/internal/infrastructure/samlkit"
)

func TestLoadKeyPair(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) (certPath, keyPath string)
		wantErr  bool
		errMatch string
	}{
		{
			name: "valid RSA key pair",
			setup: func(t *testing.T) (string, string) {
				t.Helper()
				return generateTestRSAKeyPair(t)
			},
		},
		{
			name: "empty cert path",
			setup: func(_ *testing.T) (string, string) {
				return "", "/some/key.pem"
			},
			wantErr:  true,
			errMatch: "certificate path must not be empty",
		},
		{
			name: "empty key path",
			setup: func(_ *testing.T) (string, string) {
				return "/some/cert.pem", ""
			},
			wantErr:  true,
			errMatch: "key path must not be empty",
		},
		{
			name: "cert file does not exist",
			setup: func(_ *testing.T) (string, string) {
				return "/nonexistent/cert.pem", "/nonexistent/key.pem"
			},
			wantErr:  true,
			errMatch: "failed to load key pair",
		},
		{
			name: "invalid cert content",
			setup: func(t *testing.T) (string, string) {
				t.Helper()
				tmpDir := t.TempDir()
				certPath := filepath.Join(tmpDir, "cert.pem")
				keyPath := filepath.Join(tmpDir, "key.pem")
				if err := os.WriteFile(certPath, []byte("not a cert"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(keyPath, []byte("not a key"), 0o600); err != nil {
					t.Fatal(err)
				}
				return certPath, keyPath
			},
			wantErr:  true,
			errMatch: "failed to load key pair",
		},
		{
			name: "non-RSA key (ECDSA) is rejected",
			setup: func(t *testing.T) (string, string) {
				t.Helper()
				return generateTestECDSAKeyPair(t)
			},
			wantErr:  true,
			errMatch: "not RSA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certPath, keyPath := tt.setup(t)

			kp, err := samlkit.LoadKeyPair(certPath, keyPath)
			if tt.wantErr {
				if err == nil {
					t.Fatal("LoadKeyPair() expected error, got nil")
				}
				if tt.errMatch != "" && !containsString(err.Error(), tt.errMatch) {
					t.Errorf("LoadKeyPair() error = %q, want to contain %q", err.Error(), tt.errMatch)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadKeyPair() unexpected error: %v", err)
			}
			if kp == nil {
				t.Fatal("LoadKeyPair() returned nil")
			}
			if kp.Certificate == nil {
				t.Error("LoadKeyPair() Certificate is nil")
			}
			if kp.PrivateKey == nil {
				t.Error("LoadKeyPair() PrivateKey is nil")
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// generateTestRSAKeyPair creates a temporary RSA cert+key pair and returns their paths.
func generateTestRSAKeyPair(t *testing.T) (certPath, keyPath string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	tmpDir := t.TempDir()
	certPath = filepath.Join(tmpDir, "cert.pem")
	keyPath = filepath.Join(tmpDir, "key.pem")

	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("failed to write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}

	return certPath, keyPath
}

// generateTestECDSAKeyPair creates a temporary ECDSA cert+key pair and returns their paths.
func generateTestECDSAKeyPair(t *testing.T) (certPath, keyPath string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ECDSA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-ecdsa"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal ECDSA key: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	tmpDir := t.TempDir()
	certPath = filepath.Join(tmpDir, "cert.pem")
	keyPath = filepath.Join(tmpDir, "key.pem")

	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("failed to write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}

	return certPath, keyPath
}
