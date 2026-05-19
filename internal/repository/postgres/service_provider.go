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
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/tracing"
)

// uniqueViolationCode is the PostgreSQL error code for unique_violation (23505).
const uniqueViolationCode = "23505"

// ServiceProviderRepo is the PostgreSQL implementation of
// repository.ServiceProviderRepository.
type ServiceProviderRepo struct {
	db     DBTX
	tracer tracing.TracingInterface
}

// NewServiceProviderRepo creates a new ServiceProviderRepo backed by
// the given DBTX (either a *pgxpool.Pool or a pgx.Tx).
func NewServiceProviderRepo(db DBTX, tracer tracing.TracingInterface) *ServiceProviderRepo {
	return &ServiceProviderRepo{db: db, tracer: tracer}
}

// Save persists a service provider, upserting on conflict by entity_id.
func (r *ServiceProviderRepo) Save(ctx context.Context, sp *domain.ServiceProvider) error {
	ctx, span := r.tracer.Start(ctx, "repo.postgres.save_sp")
	defer span.End()
	span.SetAttributes(attribute.String("db.system", "postgresql"))

	// Serialize attribute mapping to JSONB (nil-safe).
	var mappingJSON interface{}
	if sp.AttributeMapping != nil {
		data, err := json.Marshal(sp.AttributeMapping)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("marshal attribute mapping: %w", err)
		}
		mappingJSON = data
	}

	query, args, err := psql.
		Insert("service_providers").
		Columns("entity_id", "acs_url", "acs_binding", "attribute_mapping").
		Values(sp.EntityID, sp.ACSURL, sp.ACSBinding, mappingJSON).
		Suffix(`ON CONFLICT (entity_id) DO UPDATE SET
			acs_url           = EXCLUDED.acs_url,
			acs_binding       = EXCLUDED.acs_binding,
			attribute_mapping = EXCLUDED.attribute_mapping`).
		ToSql()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("build save service provider query: %w", err)
	}

	_, err = r.db.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
			return &domain.ErrConflict{Resource: "service_provider", ID: sp.EntityID}
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("exec save service provider: %w", err)
	}
	return nil
}

// GetByEntityID retrieves a service provider by its entity ID.
// Returns *domain.ErrNotFound if no matching service provider exists.
func (r *ServiceProviderRepo) GetByEntityID(ctx context.Context, entityID string) (*domain.ServiceProvider, error) {
	ctx, span := r.tracer.Start(ctx, "repo.postgres.get_sp")
	defer span.End()
	span.SetAttributes(attribute.String("db.system", "postgresql"))

	query, args, err := psql.
		Select("entity_id", "acs_url", "acs_binding", "attribute_mapping").
		From("service_providers").
		Where(sq.Eq{"entity_id": entityID}).
		ToSql()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("build get service provider query: %w", err)
	}

	var sp domain.ServiceProvider
	var mappingJSON *string
	err = r.db.QueryRow(ctx, query, args...).Scan(
		&sp.EntityID, &sp.ACSURL, &sp.ACSBinding, &mappingJSON,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &domain.ErrNotFound{Resource: "service_provider", ID: entityID}
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("scan service provider %s: %w", entityID, err)
	}

	// Deserialize attribute mapping if present.
	if mappingJSON != nil && *mappingJSON != "" {
		var mapping domain.AttributeMapping
		if err := json.Unmarshal([]byte(*mappingJSON), &mapping); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("unmarshal attribute mapping for SP %s: %w", entityID, err)
		}
		sp.AttributeMapping = &mapping
	}

	return &sp, nil
}

// GetAttributeMapping retrieves only the attribute mapping for a
// service provider. Returns nil (without error) if the SP exists but
// has no mapping configured. Returns *domain.ErrNotFound if the SP
// does not exist.
func (r *ServiceProviderRepo) GetAttributeMapping(ctx context.Context, entityID string) (*domain.AttributeMapping, error) {
	ctx, span := r.tracer.Start(ctx, "repo.postgres.get_attribute_mapping")
	defer span.End()
	span.SetAttributes(attribute.String("db.system", "postgresql"))

	query, args, err := psql.
		Select("attribute_mapping").
		From("service_providers").
		Where(sq.Eq{"entity_id": entityID}).
		ToSql()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("build get attribute mapping query: %w", err)
	}

	var mappingJSON *string
	err = r.db.QueryRow(ctx, query, args...).Scan(&mappingJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &domain.ErrNotFound{Resource: "service_provider", ID: entityID}
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("scan attribute mapping for SP %s: %w", entityID, err)
	}

	if mappingJSON == nil || *mappingJSON == "" {
		return nil, nil
	}

	var mapping domain.AttributeMapping
	if err := json.Unmarshal([]byte(*mappingJSON), &mapping); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("unmarshal attribute mapping for SP %s: %w", entityID, err)
	}
	return &mapping, nil
}
