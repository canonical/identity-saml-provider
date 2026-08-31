## 1. Configuration & DSN Generation

- [x] 1.1 Add `DBCACertPath` field with `envconfig:"SAML_PROVIDER_DB_CA_CERT_PATH"` to `app.Config` in `internal/app/config.go` and update validation in `Validate()` to reject conflicting `sslmode=disable` when `DBCACertPath` is set (Unit tests: verify env loading and validation in `internal/app/config_test.go`).
- [x] 1.2 Update `Config.DatabaseDSN()` in `internal/app/config.go` to default to `verify-full` when `DBCACertPath` is set and `DBSSLMode` is unspecified, and append `sslrootcert` query parameter (Unit tests: table-driven tests in `internal/app/config_test.go` verifying DSN formatting across all SSL modes and with/without CA cert path).

## 2. Verification Suite

- [x] 2.1 Run `make fmt`, `make lint`, `make test`, `make build`, and `make license-check` to ensure the entire verification suite passes with zero warnings.

## 3. Documentation & Rollout

- [x] 3.1 Update environment variable documentation or README if applicable.
- [x] 3.2 Ensure backwards compatibility: deployments without `SAML_PROVIDER_DB_CA_CERT_PATH` continue operating unchanged.

## Implementation Notes

<!-- Record any deviations from the plan during implementation -->
