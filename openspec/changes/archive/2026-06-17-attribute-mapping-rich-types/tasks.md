## 1. Domain types

- [x] 1.1 In `internal/domain/attribute_mapping.go`, add the
  `DefaultNameFormat` constant set to
  `"urn:oasis:names:tc:SAML:2.0:attrname-format:uri"`.
- [x] 1.2 In `internal/domain/attribute_mapping.go`, add the
  `SAMLAttributeDef` struct with `Name`, `FriendlyName`, and
  `NameFormat` JSON-tagged fields, plus `EffectiveNameFormat()` method
  returning the explicit format or `DefaultNameFormat`.
- [x] 1.3 In `internal/domain/attribute_mapping.go`, add the
  `UserAttributes` struct with `Subject`, `Email`, `Name`, `Groups
  []string`, and `Custom map[string]string` fields (JSON-tagged), plus a
  `GetField(name string) string` helper covering the four well-known
  fields with `Custom` fallback for unknown names.
- [x] 1.4 Change `AttributeMapping.SAMLAttributes map[string]string` to
  `AttributeMapping.SAMLAttributeMappings map[string]SAMLAttributeDef`;
  update the JSON tag to `saml_attribute_mappings`.
- [x] 1.5 Rename `AttributeMapping.OIDCClaims` to
  `AttributeMapping.OIDCClaimMappings`; update the JSON tag to
  `oidc_claim_mappings`.
- [x] 1.6 Extend `AttributeMapping.Validate()` to return a validation
  error identifying `saml_attribute_mappings.<field>.name` when any
  `SAMLAttributeDef.Name` is empty, while preserving the existing
  `nameid_format` checks and the nil-receiver-is-valid contract.

## 2. Domain tests

- [x] 2.1 In `internal/domain/attribute_mapping_test.go`, replace
  fixtures using `map[string]string` SAML attributes with
  `map[string]SAMLAttributeDef` values throughout the existing tests.
- [x] 2.2 Add a table-driven test for `EffectiveNameFormat()` covering
  explicit and defaulted cases.
- [x] 2.3 Add a table-driven test for `UserAttributes.GetField()`
  covering each well-known field, a custom-map hit, and a miss.
- [x] 2.4 Add `Validate()` test cases for: empty
  `SAMLAttributeDef.Name` (rejected with `Field` =
  `saml_attribute_mappings.email.name`), a valid full-URN
  `nameid_format`, and a nil receiver.
- [x] 2.5 Add a JSON round-trip test for `AttributeMapping` confirming
  the Phase 2 shape (`saml_attribute_mappings`, `oidc_claim_mappings`,
  `SAMLAttributeDef` object value) serialises and deserialises without
  field loss.

## 3. Service refactor

- [x] 3.1 In `internal/service/attribute_mapping.go`, rename
  `buildInternalModel` to `BuildUserAttributes` and change its return
  type to `*domain.UserAttributes`; use the default
  `{"sub":"subject","email":"email","name":"name","groups":"groups"}`
  mapping when `OIDCClaimMappings` is empty; populate `Subject`,
  `Email`, `Name` directly and route unrecognised internal field names
  to `Custom`.
- [x] 3.2 In `BuildUserAttributes`, extract groups from
  `RawOIDCClaims[<claim>]` as `[]string` when the claim is a JSON
  array, with session `Groups` as fallback; remove all `"\x00"`-based
  encoding paths.
- [x] 3.3 Update `ApplyMapping` to operate on `*domain.UserAttributes`
  (subject, email, name, groups, custom) rather than the old
  `map[string]string` model; preserve the existing transform branch
  for `Options.LowercaseEmail` (apply only to `Email`).
- [x] 3.4 Rewrite `buildSAMLAttributes` (or inline replacement) to
  iterate `mapping.SAMLAttributeMappings`, populate
  `domain.Attribute.Name`, `FriendlyName` (only when non-empty),
  `NameFormat = def.EffectiveNameFormat()`; emit single-valued
  attributes from `attrs.GetField(field)` and the `groups` field as one
  `<saml:Attribute>` with one `<saml:AttributeValue>` per element of
  `attrs.Groups`.
- [x] 3.5 Emit a DEBUG log via `logging.FromContext(ctx, s.logger)`
  whenever a mapped attribute is omitted because its source value is
  empty (or, for `groups`, the slice is empty); include `entityID`,
  `internalField`, and `samlAttrName` keys.
- [x] 3.6 Retain the existing field-clearing block
  (`UserEmail`/`UserCommonName`/`UserName`/`UserSurname`/
  `UserGivenName`/`UserScopedAffiliation`/`Groups`) on the mapped
  session when `SAMLAttributeMappings` is non-empty; do **not** clear
  fields when `SAMLAttributeMappings` is empty.
- [x] 3.7 Confirm `nameIDFormatToURN` and `getNameIDValue` are
  unchanged in behavior (persistent NameID resolution remains
  out-of-scope for this change).

## 4. Service tests

- [x] 4.1 In `internal/service/attribute_mapping_test.go`, replace all
  fixtures using `map[string]string` SAML attributes and
  null-byte-encoded groups with the Phase 2 shape
  (`map[string]SAMLAttributeDef`, `Groups []string`).
- [x] 4.2 Add a test asserting `BuildUserAttributes` populates
  well-known fields from a default mapping over a session containing
  `sub`, `email`, `name` raw claims.
- [x] 4.3 Add a test asserting a custom `oidc_claim_mappings` entry
  (`"department": "dept"`) routes the claim value into
  `UserAttributes.Custom["dept"]`.
