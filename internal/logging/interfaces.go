package logging

//go:generate mockgen -destination=../../mocks/mock_logger.go -package=mocks . Logger

// Logger defines the structured logging interface used across all layers.
//
// Fatalw is intentionally excluded: it calls os.Exit(1) and should only
// be used at the top-level CLI entrypoint (internal/cmd/serve.go) via
// the concrete *zap.SugaredLogger.
type Logger interface {
	Infow(msg string, keysAndValues ...interface{})
	Warnw(msg string, keysAndValues ...interface{})
	Errorw(msg string, keysAndValues ...interface{})
	Debugw(msg string, keysAndValues ...interface{})
	With(args ...interface{}) Logger
}
