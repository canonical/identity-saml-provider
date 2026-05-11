package logging

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ZapLogger wraps *zap.SugaredLogger to satisfy the Logger interface.
// The only purpose of this wrapper is to return Logger (not
// *zap.SugaredLogger) from With().
type ZapLogger struct {
	*zap.SugaredLogger
}

// NewZapLogger creates a Logger from a *zap.SugaredLogger.
func NewZapLogger(s *zap.SugaredLogger) *ZapLogger {
	return &ZapLogger{SugaredLogger: s}
}

// With returns a new Logger with the given key-value pairs attached
// to every subsequent log line.
func (z *ZapLogger) With(args ...interface{}) Logger {
	return &ZapLogger{SugaredLogger: z.SugaredLogger.With(args...)}
}

// NewNopLogger creates a no-op Logger that discards all output.
// Useful for CLI commands where service-layer logs should be
// suppressed, and for tests that don't care about log output.
func NewNopLogger() Logger {
	return NewZapLogger(zap.NewNop().Sugar())
}

// Sync flushes any buffered log entries. It is safe to call on
// shutdown; callers may ignore the returned error because Sync
// returns a benign ENOTTY on stdout/stderr under Linux.
func (z *ZapLogger) Sync() error {
	return z.SugaredLogger.Sync()
}

// BuildLogger constructs a production-ready Logger for the given
// level string. Accepted values: "debug", "info", "warn", "error".
// When level is "debug", a development config is used (human-readable
// output, stack traces on warn+). All other levels use production
// config (JSON output).
//
// Returns the Logger and any build error. Call Sync() on the returned
// logger to flush buffered entries before exit.
func BuildLogger(level string) (*ZapLogger, error) {
	zapCfg := zap.NewProductionConfig()
	if level == "debug" {
		zapCfg = zap.NewDevelopmentConfig()
	}

	parsed, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", level, err)
	}
	zapCfg.Level = zap.NewAtomicLevelAt(parsed)

	zapLogger, err := zapCfg.Build()
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}

	return NewZapLogger(zapLogger.Sugar()), nil
}
