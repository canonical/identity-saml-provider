package memory

import (
	"context"
	"sync"
	"time"

	"github.com/canonical/identity-saml-provider/internal/domain"
)

// PendingRequestRepo is a thread-safe in-memory implementation of
// repository.PendingRequestRepository.
type PendingRequestRepo struct {
	mu       sync.Mutex
	requests map[string]*domain.PendingAuthnRequest
	now      func() time.Time // clock function for testability
}

// Option configures a PendingRequestRepo.
type Option func(*PendingRequestRepo)

// WithClock sets a custom clock function for time-based operations.
// This is primarily useful for deterministic testing without time.Sleep.
func WithClock(now func() time.Time) Option {
	return func(r *PendingRequestRepo) {
		r.now = now
	}
}

// NewPendingRequestRepo creates a new in-memory pending request repository.
func NewPendingRequestRepo(opts ...Option) *PendingRequestRepo {
	r := &PendingRequestRepo{
		requests: make(map[string]*domain.PendingAuthnRequest),
		now:      time.Now,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Save stores a pending authentication request.
func (r *PendingRequestRepo) Save(_ context.Context, req *domain.PendingAuthnRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests[req.RequestID] = req
	return nil
}

// GetAndDelete retrieves and removes a pending request by its ID.
// Returns *domain.ErrNotFound if the request does not exist.
func (r *PendingRequestRepo) GetAndDelete(_ context.Context, requestID string) (*domain.PendingAuthnRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	req, ok := r.requests[requestID]
	if !ok {
		return nil, &domain.ErrNotFound{Resource: "pending_request", ID: requestID}
	}
	delete(r.requests, requestID)
	return req, nil
}

// DeleteExpired removes entries older than 10 minutes and returns the
// number of deleted entries.
func (r *PendingRequestRepo) DeleteExpired(_ context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var count int64
	now := r.now()
	for id, req := range r.requests {
		if now.After(req.CreatedAt.Add(10 * time.Minute)) {
			delete(r.requests, id)
			count++
		}
	}
	return count, nil
}
