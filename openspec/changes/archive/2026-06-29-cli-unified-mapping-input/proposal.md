## Why

The `sp add` CLI exposes two overlapping flags for attribute mapping
configuration — `--attribute-mapping-file` and `--nameid-format` — and silently
prefers the file when both are provided. Operators cannot tell which flag won,
which means a `--nameid-format` value can be quietly discarded with no warning
in logs or output. As Phase 2 of per-SP attribute mapping introduces richer
mapping fields (rich SAML attribute definitions, cross-map validation,
options), continuing to cherry-pick individual mapping fields as CLI flags
would expand this confusion without bound. Collapsing to a single input path
now keeps the CLI contract small and unambiguous before more fields land.

This change addresses Problem Statement P7 and Functional Requirement FR-8 of
`docs/requirement/per-sp-attribute-mapping-v2-requirements.md`, and implements
Phase 2e / Decision D10 of `docs/design/per-sp-attribute-mapping-v2-design.md`.

## What Changes

- **BREAKING**: Remove the `--nameid-format` flag from `identity-saml-provider
  sp add`. Operators who previously used `--nameid-format <value>` must instead
  pass `--attribute-mapping-file <path>` referencing a JSON document that
  contains `{"nameid_format": "<value>"}` (other mapping fields optional).
- Make `--attribute-mapping-file` the sole entry point for every mapping
  configuration field, including `nameid_format`. No new flags are introduced
  for individual mapping fields.
- A mapping file whose only content is `{"nameid_format": "<value>"}` is
  accepted as valid input and produces a registered SP whose
  `AttributeMapping.NameIDFormat` is set and whose `SAMLAttributeMappings` is
  empty (treated as unmapped for attribute emission, per FR-6).
- Update CLI help text and README CLI documentation to describe the single
  input path.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `per-sp-attribute-mapping`: Replace the requirement (and scenarios) that
  describes the `sp add` CLI input surface so that `--attribute-mapping-file`
  is the sole input path for mapping configuration, the `--nameid-format`
  flag is removed, and a minimal `{"nameid_format": "<value>"}` file is
  accepted.

## Impact

- **Code**: `internal/cmd/sp_add.go` (remove the `--nameid-format` flag,
  the `spNameIDFormat` variable, and the `else if spNameIDFormat != ""`
  branch in `buildServiceProvider`), `internal/cmd/sp_test.go` (drop the
  `--nameid-format` table case, add a minimal-JSON-file case, and remove
  `spNameIDFormat` from the test helper that re-registers flags).
- **Docs**: `README.md` CLI section.
- **No changes** to: `internal/domain/**`, `internal/repository/**`,
  `internal/service/**`, `internal/handler/**`, `internal/app/**`,
  database migrations, persisted JSONB schema, admin HTTP API, or the
  `AttributeMapping` Go struct. `NameIDFormat` is already a field on
  `domain.AttributeMapping` and the JSON-file path already populates it.
- **Dependencies**: None added or removed.
- **Coordination**: Independent of the in-flight
  `persistent-nameid-resolution` change and of Phase 2a/2b/2c/2d work. Can
  land in any order relative to those changes.

## Non-goals

- Fixing persistent NameID semantics (still Phase 2b — operators selecting
  `nameid_format: persistent` via the JSON file will continue to receive the
  raw OIDC `sub` until that change lands).
- Introducing per-field CLI flags for any other mapping configuration value
  (`saml_attribute_mappings`, `oidc_claim_mappings`, `options.*`, etc.).
- Adding GET/PUT/DELETE admin HTTP endpoints for attribute mapping (Phase 2d).
- Rewriting `SAMLAttributeMappings` as `SAMLAttributeDef` (Phase 2a) or any
  domain/repository/service-layer refactor.
- Adding configuration through environment variables, YAML files, or any
  other channel beyond the existing JSON mapping file.

## Success Metrics

- `identity-saml-provider sp add --help` lists exactly one mapping input
  flag (`--attribute-mapping-file`); `--nameid-format` is absent.
- `sp add --entity-id ... --acs-url ... --attribute-mapping-file f.json`
  where `f.json` is `{"nameid_format": "persistent"}` succeeds and persists
  an SP whose stored `AttributeMapping.NameIDFormat == "persistent"` and
  whose `SAMLAttributeMappings` is empty.
- `make build`, `make fmt`, `make lint`, `make test`, and
  `make license-check` all pass.
