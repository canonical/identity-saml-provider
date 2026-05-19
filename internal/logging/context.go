// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package logging

import "context"

type ctxKey struct{}

// WithLogger returns a new context with the given Logger stored as a value.
// Used by HTTP middleware to attach a request-enriched logger.
func WithLogger(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext retrieves the Logger from the context. If no logger is
// present, it returns the fallback logger. This ensures nil-safe
// operation in tests, CLI commands, and background jobs where
// middleware is not applied.
func FromContext(ctx context.Context, fallback Logger) Logger {
	if l, ok := ctx.Value(ctxKey{}).(Logger); ok {
		return l
	}
	return fallback
}
