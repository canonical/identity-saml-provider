package tracing

import "context"

// NewNoopConfig returns a Config with tracing disabled.
// Useful for tests and CLI commands that don't need tracing.
func NewNoopConfig() *Config {
	return &Config{}
}

// NewNoopTracer returns a Tracer backed by a no-op provider.
// Useful for tests and CLI commands that don't need tracing.
func NewNoopTracer() *Tracer {
	return NewTracer(context.Background(), NewNoopConfig())
}
