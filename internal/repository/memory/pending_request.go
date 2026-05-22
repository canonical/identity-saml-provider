// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package memory

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/tracing"
)

// PendingRequestRepo is a thread-safe in-memory implementation of
// repository.PendingRequestRepository.
type PendingRequestRepo struct {
	mu       sync.Mutex
	requests map[string]*domain.PendingAuthnRequest
	now      func() time.Time // clock function for testability
	tracer   tracing.TracingInterface
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
func NewPendingRequestRepo(tracer tracing.TracingInterface, opts ...Option) *PendingRequestRepo {
	r := &PendingRequestRepo{
		requests: make(map[string]*domain.PendingAuthnRequest),
		now:      time.Now,
		tracer:   tracer,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Save stores a pending authentication request.
func (r *PendingRequestRepo) Save(ctx context.Context, req *domain.PendingAuthnRequest) error {
	_, span := r.tracer.Start(ctx, "repo.memory.save_pending")
	defer span.End()
	span.SetAttributes(attribute.String("db.system", "memory"))

	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests[req.RequestID] = req
	return nil
}

// GetAndDelete retrieves and removes a pending request by its ID.
// Returns *domain.ErrNotFound if the request does not exist.
func (r *PendingRequestRepo) GetAndDelete(ctx context.Context, requestID string) (*domain.PendingAuthnRequest, error) {
	_, span := r.tracer.Start(ctx, "repo.memory.get_and_delete_pending")
	defer span.End()
	span.SetAttributes(attribute.String("db.system", "memory"))

	r.mu.Lock()
	defer r.mu.Unlock()
	req, ok := r.requests[requestID]
	if !ok {
		err := &domain.ErrNotFound{Resource: "pending_request", ID: requestID}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	delete(r.requests, requestID)
	return req, nil
}

// DeleteExpired removes entries older than 10 minutes and returns the
// number of deleted entries.
func (r *PendingRequestRepo) DeleteExpired(ctx context.Context) (int64, error) {
	_, span := r.tracer.Start(ctx, "repo.memory.delete_expired_pending")
	defer span.End()
	span.SetAttributes(attribute.String("db.system", "memory"))

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
