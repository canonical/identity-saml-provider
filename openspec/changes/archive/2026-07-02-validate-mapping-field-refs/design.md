## Context

`internal/domain/attribute_mapping.go` defines two parallel string-keyed
maps inside `AttributeMapping`:

- `OIDCClaimMappings map[string]string` — OIDC claim name → internal
  field name. Populated keys feed the typed `UserAttributes` struct
  (`Subject`, `Email`, `Name`, `Groups`) when the value matches one
  of the well-known field names, otherwise the value-named entry is
  written into the `Custom` map.
- `SAMLAttributeMappings map[string]SAMLAttributeDef` — internal
  field name → SAML attribute definition. At assertion time
  `buildSAMLAttributes` (in `internal/service/attribute_mapping.go`)
  iterates this map and reads each value through
  `UserAttributes.GetField(name)` (for single-valued fields) or
  `UserAttributes.Groups` (for the `groups` field).

The set of "well-known" internal field names is currently implicit and
duplicated across three call sites:

1. `BuildUserAttributes` — switch statement deciding whether to
   populate a struct field or the `Custom` map.
2. `UserAttributes.GetField` — switch statement returning the value
   for a name.
3. `buildSAMLAttributes` — special-case branch for `"groups"`.

Today `AttributeMapping.Validate` only enforces two structural rules:
the NameID format must be recognised and every `SAMLAttributeDef.Name`
must be non-empty. It does not check whether a key in
`SAMLAttributeMappings` can ever be populated. As a result, a typo
such as `oidc_claim_mappings: {"email": "emal"}` combined with
`saml_attribute_mappings: {"email": {...}}` is accepted, and at every
authentication the `email` SAML attribute is silently omitted with a
DEBUG log from `buildSAMLAttributes`.

This change moves that failure mode from authentication time to
configuration time.

## Goals / Non-Goals

**Goals:**

- Reject configurations where a `SAMLAttributeMappings` key cannot
  resolve to either a well-known internal field or a value produced
  by `OIDCClaimMappings`.
- Make the well-known internal field set a single source of truth in
  the domain layer that the validator, `BuildUserAttributes`, and
  `buildSAMLAttributes` can all consult.
- Keep validation cheap and pure: no I/O, no database access, no
  context plumbing.
- Preserve the contract of every existing
  `AttributeMapping.Validate()` caller (CLI, admin handler, service
  layer) — they continue to call `Validate()` with no signature
  change.

**Non-Goals:**

- Verifying that referenced OIDC claims will actually be present in
  any given ID token. Claims are dynamic and per-user; the validator
  works on configuration alone.
- Validating `oidc_claim_mappings` target names against the
  well-known field set in isolation (a custom target like `"dept"`
  is legitimate when paired with a matching SAML key).
- Re-validating mapping rows already persisted in the database. The
  next admin write is the trigger.
- Any change to the assertion construction path, the field
  suppression performed by `MappingService.ApplyMapping`, or the
  CLI flag surface.

## Decisions

### D1: Single source of truth for well-known fields

The set `{"subject", "email", "name", "groups"}` becomes a package-
level value in `internal/domain` — a `map[string]struct{}` exposed
through a small helper:

```go
// IsWellKnownField reports whether name identifies a well-known
// internal user attribute that AttributeMapping consumers populate
// directly on UserAttributes (Subject, Email, Name, Groups) rather
// than through UserAttributes.Custom.
func IsWellKnownField(name string) bool
```

**Why a helper, not an exported variable**: exposing a mutable map
risks accidental mutation by callers. A function preserves
encapsulation while still being trivially testable.

**Why expose at all** (vs. keeping the constant inside `Validate`):
the design proposal in this change body — and the
`UserAttributes.GetField` / `BuildUserAttributes` call sites — all
need the same set. Centralising removes a maintenance hazard if a
fifth well-known field is ever added.

**Alternative considered**: a `WellKnownFields` exported map. Rejected
to keep the API surface read-only.

### D2: Validation algorithm

`AttributeMapping.Validate` gains a third structural check, run after
the existing NameID-format and non-empty-name checks pass:

1. Skip the cross-map check entirely when
   `len(m.SAMLAttributeMappings) == 0`. An SP with no SAML attribute
   mappings has nothing to resolve.
2. Build a one-shot lookup set `oidcTargets` of all values in
   `m.OIDCClaimMappings`. This is an O(N) walk over the source map
   and avoids a nested loop per SAML key.
3. For each key `k` of `m.SAMLAttributeMappings`:
   - If `IsWellKnownField(k)` → accept.
   - Else if `k` is in `oidcTargets` → accept.
   - Else → return `&ErrValidation{Field: "saml_attribute_mappings." + k, Message: ...}`.

Complexity: O(|OIDCClaimMappings| + |SAMLAttributeMappings|). No
allocations beyond the lookup set.

**Why iterate `SAMLAttributeMappings` (not `OIDCClaimMappings`)**:
the failure mode is an unreachable SAML attribute. Iterating the
target map matches the failure mode and lets the error name the
offending SAML field path directly.

**Error message**: identify the unresolvable key and explain both
remediation paths in one sentence — "must be a well-known field
(subject, email, name, groups) or a target value in
`oidc_claim_mappings`". The `Field` is the JSON path the API client
sent (`saml_attribute_mappings.<key>`), matching the convention
already used by the empty-name check.

### D3: Determinism for tests and operators

