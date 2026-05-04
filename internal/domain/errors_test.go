package domain_test

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/domain"
)

func TestErrNotFound_Error(t *testing.T) {
	t.Parallel()

	err := &domain.ErrNotFound{Resource: "session", ID: "abc123"}

	want := "session not found: abc123"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestErrConflict_Error(t *testing.T) {
	t.Parallel()

	err := &domain.ErrConflict{Resource: "service_provider", ID: "sp-1"}

	want := "service_provider already exists: sp-1"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestErrValidation_Error(t *testing.T) {
	t.Parallel()

	err := &domain.ErrValidation{Field: "email", Message: "required"}

	want := "validation error on email: required"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestErrAuthentication_Error(t *testing.T) {
	t.Parallel()

	err := &domain.ErrAuthentication{Reason: "invalid token"}

	want := "authentication failed: invalid token"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestErrUpstream_Error(t *testing.T) {
	t.Parallel()

	cause := fmt.Errorf("connection refused")
	err := &domain.ErrUpstream{Service: "hydra", Err: cause}

	want := "upstream hydra error: connection refused"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestErrUpstream_Unwrap(t *testing.T) {
	t.Parallel()

	cause := io.ErrUnexpectedEOF
	err := &domain.ErrUpstream{Service: "hydra", Err: cause}

	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Error("errors.Is(err, io.ErrUnexpectedEOF) = false, want true")
	}

	unwrapped := errors.Unwrap(err)
	if unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestErrors_As(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		// target will be checked via errors.As
		check func(err error) bool
	}{
		{
			name: "ErrNotFound via errors.As",
			err:  fmt.Errorf("wrapped: %w", &domain.ErrNotFound{Resource: "session", ID: "1"}),
			check: func(err error) bool {
				var target *domain.ErrNotFound
				return errors.As(err, &target) && target.Resource == "session"
			},
		},
		{
			name: "ErrConflict via errors.As",
			err:  fmt.Errorf("wrapped: %w", &domain.ErrConflict{Resource: "sp", ID: "2"}),
			check: func(err error) bool {
				var target *domain.ErrConflict
				return errors.As(err, &target) && target.Resource == "sp"
			},
		},
		{
			name: "ErrValidation via errors.As",
			err:  fmt.Errorf("wrapped: %w", &domain.ErrValidation{Field: "url", Message: "bad"}),
			check: func(err error) bool {
				var target *domain.ErrValidation
				return errors.As(err, &target) && target.Field == "url"
			},
		},
		{
			name: "ErrAuthentication via errors.As",
			err:  fmt.Errorf("wrapped: %w", &domain.ErrAuthentication{Reason: "expired"}),
			check: func(err error) bool {
				var target *domain.ErrAuthentication
				return errors.As(err, &target) && target.Reason == "expired"
			},
		},
		{
			name: "ErrUpstream via errors.As",
			err:  fmt.Errorf("wrapped: %w", &domain.ErrUpstream{Service: "hydra", Err: io.EOF}),
			check: func(err error) bool {
				var target *domain.ErrUpstream
				return errors.As(err, &target) && target.Service == "hydra"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if !tt.check(tt.err) {
				t.Error("errors.As check failed")
			}
		})
	}
}

func TestErrUpstream_ErrorChain(t *testing.T) {
	t.Parallel()

	inner := &domain.ErrNotFound{Resource: "session", ID: "x"}
	outer := &domain.ErrUpstream{Service: "store", Err: inner}

	// errors.Is should not match because ErrNotFound is a struct pointer,
	// but errors.As should unwrap through the chain.
	var target *domain.ErrNotFound
	if !errors.As(outer, &target) {
		t.Fatal("errors.As should find ErrNotFound through ErrUpstream chain")
	}
	if target.ID != "x" {
		t.Errorf("ErrNotFound.ID = %q, want %q", target.ID, "x")
	}
}
