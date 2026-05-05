package provider

import (
	"github.com/canonical/identity-saml-provider/internal/infrastructure/samlkit"
	"go.uber.org/zap"
)

// Deprecated: ZapLoggerAdapter is deprecated. Use samlkit.ZapLoggerAdapter instead.
// This type alias is kept for backward compatibility during the refactoring migration.
type ZapLoggerAdapter = samlkit.ZapLoggerAdapter

// Deprecated: NewZapStdLogger is deprecated. Use samlkit.NewZapLoggerAdapter instead.
// This function is kept for backward compatibility during the refactoring migration.
func NewZapStdLogger(zapLogger *zap.Logger) *ZapLoggerAdapter {
	return samlkit.NewZapLoggerAdapter(zapLogger)
}