Go's map iteration order is non-deterministic. To keep error
messages stable when multiple keys would fail, the validator
collects unresolvable keys, sorts them, and reports the first one
(lexicographic order). This costs at most O(K log K) for K
SAMLAttributeMappings entries — negligible at realistic
configuration sizes — and removes a flaky-test surface area.

**Alternative considered**: return all unresolvable keys in a single
error. Rejected because `ErrValidation` is single-field by design,
and the existing handler / CLI error formatting expects one field
per error.

### D4: When validation runs

No new call sites. The existing chain is preserved:

```text
sp add CLI ─┐                                                ┌─ ServiceProvider.Validate()
            │                                                │
admin POST ─┼─► ServiceProviderService.Register      ──►─────┤
            │                                                │
admin PUT  ─┴─► ServiceProviderService.UpdateAttributeMapping┤
                                                             │
                                                             └─ AttributeMapping.Validate()
```

`ServiceProvider.Validate()` already calls
`AttributeMapping.Validate()` when an attribute mapping is present,
and `UpdateAttributeMapping` calls `AttributeMapping.Validate()`
directly. Both paths inherit the new check without code changes.

### D5: Backward compatibility for stored rows

Rows already in `service_providers.attribute_mapping` are not
re-validated. The new check fires only on the next register / update
write. This matches the existing pre-production stance of the
codebase (no Phase 1 backward-compatibility code) and avoids a
migration that would arbitrarily reject opaque legacy data on
startup.

**Alternative considered**: validate-on-read inside the repository's
`GetByEntityID`. Rejected — it conflates persistence with policy and
would break the bridge for SPs whose mappings are still functionally
correct under the legacy rules.

### D6: Test surface

Table-driven tests in `internal/domain/attribute_mapping_test.go`
add the following scenarios. Each row shows the literal map contents
that would be passed in, so the contract is unambiguous.

| # | Case | `oidc_claim_mappings` | `saml_attribute_mappings` | Expected |
| --- | --- | --- | --- | --- |
| 1 | Well-known SAML key, no OIDC mappings | `{}` | `{"email": {"name": "mail"}}` | accept |
| 2 | Custom SAML key, matching OIDC target | `{"department": "dept"}` | `{"dept": {"name": "urn:oid:2.5.4.11", "friendly_name": "ou"}}` | accept |
| 3 | Custom SAML key, no OIDC target | `{"sub": "subject"}` | `{"dept": {"name": "department"}}` | reject with `Field = "saml_attribute_mappings.dept"` |
| 4 | Typo in OIDC target, SAML key is well-known | `{"email": "emal"}` | `{"email": {"name": "mail"}}` | accept (well-known short-circuit; `email` will be empty at runtime and the attribute will be omitted with a DEBUG log) |
| 5 | Typo mirrored on both sides | `{"email": "emal"}` | `{"emal": {"name": "mail"}}` | accept (`emal` is a target in `oidc_claim_mappings`; structural rule satisfied) |
| 6 | `nameid_format` set, `saml_attribute_mappings` empty | `{}` | `{}` | accept (cross-map check is skipped) |
| 7 | Multiple unresolvable keys | `{"sub": "subject"}` | `{"zeta": {"name": "z"}, "alpha": {"name": "a"}}` | reject with `Field = "saml_attribute_mappings.alpha"` (sorted, first-key wins per D3) |

A representative full payload for case 3:

```json
{
  "nameid_format": "transient",
  "oidc_claim_mappings": { "sub": "subject" },
  "saml_attribute_mappings": {
    "dept": { "name": "department" }
  }
}
```

A representative full payload for case 2 (accepted):

```json
{
  "nameid_format": "transient",
  "oidc_claim_mappings": { "department": "dept" },
  "saml_attribute_mappings": {
    "dept": { "name": "urn:oid:2.5.4.11", "friendly_name": "ou" }
  }
}
```

Cases 4 and 5 pin the contract explicitly: the validator is not a
spell-checker. A typo is rejected only when it produces an
unresolvable SAML key.

## Risks / Trade-offs

- **Risk**: existing tests in the codebase or external test data
  fixtures may include "orphan" SAML keys that incidentally pass
  today. → **Mitigation**: run `make test` after the validator
  change; fix or delete fixtures that were never realistic.
- **Risk**: operators with persisted invalid mappings will be
  surprised when a re-save fails. → **Mitigation**: the error
  message names the field path and the remediation; no silent
  behaviour change for reads.
- **Trade-off**: the validator is intentionally structural, not
  semantic. A typo `"emal"` mirrored on both sides of the maps will
  pass validation and produce a SAML attribute named after the typo.
  Catching that would require an external schema of accepted
  upstream claims — out of scope and arguably the wrong layer.
- **Trade-off**: making `IsWellKnownField` part of the domain API
  exposes a small piece of internal vocabulary. Acceptable because
  the same vocabulary already appears in the JSON schema operators
  write (`"subject"`, `"email"`, etc.) and in the existing
  `UserAttributes` struct fields.

## Migration Plan

1. Land the domain-layer change (`IsWellKnownField` + extended
   `Validate`) with new unit tests.
2. Run `make build`, `make fmt`, `make lint`, `make test`,
   `make license-check`. No mocks to regenerate (no interface
   change).
3. No database migration. No CLI / API surface change.
4. **Rollback**: revert the domain-layer commit. Stored rows are
   untouched and remain readable.

## Open Questions

None. The shape of the check, its placement, and its error contract
are all determined by the existing `AttributeMapping.Validate`
behaviour and the `ErrValidation` convention already used in this
package.
