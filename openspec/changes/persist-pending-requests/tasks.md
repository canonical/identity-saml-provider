# Tasks: Persist pending requests

## 1. Database Schema

- [x] 1.1 Create or update Goose migration file
  `migrations/004_add_pending_requests.sql` to define the `pending_requests`
  table with `expire_at TIMESTAMPTZ NOT NULL` and index
  `idx_pending_requests_expire_at`. Enable aggressive autovacuum table settings
  (`autovacuum_vacuum_scale_factor = 0.05` and
  `autovacuum_vacuum_threshold = 100`).
- [x] 1.2 Validate the database migration locally by running `make migrate-up`
  and `make migrate-down` to ensure smooth execution and rollback.

## 2. Domain Model

- [x] 2.1 Update the `PendingAuthnRequest` entity in
  `internal/domain/pending_request.go` to include `ExpireAt time.Time` and
  `ClientMetadata map[string]string`. Update any domain validation or tests.

## 3. Repository Layer

- [x] 3.1 Update repository interface `PendingRequestRepository` in
  `internal/repository/interfaces.go` to support storing `ExpireAt` and
  retrieving/deleting.
- [x] 3.2 Update or implement `internal/repository/postgres/pending_request.go`
  to:

  - Persist `ExpireAt` during insertions.
  - Implement atomic retrieval & state consumption inside `GetAndDelete` using
    a single-query `DELETE ... RETURNING ...` that explicitly filters by
    `expire_at >= NOW()`.
  - Handle `client_metadata` JSONB safely by initializing it to an empty map
    (`make(map[string]string)`) if null.
  - Implement a batched, chunked `DeleteExpired(ctx context.Context, limit int)`
    method using the chunked DELETE pattern.

## 4. Service Layer

- [x] 4.1 Update service interface `PendingRequestService` in
  `internal/service/interfaces.go` to support saving pending requests with
  `ExpireAt` and cleaning up expired requests in batches.
- [x] 4.2 Update `internal/service/pending_request.go` to:

  - Support setting and propagating `ExpireAt` and `ClientMetadata` on save.
  - Add a batched cleanup method `CleanupExpired(ctx context.Context, limit int)`
    which delegates to the repository's `DeleteExpired` method.

- [x] 4.3 Update service-level unit tests in
  `internal/service/pending_request_test.go` to verify the batched cleanup
  logic, mock the repository calls, and test with various mock data.

## 5. Handler and Wiring

- [x] 5.1 Update handler `SAMLSessionAdapter.GetSession` in
  `internal/handler/saml_adapters.go` to:

  - Securely extract client IP from proxy headers (`X-Forwarded-For`,
    `X-Real-IP`, falling back to `RemoteAddr`) and the User-Agent.
  - Compute `ExpireAt = time.Now().Add(ttl)` using the configured TTL.
  - Populate both `ClientMetadata` and `ExpireAt` on the `PendingAuthnRequest`
    entity before saving.

- [x] 5.2 Update handler unit tests in
  `internal/handler/saml_adapters_test.go` to verify proxy header extraction
  logic and correct assignment of `ClientMetadata` and `ExpireAt`.
- [x] 5.3 Update `internal/app/app.go` to wire the new Postgres-backed
  repository and service layer, replacing memory-backed repositories.

## 6. Redesigned Cobra Janitor CLI Command

- [x] 6.1 Implement `janitor` Cobra command structure under
  `internal/cmd/janitor.go` to support a hierarchical subcommand structure.
- [x] 6.2 Implement `janitor pending-requests` subcommand supporting `--batch-size`
  (default: 1000) for batch sizes and `--format json|text` flags. Ensure it
  securely processes configuration via `app.Config`.
- [x] 6.3 Implement custom standard output printing in
  `internal/cmd/janitor_formatter.go` to print results directly to
  `cmd.OutOrStdout()` without using global application logging.
- [x] 6.4 Implement unit tests in `internal/cmd/janitor_test.go` to verify
  flags, subcommand registration, and output formatting.

## 7. Verification Suite

- [x] 7.1 Remove obsolete files:
  `internal/repository/memory/pending_request.go` and its associated unit
  tests.
- [x] 7.2 Run mock generator `make generate` to update all interfaces and mocks.
- [x] 7.3 Run the full verification suite (`make build`, `make fmt`, `make lint`,
  `make test`, and `make license-check`) and ensure zero errors and zero
  warnings are emitted.

## 8. Documentation & Rollout

- [x] 8.1 Document migration steps in the PR description, highlighting that
  database migration `004_add_pending_requests.sql` must be applied before or
  alongside the application deployment.
- [x] 8.2 Document the deployment of the redesigned `janitor pending-requests`
  Kubernetes CronJob and its flags (`--batch-size`, `--format`).
- [x] 8.3 Verify `CHANGELOG.md` is NOT modified (managed automatically by
  release-please).

## 9. SSO Route Wrapper for Missing/Expired Requests

- [x] 9.1 Implement `HandleSSO(w http.ResponseWriter, r *http.Request)` in
  `internal/handler/saml_adapters.go` to return a `400 Bad Request` JSON error
  when `SAMLRequest` is missing or empty.
- [x] 9.2 Update route registration in `internal/handler/routes.go` to map
  `/saml/sso` to `h.HandleSSO` instead of directly calling
  `h.samlIDP.ServeSSO`.
- [x] 9.3 Write unit tests in `internal/handler/saml_adapters_test.go` to cover
  the wrapper with and without the `SAMLRequest` parameter.
- [x] 9.4 Execute the verification suite (`make build`, `make fmt`, `make lint`,
  `make test`, `make license-check`) to verify correctness with zero warnings.
