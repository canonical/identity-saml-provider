## 1. Domain layer

- [x] 1.1 Add `IsWellKnownField(name string) bool` in
  `internal/domain/attribute_mapping.go`, backed by an unexported
  package-level `map[string]struct{}` containing `subject`, `email`,
  `name`, and `groups`. Include a doc comment explaining when
  callers should use the helper vs. reading struct fields directly.
- [x] 1.2 Refactor the existing `switch` statements in
  `UserAttributes.GetField` and `BuildUserAttributes`
  (`internal/service/attribute_mapping.go`) to either call
  `IsWellKnownField` or keep their typed dispatch but reference the
  same constants — whichever keeps each switch self-evident. No
  behaviour change.
- [x] 1.3 Extend `AttributeMapping.Validate` with the cross-map
  resolvability check described in `design.md` §D2:
  - Skip when `len(m.SAMLAttributeMappings) == 0`.
  - Precompute `oidcTargets` from the values of
    `m.OIDCClaimMappings`.
  - For each key, accept if `IsWellKnownField(key)` or
    `oidcTargets[key]`; otherwise collect into a slice of
    unresolvable keys.
  - Sort unresolvable keys lexicographically and return
    `&ErrValidation{Field: "saml_attribute_mappings." + keys[0],
    Message: "..."}` for the first failing key.
- [x] 1.4 Error message phrasing: include both remediation paths,
  e.g. `"must be a well-known field (subject, email, name, groups)
  or a target value in oidc_claim_mappings"`. Keep it to one
  sentence to match existing `ErrValidation` style.

## 2. Tests

- [x] 2.1 Extend the table-driven cases in
  `internal/domain/attribute_mapping_test.go` to cover every
  scenario in the spec delta:
  - well-known SAML key + empty `oidc_claim_mappings` → accept
  - custom SAML key + matching OIDC target → accept
  - custom SAML key + no OIDC target → reject with
    `saml_attribute_mappings.dept`
  - mirrored typo on both sides → accept
  - isolated OIDC-side typo + well-known SAML key → accept
  - multiple unresolvable keys → reject with
    `saml_attribute_mappings.alpha` (sorted)
  - empty `saml_attribute_mappings` with non-empty
    `nameid_format` → accept
- [x] 2.2 Add an `IsWellKnownField` unit test that pins the four
  well-known names and asserts unknown names return `false`. Drives
  the well-known set as a contract.
- [x] 2.3 Re-run the existing `attribute_mapping_test.go` cases
  unchanged and ensure none regress.
- [x] 2.4 Audit `internal/service/attribute_mapping_test.go`,
  `internal/handler/admin_test.go`, and any fixture JSON for
  mappings that incidentally pass today but would fail the new
  cross-map check. Update fixtures to use resolvable keys (the
  intent of every existing test is preserved by adding a matching
  `oidc_claim_mappings` entry, not by relaxing the validator).

## 3. Verification suite

- [x] 3.1 Run `make fmt`.
- [x] 3.2 Run `make build`.
- [x] 3.3 Run `make lint` and resolve any new warnings.
- [x] 3.4 Run `make test` and confirm both new and existing tests
  pass. No new flakiness in the validator path.
- [x] 3.5 Run `make license-check`.
- [x] 3.6 Run `make generate` to confirm no mocks need
  regeneration (no interface changes are expected; this is a
  guard against accidental interface drift).

## 4. Documentation

- [x] 4.1 Update any `README.md`, `docs/`, or admin-API examples
  that show an `AttributeMapping` with orphan
  `saml_attribute_mappings` keys (search for
  `"saml_attribute_mappings"` across `docs/`,
  `deployments/`, and the repository root and adjust as needed).

## 5. Validation against the spec

- [x] 5.1 Run `openspec validate validate-mapping-field-refs` and
  confirm it reports the change as valid after implementation.
- [x] 5.2 Cross-check each `#### Scenario` in
  `specs/per-sp-attribute-mapping/spec.md` against the test names
  in `attribute_mapping_test.go` to make sure every scenario has at
  least one covering unit test.

## 6. Implementation Notes

- Task 1.2: replaced the parallel `switch` statements in
  `UserAttributes.GetField`, `BuildUserAttributes`, and the
  session-fallback helper with a single **field registry** in
  `internal/domain/attribute_mapping.go`. Each well-known field is
  now one entry in `wellKnownFields []WellKnownField`, carrying
  typed getter/setter closures and (where applicable) a session
  fallback. `IsWellKnownField` and the exported
  `LookupWellKnownField` are backed by an index over the same
  slice. Adding a new well-known field now requires exactly two
  edits: a new struct field on `UserAttributes` and a new registry
  entry. The obsolete `sessionFieldFallback` helper in the service
  layer was removed; `extractGroups` was renamed to
  `extractGroupsFromClaims` and no longer takes a `*Session`
  because the caller now consults the registry's
  `SessionSliceFallback` directly. Behaviour is preserved.
- Task 2.4: the suite-wide test run after the validator change
  produced no failures, so no existing test fixtures used orphan
  SAML keys. No fixtures were modified.
- Task 4.1: README field description for `saml_attribute_mappings`
  now spells out the resolvability rule. The example JSON in both
  README and `docs/authentication-flow/authentication-flow.md` was
  already compliant (well-known keys plus the custom `username`
  key paired with a matching OIDC target) and did not need
  changes.
