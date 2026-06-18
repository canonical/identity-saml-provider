// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/canonical/identity-saml-provider/internal/tracing"
)

// PersistentNameIDRepo is the PostgreSQL implementation of
// repository.PersistentNameIDRepository.
type PersistentNameIDRepo struct {
	db     DBTX
	tracer tracing.TracingInterface
}

// NewPersistentNameIDRepo creates a new PersistentNameIDRepo.
func NewPersistentNameIDRepo(db DBTX, tracer tracing.TracingInterface) *PersistentNameIDRepo {
	return &PersistentNameIDRepo{db: db, tracer: tracer}
}

// GetOrCreate returns the persistent NameID for (entityID, userSubject),
// generating and persisting a new UUID v4 on first use.
func (r *PersistentNameIDRepo) GetOrCreate(ctx context.Context, entityID, userSubject string) (string, error) {
	ctx, span := r.tracer.Start(ctx, "repo.persistent_nameid.get_or_create")
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("entityID", entityID),
	)

	// The no-op DO UPDATE makes RETURNING fire on the conflict path,
	// collapsing get-or-create into a single round-trip.
	query, args, err := psql.
		Insert("persistent_nameids").
		Columns("entity_id", "user_subject", "persistent_id").
		Values(entityID, userSubject, uuid.New().String()).
		Suffix(`ON CONFLICT (entity_id, user_subject) DO UPDATE
			SET entity_id = persistent_nameids.entity_id
			RETURNING persistent_id`).
		ToSql()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("build upsert persistent nameid query: %w", err)
	}

	var result string
	if err := r.db.QueryRow(ctx, query, args...).Scan(&result); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("upsert persistent nameid for SP %s: %w", entityID, err)
	}
	span.SetStatus(codes.Ok, "")
	return result, nil
}
