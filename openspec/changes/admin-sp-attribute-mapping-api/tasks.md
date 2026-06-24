## 1. Repository layer

- [x] 1.1 In `internal/repository/interfaces.go`, add
  `UpdateAttributeMapping(ctx context.Context, entityID string,
  mapping *domain.AttributeMapping) error` to
  `ServiceProviderRepository`. Document that a `nil` mapping clears
  the column.
- [x] 1.2 Implement `UpdateAttributeMapping` in
  `internal/repository/postgres/service_provider.go`. Use a single
  `UPDATE service_providers SET attribute_mapping = $1 WHERE
  entity_id = $2` statement with `$1` bound to either the JSONB-encoded
  mapping or a SQL `NULL`. Return `domain.ErrNotFound("service_provider",
  entityID)` when the update affects zero rows so the service layer
  can map it to 404.
- [x] 1.3 Wrap any Postgres error with `fmt.Errorf("update attribute
  mapping for %q: %w", entityID, err)` and return it; do not leak raw
  driver errors past the repository boundary.

## 2. Service layer

- [x] 2.1 In `internal/service/interfaces.go`, add
  `UpdateAttributeMapping(ctx context.Context, entityID string,
  mapping *domain.AttributeMapping) error` and
  `ClearAttributeMapping(ctx context.Context, entityID string) error`
  to `ServiceProviderService`.
- [x] 2.2 In `internal/service/service_provider.go`, implement
  `UpdateAttributeMapping`: reject a `nil` mapping with a typed
  `*domain.ErrValidation` for field `attribute_mapping`; call
  `mapping.Validate()` and return any validation error unchanged;
  call `repo.GetByEntityID` first and surface `ErrNotFound` if the SP
  does not exist; otherwise call `repo.UpdateAttributeMapping(ctx,
  entityID, mapping)`.
- [x] 2.3 Implement `ClearAttributeMapping` in the same file: call
  `repo.GetByEntityID` first for the same 404 behaviour, then call
  `repo.UpdateAttributeMapping(ctx, entityID, nil)`.
- [x] 2.4 Run `make generate` to regenerate the mocks for
  `ServiceProviderRepository` and `ServiceProviderService` (the
  `//go:generate` directives are already in `interfaces.go`).
- [x] 2.5 Add table-driven unit tests in
  `internal/service/service_provider_test.go` for
  `UpdateAttributeMapping` covering: nil mapping rejected,
  invalid mapping rejected (each `Validate` failure path), unknown
  SP returns `ErrNotFound`, successful update calls the repo once
  with the validated mapping. Mock the repository with the
  generated `mock_service_provider_repository`.
- [x] 2.6 Add table-driven unit tests for `ClearAttributeMapping`
  covering: unknown SP returns `ErrNotFound`, successful clear calls
  the repo once with `nil`, repository error is surfaced.

## 3. Handler layer

- [x] 3.1 In `internal/handler/admin.go`, declare
  `GetServiceProviderResponse` with JSON tags
  `entity_id`, `acs_url`, `acs_binding`, and
  `attribute_mapping,omitempty`. Add a small `spToResponse(*domain.ServiceProvider)`
  helper so the conversion is in one place.
- [x] 3.2 Implement `HandleGetServiceProvider`: read
  `entity_id` from the query string, return `400` with the existing
  `APIError` envelope when missing or empty; call
  `spService.GetByEntityID`; map `domain.ErrNotFound` to `404` via
  the existing `WriteError` helper; on success, write the response
  DTO with `WriteJSON(w, 200, …)`. Add request-scoped INFO logging
  with `entity_id`.
- [x] 3.3 Implement `HandleUpdateAttributeMapping`: validate the
  `entity_id` query parameter; decode the body into a
  `domain.AttributeMapping`, returning `400 invalid JSON` on decode
  failure; call `mapping.Validate()` and return its
  `*ErrValidation` via `WriteError`; call
  `spService.UpdateAttributeMapping`; on success, write
  `{"status":"success","message":"Attribute mapping updated"}` with
  `200`. Log INFO with `entity_id` and operation `update`; log
  ERROR with the wrapped error on failure.
- [x] 3.4 Implement `HandleDeleteAttributeMapping`: validate the
  `entity_id` query parameter; call
  `spService.ClearAttributeMapping`; on success, write
  `{"status":"success","message":"Attribute mapping cleared"}` with
  `200`. Log INFO with `entity_id` and operation `clear`; log ERROR
  with the wrapped error on failure.
- [x] 3.5 Register the three new routes in
  `internal/handler/routes.go`:
  `r.Get("/admin/service-providers", h.HandleGetServiceProvider)`,
  `r.Put("/admin/service-providers/attribute-mapping",
  h.HandleUpdateAttributeMapping)`,
  `r.Delete("/admin/service-providers/attribute-mapping",
  h.HandleDeleteAttributeMapping)`. Verify the existing
  `POST /admin/service-providers` still routes correctly (chi
  scopes by method so the two share a path).
