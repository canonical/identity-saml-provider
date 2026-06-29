## 1. Remove the `--nameid-format` flag

- [x] 1.1 In `internal/cmd/sp_add.go`, delete the package-level
  `spNameIDFormat` variable, the `spAddCmd.Flags().StringVar(...)`
  registration that exposes `--nameid-format`, and the
  `else if spNameIDFormat != ""` branch in `buildServiceProvider()`.
  Update the doc comment on `buildServiceProvider()` so it no longer
  references `--nameid-format`. **Testing**: unit test that
  `buildServiceProvider()` with no `--attribute-mapping-file` and the
  required `--entity-id` / `--acs-url` flags returns a `ServiceProvider`
  whose `AttributeMapping` is nil.
- [x] 1.2 In `internal/cmd/sp_test.go`, drop the `"with nameid-format"`
  table-test case and remove the `cmd.Flags().StringVar(&spNameIDFormat,
  "nameid-format", "", "")` line from the test helper that re-registers
  flags on a fresh cobra command, along with the `"nameid-format"` entry
  in the reset slice. **Testing**: `go test ./internal/cmd/...` passes
  without referencing `spNameIDFormat`.

## 2. Cover the unified input path in CLI tests

- [x] 2.1 Add a unit test in `internal/cmd/sp_test.go` that writes a
  temporary file containing `{"nameid_format": "persistent"}`, invokes
  `buildServiceProvider()` with `--attribute-mapping-file` pointing at
  that path, and asserts the returned `ServiceProvider` has
  `AttributeMapping.NameIDFormat == "persistent"` and an empty
  `SAMLAttributeMappings`. **Testing**: unit test in `internal/cmd`.
- [x] 2.2 Add a unit test that asserts `buildServiceProvider()` returns
  an error whose message includes the file path when
  `--attribute-mapping-file` points at a non-existent file, and a
  separate case where the file exists but contains invalid JSON.
  **Testing**: unit test in `internal/cmd`.
- [x] 2.3 Add a unit test that asserts `buildServiceProvider()` returns
  the validation error from `sp.Validate()` when
  `--attribute-mapping-file` points at a JSON document that parses but
  describes an invalid `AttributeMapping` (for example, a SAML attribute
  with an empty `name`). **Testing**: unit test in `internal/cmd`.

## 3. Documentation & rollout

- [x] 3.1 Update the `sp add` command's `Long` help text in
  `internal/cmd/sp_add.go` and the README CLI section so that
  `--attribute-mapping-file` is documented as the single input for all
  mapping settings, with a worked example of a minimal
  `{"nameid_format": "persistent"}` file. **Testing**: `go vet` and
  `make lint` succeed; manual `identity-saml-provider sp add --help`
  inspection confirms `--nameid-format` is absent and
  `--attribute-mapping-file` is described.
- [x] 3.2 Search the rest of the repository (`docs/`, `test/`, k8s
  manifests, skaffold scripts, `Makefile`, sample JSON files) for any
  remaining `--nameid-format` reference and either remove it or convert
  it to the JSON-file pattern. **Testing**: `grep -R "nameid-format"` in
  the working tree returns only OpenSpec history (under
  `openspec/changes/archive/` and `openspec/changes/`) and SAML URN
  occurrences such as
  `urn:oasis:names:tc:SAML:2.0:nameid-format:persistent`.

## 4. Verification suite

- [x] 4.1 Run `make build` and ensure it exits 0.
- [x] 4.2 Run `make fmt` and ensure no files are reformatted in the
  resulting diff.
- [x] 4.3 Run `make lint` and ensure it exits 0 with no warnings.
- [x] 4.4 Run `make test` and ensure all packages pass, including the
  new and updated `internal/cmd` tests.
- [x] 4.5 Run `make license-check` and ensure no missing or malformed
  AGPL-3.0-only headers are reported. (No new Go files are added by
  this change, but the check guards against accidental introductions.)
- [x] 4.6 Run `openspec validate cli-unified-mapping-input` and confirm
  the change is still reported as valid.

## 5. Implementation Notes

- Task 3.2 acceptance was phrased to allow only OpenSpec history and
  SAML URN occurrences as remaining matches for `grep -R nameid-format`.
  In practice the historical Phase 1 docs under `docs/refactor/`,
  `docs/per-sp-attribute-mapping-{design,requirements}.md`, and
  `docs/plan-merge-sp-admin-cli.md` retain the flag in their archival
  text (analogous to OpenSpec history), and the V2 requirement/design
  docs that authorise the removal also reference it. These were left
  untouched because they document past or planned states rather than
  current operator guidance. `.idea/copilotDiffState.xml` (IDE state)
  was also left alone. The only operator-facing CLI doc (`README.md`)
  was updated.
- The existing `with attribute mapping file` table case (Phase 1
  `saml_attributes` shape) was rewritten as `with full attribute mapping
  file` using the current Phase 2 `saml_attribute_mappings`/
  `SAMLAttributeDef` shape so it actually exercises the parser.
