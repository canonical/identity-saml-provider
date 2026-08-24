# Tasks: Configurable Persistent NameID Mode (Public Default vs Pairwise)

## 1. Domain Model & Validation

- [x] 1.1 Add `PersistentType` string field (`json:"persistent_type,omitempty"`)
  to `AttributeMapping` in `internal/domain/attribute_mapping.go`. Includes
  unit tests in `internal/domain/attribute_mapping_test.go`.
- [x] 1.2 Update `AttributeMapping.Validate()` in
  `internal/domain/attribute_mapping.go` to validate `persistent_type`:
  - Must be `"public"` or `"pairwise"` when non-empty.
  - Must be rejected if `nameid_format` is explicitly non-persistent
    (e.g., `"transient"` or `"emailAddress"`).
  - Includes unit tests covering valid and invalid values in
    `internal/domain/attribute_mapping_test.go`.

## 2. Service Layer Implementation

- [x] 2.1 Update `mappingService.resolveNameID` in
  `internal/service/attribute_mapping.go` to check `PersistentType`. If
  `"pairwise"`, query/generate UUID via `PersistentNameIDRepository`. If
  `"public"` or empty (default), return canonical subject directly without
  database query. Includes unit tests in `internal/service/attribute_mapping_test.go`.

## 3. Handlers & API Testing

- [x] 3.1 Verify HTTP request/response JSON serialization of `persistent_type`
  in admin API tests (`internal/handler/admin_test.go`). Note:
  `HandleUpdateAttributeMapping` unmarshals directly into `domain.AttributeMapping`,
  so no new DTO struct is required.

## 4. Verification Suite

- [x] 4.1 Run full verification suite (`make fmt`, `make lint`, `make test`,
  `make build`, `make license-check`) and ensure zero errors or warnings.

## 5. Documentation & Rollout

- [x] 5.1 Update `docs/authentication-flow/authentication-flow.md` and
  `docs/design/per-sp-attribute-mapping-v2-design.md` with details on
  `persistent_type` configuration (`"public"` default vs `"pairwise"` opt-in)
  and highlight the breaking change.

## Implementation Notes

Record any deviations from the plan during implementation here.
