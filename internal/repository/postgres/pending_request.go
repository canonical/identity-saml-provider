// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/tracing"
)

// PendingRequestRepo is the PostgreSQL implementation of repository.PendingRequestRepository.
type PendingRequestRepo struct {
	db     DBTX
	tracer tracing.TracingInterface
}

// NewPendingRequestRepo creates a new PendingRequestRepo backed by the given DBTX.
func NewPendingRequestRepo(db DBTX, tracer tracing.TracingInterface) *PendingRequestRepo {
	return &PendingRequestRepo{db: db, tracer: tracer}
}

// Save stores a pending authentication request.
func (r *PendingRequestRepo) Save(ctx context.Context, req *domain.PendingAuthnRequest) error {
	ctx, span := r.tracer.Start(ctx, "repo.postgres.save_pending")
	defer span.End()
	span.SetAttributes(attribute.String("db.system", "postgresql"))

	var metadataJSON []byte
	if req.ClientMetadata != nil {
		var err error
		metadataJSON, err = json.Marshal(req.ClientMetadata)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("marshal client metadata: %w", err)
		}
	} else {
		metadataJSON = []byte("{}")
	}

	query, args, err := psql.
		Insert("pending_requests").
		Columns("request_id", "saml_request", "relay_state", "client_metadata", "created_at", "expire_at").
		Values(req.RequestID, req.SAMLRequest, req.RelayState, metadataJSON, req.CreatedAt, req.ExpireAt).
		// We use DO UPDATE to handle user login retries with the same request_id.
		// This refreshes the TTL (expire_at) and updates the client metadata
		// (IP and User-Agent) to match the latest retry attempt.
		// Replays remain protected because consumed requests are deleted upon callback.
		Suffix(`ON CONFLICT (request_id) DO UPDATE SET
			saml_request    = EXCLUDED.saml_request,
			relay_state     = EXCLUDED.relay_state,
			client_metadata = EXCLUDED.client_metadata,
			created_at      = EXCLUDED.created_at,
			expire_at       = EXCLUDED.expire_at`).
		ToSql()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("build save pending request query: %w", err)
	}

	_, err = r.db.Exec(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("exec save pending request: %w", err)
	}
	return nil
}

// GetAndDelete retrieves and removes a pending request by its ID in a single atomic transaction.
// Returns *domain.ErrNotFound if the request does not exist or has expired.
func (r *PendingRequestRepo) GetAndDelete(ctx context.Context, requestID string) (*domain.PendingAuthnRequest, error) {
	ctx, span := r.tracer.Start(ctx, "repo.postgres.get_and_delete_pending")
	defer span.End()
	span.SetAttributes(attribute.String("db.system", "postgresql"))

	query, args, err := psql.
		Delete("pending_requests").
		Where(sq.Eq{"request_id": requestID}).
		Where("expire_at >= NOW()").
		Suffix("RETURNING request_id, saml_request, relay_state, client_metadata, created_at, expire_at").
		ToSql()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("build get and delete pending request query: %w", err)
	}

	var req domain.PendingAuthnRequest
	var metadataBytes []byte
	err = r.db.QueryRow(ctx, query, args...).Scan(
		&req.RequestID,
		&req.SAMLRequest,
		&req.RelayState,
		&metadataBytes,
		&req.CreatedAt,
		&req.ExpireAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &domain.ErrNotFound{Resource: "pending_request", ID: requestID}
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("scan get and delete pending request %s: %w", requestID, err)
	}

	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &req.ClientMetadata); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("unmarshal client_metadata for %s: %w", requestID, err)
		}
	}
	if req.ClientMetadata == nil {
		req.ClientMetadata = make(map[string]string)
	}

	return &req, nil
}

// DeleteExpired removes expired pending requests in chunks.
// The passed context.Context should carry a reasonable timeout (e.g. from the caller)
// to prevent indefinite blocking in the event of high database lock contention.
func (r *PendingRequestRepo) DeleteExpired(ctx context.Context, limit int) (int64, error) {
	if limit <= 0 {
		return 0, fmt.Errorf("limit must be positive, got %d", limit)
	}

	ctx, span := r.tracer.Start(ctx, "repo.postgres.delete_expired_pending")
	defer span.End()
	span.SetAttributes(attribute.String("db.system", "postgresql"))

	subquery := sq.Select("request_id").
		From("pending_requests").
		Where("expire_at < NOW()").
		Suffix("FOR UPDATE SKIP LOCKED").
		Limit(uint64(limit))

	query, args, err := psql.
		Delete("pending_requests").
		Where(sq.Expr("request_id IN (?)", subquery)).
		ToSql()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, fmt.Errorf("build delete expired query: %w", err)
	}

	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, fmt.Errorf("exec delete expired: %w", err)
	}
	return tag.RowsAffected(), nil
}
