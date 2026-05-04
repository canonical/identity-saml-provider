package logging

//go:generate mockgen -destination=../../mocks/mock_logger.go -package=mocks . Logger

// Logger defines the structured logging interface used across all layers.
// *zap.SugaredLogger satisfies this interface natively — no adapter needed.
type Logger interface {
	Infow(msg string, keysAndValues ...interface{})
	Warnw(msg string, keysAndValues ...interface{})
	Errorw(msg string, keysAndValues ...interface{})
	Debugw(msg string, keysAndValues ...interface{})
}
