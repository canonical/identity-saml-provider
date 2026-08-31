## Why

When connecting to a PostgreSQL database secured with TLS certificates signed by a private or internal Certificate Authority (CA), the SAML provider application must verify the database server certificate against the provided CA bundle. Currently, the application only supports basic connection parameters without custom CA certificate validation for PostgreSQL, preventing secure deployments with TLS-enabled databases requiring `verify-ca` or `verify-full` SSL modes.

## What Changes

- Add support for configuring a custom PostgreSQL CA certificate file path via the environment variable `SAML_PROVIDER_DB_CA_CERT_PATH`.
- Update the internal database connection string generator (`DatabaseDSN`) to include `sslrootcert` when a CA certificate path is provided.
- Support `verify-full` (default) and `verify-ca` SSL modes with custom CA certificate verification for application runtime database connections (`pgxpool`).
- Migration commands continue to accept a standard PostgreSQL DSN via `--dsn`; operators can include `sslrootcert` in that DSN to run migrations against TLS-enabled PostgreSQL instances.

## Capabilities

### New Capabilities
- `database-tls-verification`: Allows the application to securely connect to TLS-enabled PostgreSQL databases using a custom CA certificate and `verify-ca` or `verify-full` SSL modes.

### Modified Capabilities
<!-- No existing capabilities requirements are changing -->

## Non-goals

- Client certificate (mutual TLS / mTLS) authentication for PostgreSQL.
- In-memory certificate provisioning via raw strings (the application accepts a file path to the certificate bundle).

## Success Metrics

- Unit tests verify all DSN parameter permutations (including `sslrootcert`, `verify-full` default, and `verify-ca`).
- Configuration validation correctly accepts valid configurations and rejects conflicting settings (such as `SAML_PROVIDER_DB_CA_CERT_PATH` paired with `SAML_PROVIDER_DB_SSLMODE=disable`).

## Impact

- Environment variables: Introduces `SAML_PROVIDER_DB_CA_CERT_PATH`.
- Configuration: Updates `app.Config` and connection URL generation (`DatabaseDSN`).
- Database migrations: CLI migration commands accept standard DSNs containing `sslrootcert` without code changes.
