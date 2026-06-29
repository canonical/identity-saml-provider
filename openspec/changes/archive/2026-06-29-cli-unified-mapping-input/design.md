## Context

The `sp add` Cobra command in `internal/cmd/sp_add.go` currently exposes two
flags that both write into `domain.AttributeMapping`:

- `--attribute-mapping-file <path>`: parses a JSON file and assigns it to
  `sp.AttributeMapping` in full.
- `--nameid-format <value>`: when the file flag is empty, constructs a
  minimal `&domain.AttributeMapping{NameIDFormat: spNameIDFormat}` from
  this flag alone.

When both flags are present, the file branch wins and `--nameid-format` is
silently dropped — there is no warning, no log line, no validation error.
`domain.AttributeMapping.NameIDFormat` is the same string field whether it
arrives via the file or the flag, so the second flag is purely a convenience
shortcut for the "only NameID format" case.

The relevant cobra state is registered in `init()` of `sp_add.go` and the
flag value flows through the package-level `spNameIDFormat` variable into
`buildServiceProvider()`. Tests in `internal/cmd/sp_test.go` exercise the
flag directly through a copied cobra flag-set and through the table case
named `"with nameid-format"`.

No other component in the codebase (handlers, services, repositories,
migrations, docs in `docs/authentication-flow/`, k8s manifests, skaffold
scripts) references the flag. `grep -R nameid-format` confirms only
`sp_add.go`, `sp_test.go`, and historical OpenSpec/docs artifacts touch the
identifier.

## Goals / Non-Goals

**Goals:**

- Remove `--nameid-format` from the `sp add` command surface so that
  `--attribute-mapping-file` is the only CLI input for any
  `domain.AttributeMapping` field.
- Keep the minimal-NameID workflow possible by relying on the existing
  JSON path: a file containing `{"nameid_format": "<value>"}` continues to
  produce the same in-memory and persisted SP as the old flag.
- Keep the change strictly local to the CLI package. No domain, repository,
  service, handler, app-wiring, or migration code changes.

**Non-Goals:**

- Adding a friendlier or guided error message when the removed flag is
  supplied. The default cobra `Error: unknown flag: --nameid-format`
  behavior is sufficient.
- Adding any deprecation period, alias, or compatibility shim.
- Touching the persisted JSONB schema, `AttributeMapping` Go struct, or
  `Validate()` semantics. `NameIDFormat` already validates correctly
  whether it arrives via file or programmatically.
- Cherry-picking other mapping fields as new CLI flags now or later.

## Decisions

### D1 — Remove the flag outright; no custom unknown-flag handler

Drop the flag registration, the `spNameIDFormat` package variable, and the
`else if spNameIDFormat != ""` branch in `buildServiceProvider()`. Do not
register a `SetFlagErrorFunc`, do not parse `os.Args` manually, do not add
a `PreRunE` hook that looks for the legacy spelling.

Alternatives considered:

- *Keep the flag but mark it deprecated.* Rejected: nothing depends on the
  legacy spelling, and an alias would re-introduce the silent-priority
  problem the change is solving.
- *Add a `SetFlagErrorFunc` that intercepts the unknown-flag error and
  emits a migration hint.* Rejected per user direction in the proposal
  iteration: cobra's default unknown-flag error already exits non-zero with
  a clear message, and adding bespoke handling expands the surface for no
  measurable benefit.

### D2 — Reuse the existing file-parsing path verbatim

`buildServiceProvider()` already does:

```go
if spAttributeMappingFile != "" {
    data, _ := os.ReadFile(spAttributeMappingFile)
    var mapping domain.AttributeMapping
    _ = json.Unmarshal(data, &mapping)
    sp.AttributeMapping = &mapping
}
```

A file containing `{"nameid_format": "persistent"}` unmarshals to
`domain.AttributeMapping{NameIDFormat: "persistent"}` — byte-for-byte the
same struct the deleted branch produced. So removing the branch leaves the
minimal-NameID workflow intact; no new code is needed to preserve it.

Alternatives considered:

- *Replace the deleted branch with an inline default mapping.* Rejected:
  no caller needs it, and adding an inline default would obscure the rule
  that "all mapping config flows through the file."

### D3 — Test surface mirrors the production surface

Update `internal/cmd/sp_test.go`:

- Drop the `"with nameid-format"` table case and the `spNameIDFormat`
  reference in the helper that re-registers flags on a fresh cobra command.
- Add a `"with attribute-mapping-file containing only nameid_format"`
  case that writes a temp JSON file with `{"nameid_format": "persistent"}`
  and asserts that `buildServiceProvider()` produces an SP whose
  `AttributeMapping.NameIDFormat == "persistent"` and whose
  `SAMLAttributeMappings` is empty/nil.

Alternative: keeping a test that asserts cobra rejects `--nameid-format`.
Rejected: that is cobra's own behavior; testing it here would couple us to
cobra error strings and add no coverage of our code.

## Risks / Trade-offs

- **Risk**: Local development scripts or `Makefile` targets in someone's
  branch still pass `--nameid-format`. → **Mitigation**: cobra exits
  non-zero with `unknown flag`, surfacing the breakage immediately.
- **Risk**: An operator who knew the shortcut will need to learn the JSON
  form. → **Mitigation**: README CLI section gains the minimal-JSON
  example. Cost is one extra file on disk; benefit is one input contract.
- **Trade-off**: We lose the inline shortcut for the simplest case
  (NameID-only). → **Accepted** because the shortcut's existence is what
  enables the silent-priority bug the change targets.

## Migration Plan

No data migration. Implementation steps live in `tasks.md`; deployment is
a normal binary release. Rollback is `git revert` of the implementation
commit — no schema, persisted data, or downstream contract is touched.

## Open Questions

None.
