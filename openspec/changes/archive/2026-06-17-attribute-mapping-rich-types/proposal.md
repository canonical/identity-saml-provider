## Why

Phase 1 of the per-SP attribute mapping feature stores SAML attribute
configuration as `map[string]string` and represents the internal user model
as an untyped `map[string]string`. This shape blocks two production
requirements identified in
[`per-sp-attribute-mapping-v2-requirements.md`][reqs]:

- **P2 — SAML attribute metadata is incomplete.** Real-world SPs (NetSuite,
  ServiceNow, Shibboleth-based systems) require independent `Name`,
  `FriendlyName`, and `NameFormat` per attribute. The current code
  hard-codes `NameFormat=basic` and copies `Name` into `FriendlyName`, so
  these SPs cannot be onboarded.
- **P4 — Internal user model lacks structure.** The
  `map[string]string` internal model forces a `\x00` null-byte encoding
  for multi-valued claims (groups), is unsafe to extend, and produces
  typo-prone callers.

This change is the foundation for the rest of Phase 2 — persistent NameID,
custom `SPAssertionMaker`, and the admin GET/PUT/DELETE endpoints all
consume the new data types. Landing this slice first unblocks the
remainder of the v2 work and lets every subsequent PR review one concern
at a time.

## What Changes

- **BREAKING** Replace `AttributeMapping.SAMLAttributes
  map[string]string` with `AttributeMapping.SAMLAttributeMappings
  map[string]SAMLAttributeDef`. The JSONB column shape stored in
  `service_providers.attribute_mapping` changes incompatibly. Per
  [requirements NFR-1][reqs] and [design D3][design], no backward
  compatibility is implemented — Phase 1 data is discarded and any
  registered SP must be re-registered.
- **BREAKING** Rename `AttributeMapping.OIDCClaims map[string]string`
  to `AttributeMapping.OIDCClaimMappings map[string]string` for naming
  consistency with `SAMLAttributeMappings`.
- Introduce `SAMLAttributeDef{Name, FriendlyName, NameFormat}` with
  `DefaultNameFormat =
  "urn:oasis:names:tc:SAML:2.0:attrname-format:uri"` and
  `EffectiveNameFormat()` helper (FR-2).
- Introduce `UserAttributes{Subject, Email, Name, Groups, Custom}` as the
  internal user model returned by `BuildUserAttributes()`. Multi-valued
  `Groups` is `[]string` natively — the `\x00` null-byte encoding hack is
  removed (FR-1).
- Refactor `MappingService` so `buildInternalModel()` becomes
  `BuildUserAttributes()` returning `*UserAttributes`, and
  `buildSAMLAttributes()` consumes `SAMLAttributeDef` per attribute,
  emitting one `<saml:Attribute>` with multiple `<saml:AttributeValue>`
  children for `Groups`.
- Strengthen `AttributeMapping.Validate()` to require non-empty
  `SAMLAttributeDef.Name` for every entry (NFR-5, structural validation
  only; cross-map semantic validation is out of scope and addressed by
  a later change).
- Default `NameFormat` for mapped attributes changes from `basic`
  (hard-coded today) to `uri` (new explicit default per FR-2). SPs that
  rely on `basic` must declare it explicitly in their config.
- Field-clearing workaround in `MappingService.ApplyMapping()` is
  **retained** in this change. Removal is deferred to the
  `SPAssertionMaker` change.

## Capabilities

### New Capabilities

- `per-sp-attribute-mapping`: Per-SP configuration that maps OIDC ID
  token claims into SAML assertion attributes and the NameID. This
  capability owns the `AttributeMapping` domain type, the internal
  `UserAttributes` user model, the `SAMLAttributeDef` per-attribute
  metadata, and the `MappingService.ApplyMapping` pipeline. Phase 2a
  establishes the data model; subsequent changes extend it with
  persistent NameID, custom assertion construction, admin API
  endpoints, and cross-map validation.

### Modified Capabilities

None — `openspec/specs/` is currently empty, so this change introduces
the first version of the capability rather than modifying an existing
spec.

## Impact

**Code (in scope for this change)**

- `internal/domain/attribute_mapping.go` — new `SAMLAttributeDef`,
  `UserAttributes`, `DefaultNameFormat`; `AttributeMapping` field rename
  and type change; expanded `Validate()`.
- `internal/service/attribute_mapping.go` — `buildInternalModel` →
  `BuildUserAttributes`; `buildSAMLAttributes` consumes
  `SAMLAttributeDef`; groups handled as `[]string`; field-clearing block
  retained.
- `internal/domain/attribute_mapping_test.go`,
  `internal/service/attribute_mapping_test.go` — all fixtures updated.
- `mocks/` — regenerated via `make generate` (no interface signature
  changes are introduced by this change, but mock regeneration is part
  of the project's standard procedure).

**Persistence**

- JSONB shape in `service_providers.attribute_mapping` changes
  incompatibly. No database migration is added — the column type
  (`JSONB`) is unchanged. Any rows written by Phase 1 will fail to
  deserialize after this change.

**Operational**

- Every existing SP registration (dev, staging, demo environments)
  must be deleted and re-registered using the Phase 2 JSON format.
  Production is unaffected — project is pre-production (NFR-1).
- SPs that previously received `NameFormat=basic` implicitly will now
  receive `NameFormat=uri` unless their config declares `basic`
  explicitly. Re-registration is the remediation path.

**Out of scope (deferred to later changes)**

- Persistent NameID storage and resolution (P1 / FR-3).
- Custom `SPAssertionMaker` and removal of the field-clearing
  workaround (P5 / FR-6).
- Admin GET/PUT/DELETE endpoints (P3 / FR-4 / FR-5).
- Cross-map semantic validation between `oidc_claim_mappings` and
  `saml_attribute_mappings` (P6 / FR-7).
- CLI flag unification, removal of `--nameid-format` (P7 / FR-8).

[reqs]: ../../../docs/requirement/per-sp-attribute-mapping-v2-requirements.md
[design]: ../../../docs/design/per-sp-attribute-mapping-v2-design.md
