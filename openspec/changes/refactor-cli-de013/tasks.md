## 1. Refactor Service Provider Command Grammar & Source Files

- [ ] 1.1 Rename source and test files in `internal/cmd/`: `sp.go` $\rightarrow$ `service_provider.go`, `sp_add.go` $\rightarrow$ `service_provider_create.go`, `sp_formatter.go` $\rightarrow$ `service_provider_formatter.go`, `sp_formatter_test.go` $\rightarrow$ `service_provider_formatter_test.go`, and `sp_test.go` $\rightarrow$ `service_provider_test.go`.
- [ ] 1.2 Refactor command definition from `sp add` to `service-provider create`. Unit test coverage in `internal/cmd/service_provider_test.go`.
- [ ] 1.3 Remove short flags (`-e`, `-a`, `-b`) from `service-provider create` flags initialization in `internal/cmd/service_provider_create.go`, retaining only long flags (`--entity-id`, `--acs-url`, `--acs-binding`, `--attribute-mapping-file`). Unit test coverage in `internal/cmd/service_provider_test.go`.

## 2. Refactor Requests Cleanup Command Grammar & Source Files

- [ ] 2.1 Rename source and test files in `internal/cmd/`: `janitor.go` $\rightarrow$ `requests.go`, `janitor_pending_requests.go` $\rightarrow$ `requests_prune.go`, `janitor_formatter.go` $\rightarrow$ `requests_formatter.go`, `janitor_formatter_test.go` $\rightarrow$ `requests_formatter_test.go`, and `janitor_test.go` $\rightarrow$ `requests_test.go`.
- [ ] 2.2 Refactor command definition from `janitor pending-requests` to `requests prune`. Unit test coverage in `internal/cmd/requests_test.go`.

## 3. Refactor Migrations Command Grammar, Source Files & Tabular Formatting

- [ ] 3.1 Rename source and test files in `internal/cmd/`: `migrate.go` $\rightarrow$ `migrations.go`, `migrate_formatter.go` $\rightarrow$ `migrations_formatter.go`, `migrate_formatter_test.go` $\rightarrow$ `migrations_formatter_test.go`, and `migrate_test.go` $\rightarrow$ `migrations_test.go`.
- [ ] 3.2 Refactor database migration command hierarchy from `migrate <up|down|status|check>` to `migrations [apply|rollback|show|check]` in `internal/cmd/migrations.go`. Unit test coverage in `internal/cmd/migrations_test.go`.
- [ ] 3.3 Update result payload struct or formatter signature in `internal/cmd/migrations_formatter.go` to pass `--no-headers` flag state to `formatMigrationStatuses`.
- [ ] 3.4 Update `formatMigrationStatuses` in `internal/cmd/migrations_formatter.go` to use DE013 tabular format (uppercase headers `APPLIED AT`, `MIGRATION`, 2-space delimiters, no ASCII decorations) and respect `--no-headers` in `migrations show`. Unit test coverage in `internal/cmd/migrations_formatter_test.go`.

## 4. Standardize Error Phrasing and Runner Formatting

- [ ] 4.1 Update root error runner in `internal/cmd/runner.go` line 95 to output lowercase `error: %v\n` prefix instead of `Error: %v\n`.
- [ ] 4.2 Replace "failed to" and non-direct error strings across `internal/cmd/` with passive/direct "cannot ..." phrasing:
  - `internal/cmd/requests_prune.go`: "failed to process configuration" $\rightarrow$ "cannot process configuration", "failed to execute cleanup batch" $\rightarrow$ "cannot execute cleanup batch"
  - `internal/cmd/migrations.go`: "failed to open database handle" $\rightarrow$ "cannot open database handle", "failed to connect to database" $\rightarrow$ "cannot connect to database", "failed to create goose provider" $\rightarrow$ "cannot create goose provider", "failed to check pending migrations" $\rightarrow$ "cannot check pending migrations", "failed to get current version" $\rightarrow$ "cannot get current version"
  - `internal/cmd/serve.go`: "Failed to process configuration" $\rightarrow$ "cannot process configuration", "Failed to initialize logger" $\rightarrow$ "cannot initialize logger"
  - `internal/cmd/service_provider.go`: "connect to database" $\rightarrow$ "cannot connect to database"
  - `internal/cmd/service_provider_create.go`: "load config from environment" $\rightarrow$ "cannot load config from environment", "read attribute mapping file" $\rightarrow$ "cannot read attribute mapping file", "parse attribute mapping JSON" $\rightarrow$ "cannot parse attribute mapping JSON"
- [ ] 4.3 Update corresponding test assertions in `internal/cmd/*_test.go` to check for updated error strings.

## 5. Verification Suite

- [ ] 5.1 Execute `make build` to verify clean compilation.
- [ ] 5.2 Execute `make fmt` and `make lint` to ensure zero linter or formatting issues.
- [ ] 5.3 Execute `make test` to verify all table-driven unit tests pass.
- [ ] 5.4 Execute `make license-check` to verify AGPL-3.0-only header compliance.

## 6. Documentation & Rollout

- [ ] 6.1 Update CLI usage examples in `README.md` and documentation files to reflect the new command hierarchy (`service-provider create`, `requests prune`, `migrations apply/rollback/show/check`).
- [ ] 6.2 Document breaking CLI command changes for rollout release notes.

## Implementation Notes

<!-- Record any deviations from the plan during execution here -->
