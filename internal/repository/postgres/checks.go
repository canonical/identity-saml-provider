package postgres

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/canonical/identity-saml-provider/internal/repository"
)

// Compile-time checks: *pgxpool.Pool satisfies DBTX.
// Note: pgx.Tx (interface) also satisfies DBTX but cannot be checked
// this way since it is an interface type.
var _ DBTX = (*pgxpool.Pool)(nil)

// Compile-time checks: repository implementations satisfy their interfaces.
var (
	_ repository.SessionRepository         = (*SessionRepo)(nil)
	_ repository.ServiceProviderRepository = (*ServiceProviderRepo)(nil)
)
