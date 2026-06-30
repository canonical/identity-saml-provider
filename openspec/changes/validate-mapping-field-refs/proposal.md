## Why

Today, administrators can register or update an SP attribute mapping
where `saml_attribute_mappings` references an internal field name that
no `oidc_claim_mappings` entry ever populates — for example a typo
(`"emal"` instead of `"email"`) or an orphaned custom field.
Validation accepts the configuration silently, and the problem only
surfaces at authentication time as a missing SAML attribute
accompanied by a DEBUG-level log. SPs receive incomplete assertions
and operators have no signal that their configuration is wrong.

Catching this class of error at configuration time turns a runtime
failure (silent, per-assertion, observable only in DEBUG logs) into
an immediate, actionable error returned by the admin API and CLI.

## What Changes

- `AttributeMapping.Validate` rejects any `saml_attribute_mappings`
  key that is neither a well-known internal field (`subject`, `email`,
  `name`, `groups`) nor a target value in `oidc_claim_mappings`. The
  returned error identifies the unresolvable field path so operators
  can fix the configuration immediately.
- The well-known field set is exposed as a single source of truth in
  the domain layer so the validator, `BuildUserAttributes`, and
  `buildSAMLAttributes` agree on which names bypass the `Custom` map.
- Both the admin API (`POST /admin/service-providers`,
  `PUT /admin/service-providers/attribute-mapping`) and the
  `sp add --attribute-mapping-file` CLI surface the same rejection at
  registration / update time. No new endpoints or flags.
- **BREAKING** for any operator who has saved a mapping where
  `saml_attribute_mappings` contains an unresolvable key: subsequent
  `Register` / `UpdateAttributeMapping` calls will reject that
  configuration. Existing rows already in the database are not
  re-validated automatically; the next write through the admin API or
  CLI is the trigger. Acceptable given the project is pre-production.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `per-sp-attribute-mapping`: tightens the **Structural validation of
  mapping configuration** requirement to additionally enforce
  cross-map resolvability of every `saml_attribute_mappings` key.
  Adds well-known-field semantics to the spec text and removes the
  deferral note that previously marked semantic cross-map validation
  as out of scope.

## Impact

- **Code**:
  - `internal/domain/attribute_mapping.go` — introduce
    `WellKnownFields` (or equivalent helper) and extend `Validate()`
    with cross-map checks.
  - `internal/domain/attribute_mapping_test.go` — new table-driven
    cases covering well-known fields, custom fields with an OIDC
    source, orphan keys, and typos in OIDC targets.
  - No changes required in `internal/service/service_provider.go`,
    handlers, or the CLI: they already delegate to
    `mapping.Validate()` for both register and update paths.
- **APIs**: same endpoints, same payloads. Previously-accepted invalid
  payloads now return `400 Bad Request` with an `ErrValidation`
  identifying the unresolvable `saml_attribute_mappings.<field>`.
- **Database / migrations**: none. No schema changes.
- **Dependencies**: none.
- **Docs**: spec delta only.

## Non-goals

- Re-validating mapping rows already persisted in the database. A
  background migration / sweep is **not** in scope; the next admin
  write is sufficient.
- Validating that referenced OIDC claims are actually present in the
  upstream ID token — that is a runtime concern handled by emitting a
  DEBUG log and omitting the attribute.
- Any change to the assertion-construction pipeline or to the field
  suppression performed by `MappingService.ApplyMapping` when a
  mapping is active.
- Renaming, deprecating, or extending `oidc_claim_mappings` or
  `saml_attribute_mappings` shapes.

## Success Metrics

- 100 % of misconfigured mappings of the form "`saml_attribute_mappings`
  key has no resolvable source" are rejected at registration / update
  time, with an error message naming the unresolvable field path.
- Unit tests covering the new scenarios (typo, orphan custom field,
  valid custom field with matching OIDC target, valid well-known
  field with no OIDC target) all pass under `make test`.
- No regression in the existing `Structural validation` scenarios:
  `make test` continues to pass after the delta.
- Zero new admin-side configuration failures observed in the
  authentication hot path for mappings written after this change
  lands (the failure mode shifts entirely to configuration time).
