package logging_test

import (
	"context"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/logging"
)

type wrongKey struct{}

func TestFromContext(t *testing.T) {
	t.Parallel()

	base := logging.NewNopLogger()
	enriched := base.With("key", "value")

	tests := []struct {
		name     string
		ctx      context.Context
		fallback logging.Logger
		want     logging.Logger // the logger we expect to get back
	}{
		{
			name:     "returns enriched logger from context",
			ctx:      logging.WithLogger(context.Background(), enriched),
			fallback: base,
			want:     enriched,
		},
		{
			name:     "returns fallback when context has no logger",
			ctx:      context.Background(),
			fallback: base,
			want:     base,
		},
		{
			name:     "returns fallback when context value is wrong type",
			ctx:      context.WithValue(context.Background(), wrongKey{}, "not-a-logger"),
			fallback: base,
			want:     base,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := logging.FromContext(tt.ctx, tt.fallback)
			if got != tt.want {
				t.Errorf("FromContext() returned unexpected logger")
			}
		})
	}
}

func TestWithLogger_RoundTrip(t *testing.T) {
	t.Parallel()

	logger := logging.NewNopLogger()
	ctx := logging.WithLogger(context.Background(), logger)
	got := logging.FromContext(ctx, nil)

	if got != logger {
		t.Error("round-trip: FromContext did not return the logger stored by WithLogger")
	}
}
