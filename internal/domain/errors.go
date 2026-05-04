package domain

import "fmt"

// ErrNotFound indicates the requested resource does not exist.
type ErrNotFound struct {
	Resource string // e.g. "session", "service_provider"
	ID       string // the lookup key
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

// ErrConflict indicates a uniqueness constraint violation.
type ErrConflict struct {
	Resource string
	ID       string
}

func (e *ErrConflict) Error() string {
	return fmt.Sprintf("%s already exists: %s", e.Resource, e.ID)
}

// ErrValidation represents an input validation failure.
type ErrValidation struct {
	Field   string
	Message string
}

func (e *ErrValidation) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// ErrAuthentication represents an authentication failure.
type ErrAuthentication struct {
	Reason string
}

func (e *ErrAuthentication) Error() string {
	return fmt.Sprintf("authentication failed: %s", e.Reason)
}

// ErrUpstream represents a failure in an upstream service (e.g. Hydra).
// It wraps the underlying error for use with errors.Unwrap / errors.Is.
type ErrUpstream struct {
	Service string
	Err     error
}

func (e *ErrUpstream) Error() string {
	return fmt.Sprintf("upstream %s error: %v", e.Service, e.Err)
}

// Unwrap returns the underlying error, supporting errors.Is and
// errors.As through the error chain.
func (e *ErrUpstream) Unwrap() error {
	return e.Err
}
