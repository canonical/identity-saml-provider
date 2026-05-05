package samlkit_test

import (
	"testing"

	"github.com/canonical/identity-saml-provider/internal/infrastructure/samlkit"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestNewZapLoggerAdapter(t *testing.T) {
	zapLogger := zaptest.NewLogger(t)

	adapter := samlkit.NewZapLoggerAdapter(zapLogger)
	if adapter == nil {
		t.Fatal("NewZapLoggerAdapter() returned nil")
	}

	// Verify that the adapter implements the expected interface methods
	// by calling non-fatal methods without panicking.
	adapter.Print("test print")
	adapter.Println("test println")
	adapter.Printf("test %s", "printf")
}

func TestZapLoggerAdapter_PrintMethods(t *testing.T) {
	// Use a development logger that won't exit on Fatal
	zapLogger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))

	adapter := samlkit.NewZapLoggerAdapter(zapLogger)

	// These should not panic
	adapter.Print("info message")
	adapter.Println("info message line")
	adapter.Printf("formatted %s %d", "message", 42)
}
