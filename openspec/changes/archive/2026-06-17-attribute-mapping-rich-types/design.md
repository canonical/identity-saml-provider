## Context

Phase 1 of the per-SP attribute mapping feature shipped with two data
shapes that block the production requirements identified in
[`per-sp-attribute-mapping-v2-requirements.md`][reqs]:

1. `AttributeMapping.SAMLAttributes map[string]string` — one string per
   attribute, used as both `Name` and `FriendlyName`, with
   `NameFormat` hard-coded to `urn:oasis:names:tc:SAML:2.0:
   attrname-format:basic` in `internal/service/attribute_mapping.go`.
2. An internal user model represented as `map[string]string`, built by
   `buildInternalModel()`. Multi-valued `groups` are encoded as a
   null-byte–separated string and split again at SAML attribute build
   time.

Phase 2 work is split across several PRs (see `proposal.md` "Out of
scope" section). This design covers the first PR — Phase 2a — which
introduces the new domain types and refactors the mapping pipeline to
consume them. Behavior changes are deliberately limited to (a) the
default `NameFormat` flipping from `basic` to `uri` and
(b) `SAMLAttributeDef.FriendlyName` being honoured independently of
`Name`. Persistent NameID resolution, the custom assertion maker, the
admin API, cross-map validation, and CLI unification are deferred.

The project is pre-production; NFR-1 in the requirements explicitly
permits breaking the persistence layout without a migration.
Implementation sources of truth referenced throughout this design:

- [Phase 2 requirements][reqs]
- [Phase 2 design][design] §3.1, §3.2, §3.3, §3.4

[reqs]: ../../../docs/requirement/per-sp-attribute-mapping-v2-requirements.md
[design]: ../../../docs/design/per-sp-attribute-mapping-v2-design.md

## Goals / Non-Goals

**Goals**

- Replace `map[string]string` SAML attribute config with a richer
  `map[string]SAMLAttributeDef` carrying `Name`, `FriendlyName`, and
  `NameFormat`.
- Replace the `map[string]string` internal user model with a typed
  `UserAttributes` struct that holds `Groups` as a native `[]string`.
- Establish a single `DefaultNameFormat` constant for emitted
  attributes lacking an explicit `NameFormat`.
- Extend `AttributeMapping.Validate` to require a non-empty
  `SAMLAttributeDef.Name` per entry while keeping the existing
  `nameid_format` checks.
- Land the change with no signature changes to existing service or
  repository interfaces, so this PR does not force mock regeneration
  cascades beyond the mapping subsystem.

**Non-Goals**

- Persistent NameID storage or resolution (FR-3 / problem P1).
- Custom `SPAssertionMaker` and removal of the field-clearing
  workaround (FR-6 / problem P5).
- Admin GET/PUT/DELETE endpoints for SP attribute mappings
  (FR-4, FR-5 / problem P3).
- Cross-map semantic validation between `oidc_claim_mappings` and
  `saml_attribute_mappings` (FR-7 / problem P6).
- CLI flag unification — removal of `--nameid-format` (FR-8 / problem
  P7).
- Migration of any Phase 1 JSONB rows (NFR-1 permits dropping them).

## Decisions

### D1 — Use a typed `UserAttributes` struct, not a `map[string]string`

| Option | Verdict | Why |
|---|---|---|
| A. Keep `map[string]string` | rejected | Forces null-byte encoding for `groups`; no compiler help against typos; opaque to readers. |
| B. `UserAttributes{Subject, Email, Name, Groups, Custom}` struct | **chosen** | Type safety, explicit shape, native `[]string` for multi-valued fields, `Custom` map preserves extensibility for non-standard OIDC claims. |

Mirrors the v2 design document §3.1. The `Custom` map remains the
escape hatch for fields populated through custom
`oidc_claim_mappings` targets — without it, every non-well-known
claim would force a struct change.

### D2 — `SAMLAttributeDef` with explicit `NameFormat`, defaulted to `uri`

| Option | Verdict | Why |
|---|---|---|
| A. Keep `map[string]string` and add side maps for FriendlyName / NameFormat | rejected | Three parallel maps keyed identically is worse than one struct value; admins must keep keys in sync across maps. |
| B. `SAMLAttributeDef{Name, FriendlyName, NameFormat}` value | **chosen** | One canonical place for per-attribute metadata; matches `<saml:Attribute>` XML shape 1:1. |
| C. Default `NameFormat` to `basic` (Phase 1's hard-coded value) | rejected | Phase 2 requirements (FR-2) require `uri` as the default; `basic` is the legacy value most LDAP-style SPs accept anyway, but new SPs (NetSuite, ServiceNow, OID-based deployments) expect `uri`. |
| D. Default `NameFormat` to `uri` | **chosen** | Matches the v2 design constant `DefaultNameFormat`; SPs needing `basic` declare it explicitly. |

`EffectiveNameFormat()` resolves the default at attribute build time
rather than at config load time, so persisted configs remain readable
and the default can evolve without rewriting stored JSONB.

`FriendlyName` is emitted only when non-empty. Today's code copies
`Name` into `FriendlyName` unconditionally — that's incorrect for SPs
that match by `FriendlyName` separately. Emitting an empty
`FriendlyName` attribute is structurally legal but pollutes the
assertion XML; omitting it is cleaner and matches `crewjam/saml`'s
default behavior.

### D3 — Break the JSONB layout; no compatibility shim

| Option | Verdict | Why |
|---|---|---|
| A. Custom `UnmarshalJSON` accepting both Phase 1 (`string` value) and Phase 2 (object value) | rejected | Pre-production scope (NFR-1) explicitly waives backcompat; the shim is dead code from day one. |
| B. Data migration converting Phase 1 rows to Phase 2 shape | rejected | Same reason; also no in-place way to back-fill `FriendlyName`/`NameFormat` from a single string. |
| C. Hard break — Phase 1 rows fail to deserialise | **chosen** | Matches design D3. Operators re-register the small number of dev/staging SPs. Avoids accumulating compatibility debt. |

The change does not add a Goose migration: the `attribute_mapping`
column type (`JSONB`) is unchanged. The only impact is that previously
written values become unreadable. The proposal documents this as the
operational impact.

### D4 — Field-clearing stays; assertion maker work is a separate PR

| Option | Verdict | Why |
|---|---|---|
| A. Replace field-clearing with `SPAssertionMaker` in this PR | rejected | Doubles the PR diff and couples a high-value-but-orthogonal piece of work (custom assertion construction) to a data-model refactor reviewers already need to walk carefully. |
| B. Keep field-clearing exactly as-is | **chosen** | Behavior parity for default-suppression; later change swaps the mechanism and removes the workaround in one self-contained PR. |

This is the explicit boundary between Phase 2a and the later
`SPAssertionMaker` change. The spec records that suppression is part of
this capability today and that the mechanism is an implementation
detail subject to change.

### D5 — Rename `OIDCClaims` to `OIDCClaimMappings` for consistency

| Option | Verdict | Why |
|---|---|---|
| A. Keep `OIDCClaims` and rename only `SAMLAttributes → SAMLAttributeMappings` | rejected | Inconsistent naming (`*Claims` vs `*AttributeMappings`) for two symmetric maps; future readers will trip on it. |
| B. Rename both to `OIDCClaimMappings` and `SAMLAttributeMappings` | **chosen** | Matches the v2 design (§3.3) and the JSON tags both sides will use externally. One break for both; cheaper than two separate breaks later. |

### D6 — `Validate` adds structural check only; defer cross-map validation

| Option | Verdict | Why |
|---|---|---|
| A. Include FR-7 cross-map validation in this PR | rejected | FR-7 has its own scenario matrix and error-message ergonomics work — bundling it inflates the PR and conflates two distinct review concerns. |
| B. Add only `SAMLAttributeDef.Name` non-empty + existing `NameIDFormat` rules | **chosen** | Keeps the validation surface aligned with the data-model change; FR-7 lands as a separate, focused PR. |

The spec explicitly calls out that semantic cross-map validation is
out of scope for this version of the capability.

## Risks / Trade-offs

- **Operational risk: every dev/staging SP must be re-registered.**
  → Mitigation: the proposal documents this; the rollout plan below
  includes a "re-register all SPs" runbook step. Production is unaffected
  (pre-production NFR-1).

- **Behavioral risk: default `NameFormat` flips from `basic` to `uri`.**
  Any SP that was implicitly receiving `basic` and validates it strictly
  will reject the new assertion. → Mitigation: the spec scenario locks
  the new default explicitly; SPs needing `basic` declare it in their
  config. Detection in dev/staging during re-registration sweep.

- **Behavioral risk: `FriendlyName` is no longer auto-populated from
  `Name`.** SPs that match by `FriendlyName` instead of `Name` will see
  the field disappear unless config declares it. → Mitigation:
  acceptance scenario covers this; mapping JSON examples in the
  reference docs already show `friendly_name` populated explicitly.

- **Review risk: ~600–800 LOC of diff across domain, service, tests,
  and mocks.** → Mitigation: confine the change to the mapping
  subsystem; no signature changes on `ServiceProviderRepository` or
  `MappingService` interfaces (only field shape changes on the domain
  value). Reviewers can split their pass into "domain types" then
  "service refactor" then "test fixtures".

- **Test risk: the existing service-level tests use the
  `map[string]string` shape heavily.** → Mitigation: rewrite fixtures as
  part of the same PR; this is mechanical. The acceptance scenarios in
  the spec cover the new behaviors that the rewrite must preserve.

- **Latent risk: `MappingService.ApplyMapping` still emits the wrong
  persistent NameID** (P1 unresolved). → Acknowledged and explicitly out
  of scope; the next PR addresses it. No regression vs. Phase 1.

## Migration Plan

1. **Implementation order within the PR**:
   1. Add `SAMLAttributeDef`, `UserAttributes`, and
      `DefaultNameFormat` to `internal/domain/attribute_mapping.go`.
   2. Change `AttributeMapping` field types and rename fields. Update
      `Validate()`.
   3. Refactor `internal/service/attribute_mapping.go`:
      `buildInternalModel` → `BuildUserAttributes`; `buildSAMLAttributes`
      consumes `SAMLAttributeDef`; drop the null-byte encoding for
      groups; keep field-clearing.
   4. Update fixtures in `internal/domain/attribute_mapping_test.go`
      and `internal/service/attribute_mapping_test.go`.
   5. Run `make generate` (no interface changes are expected, but
      regeneration is part of the project's standard procedure).
   6. Run `make build && make fmt && make lint && make test &&
      make license-check`.

2. **Operational rollout**:
   - Before merging: confirm no production deployment depends on Phase
     1 JSONB rows. (Per NFR-1: project is pre-production, so this is
     procedural.)
   - After merging in any environment with existing Phase 1 SPs:
     delete and re-register each SP using the Phase 2 JSON format. The
     reference document (`docs/design/per-sp-attribute-mapping-v2-design.md`
     §A) shows example configurations.

3. **Rollback strategy**:
   - This change has no database migration; rollback is purely code
     revert. After revert, JSONB rows written under the Phase 2 format
     become unreadable to the reverted code (same failure mode as
     forward break). Therefore: do not revert in an environment whose
     SPs have been re-registered under Phase 2 without also restoring
     the pre-Phase-2 SP registrations.

4. **Verification gates** (per repository `copilot-instructions.md`):
   - `make build` — passes
   - `make fmt` — clean
   - `make lint` — zero warnings
   - `make test` — all scenarios in
     `specs/per-sp-attribute-mapping/spec.md` covered
   - `make license-check` — clean

## Open Questions

None blocking. Two items deferred to follow-up changes for the
record:

- Whether `Validate` should also reject `oidc_claim_mappings` values
  that target neither well-known fields nor consistent custom keys
  (FR-7). Tracked for the validation-focused follow-up change.
- Whether `SAMLAttributeDef` should grow an `Encode` option (string vs.
  XSD typed values). Not required by any current SP requirement; will
  be revisited only if a real consumer asks for it.
