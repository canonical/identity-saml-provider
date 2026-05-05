package hydra_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/canonical/identity-saml-provider/internal/infrastructure/hydra"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name      string
		cfg       hydra.Config
		setupFile func(t *testing.T) string // returns path to temp file if needed
		wantErr   bool
	}{
		{
			name: "default config produces a valid client",
			cfg:  hydra.Config{},
		},
		{
			name: "insecure skip TLS verify",
			cfg:  hydra.Config{InsecureSkipTLSVerify: true},
		},
		{
			name: "custom CA cert file",
			cfg:  hydra.Config{}, // CACertPath set by setupFile
			setupFile: func(t *testing.T) string {
				t.Helper()
				return writeTestCACert(t)
			},
		},
		{
			name:    "CA cert file not found",
			cfg:     hydra.Config{CACertPath: "/nonexistent/path/ca.pem"},
			wantErr: true,
		},
		{
			name: "invalid CA cert PEM",
			cfg:  hydra.Config{}, // CACertPath set by setupFile
			setupFile: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				p := filepath.Join(tmpDir, "invalid.pem")
				if err := os.WriteFile(p, []byte("not valid PEM data"), 0o600); err != nil {
					t.Fatalf("failed to write invalid cert: %v", err)
				}
				return p
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			if tt.setupFile != nil {
				cfg.CACertPath = tt.setupFile(t)
			}

			client, err := hydra.NewClient(cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("NewClient() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewClient() unexpected error: %v", err)
			}
			if client == nil {
				t.Fatal("NewClient() returned nil client")
			}
			if client.Timeout != 30*time.Second {
				t.Errorf("NewClient() timeout = %v, want 30s", client.Timeout)
			}
		})
	}
}

// writeTestCACert generates a self-signed CA certificate and writes it to a temp file.
func writeTestCACert(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test CA"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "ca.pem")
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("failed to write cert: %v", err)
	}

	return certPath
}
