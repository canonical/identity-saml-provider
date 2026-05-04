package memory_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/repository/memory"
)

func TestPendingRequestRepo_SaveAndGetAndDelete(t *testing.T) {
	tests := []struct {
		name string
		req  *domain.PendingAuthnRequest
	}{
		{
			name: "basic request with all fields",
			req: &domain.PendingAuthnRequest{
				RequestID:   "req-123",
				SAMLRequest: "<AuthnRequest/>",
				RelayState:  "relay-abc",
				CreatedAt:   time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "request with empty relay state",
			req: &domain.PendingAuthnRequest{
				RequestID:   "req-456",
				SAMLRequest: "<AuthnRequest xmlns='urn:oasis:names:tc:SAML:2.0:protocol'/>",
				RelayState:  "",
				CreatedAt:   time.Date(2026, 6, 15, 8, 30, 0, 0, time.UTC),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := memory.NewPendingRequestRepo()
			ctx := context.Background()

			if err := repo.Save(ctx, tt.req); err != nil {
				t.Fatalf("Save() unexpected error: %v", err)
			}

			got, err := repo.GetAndDelete(ctx, tt.req.RequestID)
			if err != nil {
				t.Fatalf("GetAndDelete() unexpected error: %v", err)
			}
			if got.RequestID != tt.req.RequestID {
				t.Errorf("RequestID = %q, want %q", got.RequestID, tt.req.RequestID)
			}
			if got.SAMLRequest != tt.req.SAMLRequest {
				t.Errorf("SAMLRequest = %q, want %q", got.SAMLRequest, tt.req.SAMLRequest)
			}
			if got.RelayState != tt.req.RelayState {
				t.Errorf("RelayState = %q, want %q", got.RelayState, tt.req.RelayState)
			}
		})
	}
}

func TestPendingRequestRepo_GetAndDelete_RemovesEntry(t *testing.T) {
	repo := memory.NewPendingRequestRepo()
	ctx := context.Background()

	req := &domain.PendingAuthnRequest{
		RequestID:   "req-once",
		SAMLRequest: "<AuthnRequest/>",
		CreatedAt:   time.Now(),
	}

	_ = repo.Save(ctx, req)
	_, _ = repo.GetAndDelete(ctx, "req-once")

	// Second retrieval should fail — entry was consumed.
	_, err := repo.GetAndDelete(ctx, "req-once")
	if err == nil {
		t.Fatal("GetAndDelete() expected error on second call, got nil")
	}

	var notFound *domain.ErrNotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("expected *domain.ErrNotFound, got %T: %v", err, err)
	}
	if notFound.Resource != "pending_request" {
		t.Errorf("Resource = %q, want %q", notFound.Resource, "pending_request")
	}
}

func TestPendingRequestRepo_GetAndDelete_NotFound(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
	}{
		{name: "nonexistent id", requestID: "nonexistent"},
		{name: "empty id", requestID: ""},
		{name: "id with special chars", requestID: "req/special?x=1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := memory.NewPendingRequestRepo()
			ctx := context.Background()

			_, err := repo.GetAndDelete(ctx, tt.requestID)
			if err == nil {
				t.Fatal("GetAndDelete() expected error, got nil")
			}

			var notFound *domain.ErrNotFound
			if !errors.As(err, &notFound) {
				t.Fatalf("expected *domain.ErrNotFound, got %T: %v", err, err)
			}
			if notFound.Resource != "pending_request" {
				t.Errorf("Resource = %q, want %q", notFound.Resource, "pending_request")
			}
			if notFound.ID != tt.requestID {
				t.Errorf("ID = %q, want %q", notFound.ID, tt.requestID)
			}
		})
	}
}