- [x] 4.4 Add a test asserting `Groups` is returned as
  `["admin","users"]` (not null-byte-joined) when the OIDC `groups`
  claim is a JSON array.
- [x] 4.5 Add a test asserting that a missing claim with no session
  fallback leaves the corresponding `UserAttributes` field empty.
- [x] 4.6 Add a test asserting `ApplyMapping` produces one
  `domain.Attribute` per `SAMLAttributeMappings` entry with `Name`,
  `FriendlyName`, and `NameFormat` matching the `SAMLAttributeDef`
  (explicit `name_format` honoured).
- [x] 4.7 Add a test asserting `ApplyMapping` defaults missing
  `NameFormat` to `urn:oasis:names:tc:SAML:2.0:attrname-format:uri`.
- [x] 4.8 Add a test asserting `ApplyMapping` omits `FriendlyName` (as
  XML attribute) when `SAMLAttributeDef.FriendlyName` is empty.
- [x] 4.9 Add a test asserting a mapped `groups` field produces one
  `domain.Attribute` with one `domain.AttributeValue` per group.
- [x] 4.10 Add a test asserting an empty source value omits the
  corresponding attribute from the assertion and emits a captured
  DEBUG log line (use the test logger fake already used in the
  package, or `logging.NewTestLogger` if available).
- [x] 4.11 Add a test asserting `Options.LowercaseEmail = true`
  lowercases only the `Email` field and leaves `Name`/`Subject`
  untouched.
- [x] 4.12 Add a test asserting that when `SAMLAttributeMappings` is
  non-empty, the mapped session has `UserEmail`, `UserCommonName`,
  `UserName`, `UserSurname`, `UserGivenName`,
  `UserScopedAffiliation`, and `Groups` cleared.
- [x] 4.13 Add a test asserting that when `SAMLAttributeMappings` is
  empty (e.g. only `nameid_format` set), the mapped session leaves
  those fields intact.

## 5. Persistence sanity check

- [x] 5.1 Confirm `internal/repository/postgres/service_provider.go`
  marshals/unmarshals `*domain.AttributeMapping` via `encoding/json`
  with no Phase-1-specific code paths; if any Phase 1 compatibility
  shim exists, remove it.
- [x] 5.2 Run the postgres integration tests (or add one) that round-
  trip a Phase 2 mapping through `Save` and `GetByEntityID` and assert
  field equality including `SAMLAttributeDef.NameFormat`.

## 6. Mocks and consumers

- [x] 6.1 Run `make generate` and commit the regenerated `mocks/*`
  files. Confirm no interface signatures changed (only struct shapes).
- [x] 6.2 Grep the codebase for any remaining references to
  `SAMLAttributes` (Phase 1 field name) and `OIDCClaims` and update
  call sites to use the renamed fields; if no call sites exist outside
  the changed files, record that in the PR description.

## 7. Verification suite

- [x] 7.1 Run `make fmt` and commit any formatting deltas.
- [x] 7.2 Run `make build`; address any compile errors.
- [x] 7.3 Run `make lint`; resolve all warnings (zero-tolerance per
  repository constitution).
- [x] 7.4 Run `make test`; ensure every spec scenario in
  `specs/per-sp-attribute-mapping/spec.md` has a corresponding
  passing test.
- [x] 7.5 Run `make license-check`; ensure all touched Go files
  retain the AGPL-3.0-only header (new files include the required
  two-line header).

## 8. Documentation & rollout

- [x] 8.1 Update any README/CLI examples that show the Phase 1
  attribute mapping JSON shape to use `saml_attribute_mappings` with
  `SAMLAttributeDef` object values and `oidc_claim_mappings`.
- [x] 8.2 Add a CHANGELOG entry under "Unreleased" noting the
  breaking JSONB change and the required re-registration step for
  any SPs persisted under the Phase 1 layout.
- [x] 8.3 Capture the rollback caveat from `design.md` "Migration
  Plan" in the PR description: reverting after any environment has
  re-registered SPs under Phase 2 requires restoring those
  registrations.

## Implementation Notes

Two tasks were satisfied with deviations worth surfacing in PR review:

- **5.2** (Postgres round-trip integration test): the repository has
  no `internal/repository/postgres` test suite. Round-trip coverage is
  provided at the JSON layer by `TestAttributeMapping_JSONRoundTrip`
  (domain-layer unit test asserting `marshal → unmarshal → DeepEqual`
  with explicit checks that the JSON tags use the Phase 2 names).
  A real testcontainer-backed Postgres integration test belongs in a
  follow-up "introduce postgres integration test harness" change.
- **8.2** (CHANGELOG entry): this repository's `CHANGELOG.md` is
  generated by [release-please](https://github.com/googleapis/release-please)
  from conventional commits. A manual edit would conflict with the
  tooling. The breaking JSONB shape change MUST be communicated via
  the conventional commit message footer, e.g.:

  ```text
  feat(domain)!: introduce SAMLAttributeDef and UserAttributes

  BREAKING CHANGE: AttributeMapping.SAMLAttributes
  (map[string]string) is renamed to SAMLAttributeMappings
  (map[string]SAMLAttributeDef); OIDCClaims is renamed to
  OIDCClaimMappings. Any service provider persisted under the
  Phase 1 layout must be re-registered.
  ```

  release-please will emit the appropriate CHANGELOG line on the next
  release PR.

Lint sweep: three pre-existing `errcheck` warnings in
`internal/cmd/migrate.go` (unrelated to this change) were resolved
in passing to keep the verification suite at zero warnings, per the
repository constitution.