- [x] 3.6 In `internal/handler/admin_test.go`, add table-driven
  handler tests for each new handler using the generated
  `mock_service_provider_service`. Cover every scenario from the
  spec delta: GET success with and without `attribute_mapping`,
  GET 400/404; PUT success (round-trip body), PUT 400 invalid JSON,
  PUT 400 invalid mapping (verify field path appears in body), PUT
  400 missing `entity_id`, PUT 404; DELETE success, DELETE
  idempotent on unmapped SP, DELETE 400 missing `entity_id`,
  DELETE 404; the 500-no-leakage scenario by injecting a generic
  error from the mock.

## 4. PUT-empty-object vs DELETE contract tests

- [x] 4.1 Add a focused handler-level test that issues `PUT
  /admin/service-providers/attribute-mapping` with body `{}` and
  then asserts via the mocked service that
  `UpdateAttributeMapping` is called once with a non-nil
  `*domain.AttributeMapping` whose every field is at its zero
  value.
- [x] 4.2 Add a focused handler-level test that issues `DELETE` and
  asserts via the mocked service that `ClearAttributeMapping` is
  called once and `UpdateAttributeMapping` is never called.

## 5. Wiring and DI

- [x] 5.1 In `internal/app/app.go`, no new constructor dependencies
  are required: the new handlers only need the existing
  `spService` already wired into `Handlers`. Verify by inspection
  that no additional fields, parameters, or graph wiring are
  needed; record any deviation in Implementation Notes.

## 6. Documentation

- [x] 6.1 Update the README's admin API section to document all
  three new endpoints with example `curl` invocations (including
  percent-encoded `entity_id` values), expected success responses,
  and the four error cases (400 missing `entity_id`, 400 invalid
  JSON/validation, 404 unknown SP, 500 internal error).
- [x] 6.2 In the README, add a callout explicitly contrasting
  `PUT {}` and `DELETE`, mirroring the spec's "PUT with an empty
  object is distinct from DELETE" requirement.
- [x] 6.3 In the README, add a note that the admin endpoints
  currently have no authentication and SHOULD be deployed behind a
  trusted network boundary, matching the existing
  `POST /admin/service-providers` posture. Reference the future
  capability that will add auth.
- [x] 6.4 If a Postman / OpenAPI / `docs/admin-api/` artefact
  exists, add the three new endpoints there as well; if no such
  artefact exists, skip and note in Implementation Notes.

## 7. Verification suite

- [x] 7.1 Run `make license-check` and confirm the AGPL-3.0-only
  header is present on every new Go source file.
- [x] 7.2 Run `make fmt` and confirm there are no diffs.
- [x] 7.3 Run `make lint` and confirm there are no warnings.
- [x] 7.4 Run `make generate` and confirm there are no
  uncommitted changes under `mocks/` after running (mocks are up to
  date with the new interface methods).
- [x] 7.5 Run `make test` and confirm all packages pass, with the
  new handler / service / repo tests included.
- [x] 7.6 Run `make build` and confirm a clean binary build.
- [ ] 7.7 Manual smoke test against a local Postgres + bridge:
  register an SP via `POST`, `GET` it back, `PUT` a modified
  mapping, `GET` it back and confirm the body matches, `DELETE` it,
  `GET` it back and confirm `attribute_mapping` is absent.

## 8. Documentation & rollout

- [ ] 8.1 Cut a single PR titled "Add admin GET/PUT/DELETE for SP
  attribute mapping (Phase 2d)". Reference this OpenSpec change
  and the v2 design doc.
- [ ] 8.2 Deployment: code-only change, no DB migration. Roll out
  with the standard deploy. New endpoints become available
  immediately. Existing `POST /admin/service-providers` remains
  unchanged.
- [ ] 8.3 Rollback: revert the PR. No persisted state is introduced
  by this change; any `attribute_mapping` documents written via the
  new `PUT` remain valid for the previous binary.
- [ ] 8.4 After merge, archive this OpenSpec change with
  `/opsx:archive admin-sp-attribute-mapping-api`; the sync step
  will fold the new requirements into
  `openspec/specs/per-sp-attribute-mapping/spec.md`.

## 9. Implementation Notes

<!-- Record any deviations from the plan above (interface signatures
that had to change, tests that were re-scoped, README sections that
already existed, files that were renamed, etc). Append entries as
work proceeds. Leave empty if there are none. -->

- Task 6.4: No OpenAPI / Postman / `docs/admin-api/` artefact exists
  in the repository; documentation for the three new endpoints lives
  in the README only.