func TestPendingRequestRepo_DeleteExpired(t *testing.T) {
	fixedNow := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		requests      []*domain.PendingAuthnRequest
		wantDeleted   int64
		wantRemaining []string // request IDs that should still exist
		wantGone      []string // request IDs that should be deleted
	}{
		{
			name:          "empty repository",
			requests:      nil,
			wantDeleted:   0,
			wantRemaining: nil,
			wantGone:      nil,
		},
		{
			name: "only expired entries",
			requests: []*domain.PendingAuthnRequest{
				{RequestID: "old-1", SAMLRequest: "<r/>", CreatedAt: fixedNow.Add(-15 * time.Minute)},
				{RequestID: "old-2", SAMLRequest: "<r/>", CreatedAt: fixedNow.Add(-20 * time.Minute)},
			},
			wantDeleted:   2,
			wantRemaining: nil,
			wantGone:      []string{"old-1", "old-2"},
		},
		{
			name: "only fresh entries",
			requests: []*domain.PendingAuthnRequest{
				{RequestID: "new-1", SAMLRequest: "<r/>", CreatedAt: fixedNow.Add(-1 * time.Minute)},
				{RequestID: "new-2", SAMLRequest: "<r/>", CreatedAt: fixedNow.Add(-5 * time.Minute)},
			},
			wantDeleted:   0,
			wantRemaining: []string{"new-1", "new-2"},
			wantGone:      nil,
		},
		{
			name: "mixed expired and fresh",
			requests: []*domain.PendingAuthnRequest{
				{RequestID: "expired", SAMLRequest: "<r/>", CreatedAt: fixedNow.Add(-15 * time.Minute)},
				{RequestID: "fresh", SAMLRequest: "<r/>", CreatedAt: fixedNow.Add(-1 * time.Minute)},
			},
			wantDeleted:   1,
			wantRemaining: []string{"fresh"},
			wantGone:      []string{"expired"},
		},
		{
			name: "boundary exactly 10 minutes is not expired",
			requests: []*domain.PendingAuthnRequest{
				{RequestID: "boundary", SAMLRequest: "<r/>", CreatedAt: fixedNow.Add(-10 * time.Minute)},
			},
			wantDeleted:   0,
			wantRemaining: []string{"boundary"},
			wantGone:      nil,
		},
		{
			name: "one nanosecond past 10 minutes is expired",
			requests: []*domain.PendingAuthnRequest{
				{RequestID: "just-past", SAMLRequest: "<r/>", CreatedAt: fixedNow.Add(-10*time.Minute - time.Nanosecond)},
			},
			wantDeleted:   1,
			wantRemaining: nil,
			wantGone:      []string{"just-past"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := memory.NewPendingRequestRepo(memory.WithClock(func() time.Time { return fixedNow }))
			ctx := context.Background()

			for _, req := range tt.requests {
				_ = repo.Save(ctx, req)
			}

			count, err := repo.DeleteExpired(ctx)
			if err != nil {
				t.Fatalf("DeleteExpired() unexpected error: %v", err)
			}
			if count != tt.wantDeleted {
				t.Errorf("DeleteExpired() count = %d, want %d", count, tt.wantDeleted)
			}

			for _, id := range tt.wantRemaining {
				got, err := repo.GetAndDelete(ctx, id)
				if err != nil {
					t.Errorf("request %q should still exist, got error: %v", id, err)
					continue
				}
				if got.RequestID != id {
					t.Errorf("RequestID = %q, want %q", got.RequestID, id)
				}
			}

			for _, id := range tt.wantGone {
				_, err := repo.GetAndDelete(ctx, id)
				if err == nil {
					t.Errorf("request %q should be deleted, but GetAndDelete succeeded", id)
				}
			}
		})
	}
}

func TestPendingRequestRepo_ConcurrentAccess(t *testing.T) {
	repo := memory.NewPendingRequestRepo()
	ctx := context.Background()

	const numGoroutines = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrently save different requests
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			req := &domain.PendingAuthnRequest{
				RequestID:   fmt.Sprintf("concurrent-%d", id),
				SAMLRequest: "<concurrent/>",
				CreatedAt:   time.Now(),
			}
			_ = repo.Save(ctx, req)
		}(i)
	}
	wg.Wait()

	// Concurrently read and delete — must not panic
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			_, _ = repo.GetAndDelete(ctx, fmt.Sprintf("concurrent-%d", id))
			_, _ = repo.DeleteExpired(ctx)
		}(i)
	}
	wg.Wait()
}
