## Purpose

Provides secure TLS connection capabilities and custom CA certificate verification when connecting to PostgreSQL database instances.

## ADDED Requirements

### Requirement: Custom PostgreSQL CA Certificate Configuration
The system SHALL support configuring a custom Certificate Authority (CA) certificate file path for PostgreSQL connections via the `SAML_PROVIDER_DB_CA_CERT_PATH` environment variable.

#### Scenario: CA certificate path provided without explicit sslmode
- **WHEN** `SAML_PROVIDER_DB_CA_CERT_PATH` is set and `SAML_PROVIDER_DB_SSLMODE` is not explicitly set
- **THEN** the system automatically defaults the effective SSL mode to `verify-full` (secure-by-default) and generates a database connection DSN containing `sslmode=verify-full` and `sslrootcert=<path>`

#### Scenario: Valid CA certificate path provided with explicit verify-ca mode
- **WHEN** `SAML_PROVIDER_DB_SSLMODE` is set to `verify-ca` and `SAML_PROVIDER_DB_CA_CERT_PATH` is set to a valid CA certificate file path
- **THEN** the system generates a database connection DSN containing `sslmode=verify-ca` and `sslrootcert=<path>`

#### Scenario: CA certificate path omitted
- **WHEN** `SAML_PROVIDER_DB_CA_CERT_PATH` is omitted or empty
- **THEN** the system generates a database connection DSN using the default `sslmode=disable` without the `sslrootcert` parameter

#### Scenario: Conflicting configuration with disable sslmode and CA certificate path
- **WHEN** `SAML_PROVIDER_DB_CA_CERT_PATH` is set and `SAML_PROVIDER_DB_SSLMODE` is explicitly set to `disable`
- **THEN** configuration validation fails with an error indicating that a CA certificate path cannot be used with `sslmode=disable`

### Requirement: Database Connection URL Generation
The system SHALL safely include the `sslrootcert` parameter in generated PostgreSQL connection strings when a CA certificate path is configured.

#### Scenario: Connection URL formatting with special characters in path
- **WHEN** a custom CA certificate path contains spaces or special characters
- **THEN** the generated connection string URL-encodes query parameters appropriately to prevent malformed connection strings

#### Scenario: Connection URL formatting with custom CA and verify-full mode
- **WHEN** `SAML_PROVIDER_DB_SSLMODE` is set to `verify-full` and `SAML_PROVIDER_DB_CA_CERT_PATH` is set to a certificate file path
- **THEN** the generated connection string includes both `sslmode=verify-full` and `sslrootcert=<path>` query parameters
