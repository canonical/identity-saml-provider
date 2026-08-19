# Proposal: Refactor CLI Commands to Canonical Standards (DE013)

## Why

The command-line interface currently uses inconsistent command grammar, sub-optimal flag practices (such as mixing short and long flags), non-standard tabular output styling, and non-canonical error phrasing. Refactoring the CLI to align with Canonical CLI Standards (DE013) provides a predictable, concise, and professional command-line experience across all identity components.

## What Changes

- **BREAKING**: Rename `sp add` to `service-provider create`.
- **BREAKING**: Remove redundant short flags (`-e`, `-a`, `-b`) from `service-provider create` in favor of explicit long flags (`--entity-id`, `--acs-url`, `--acs-binding`).
- **BREAKING**: Rename `janitor pending-requests` to `requests prune`.
- **BREAKING**: Reorganize database migration commands from `migrate <up|down|status|check>` to `migrations [apply|rollback|show|check]`.
- Standardize tabular output in `migrations show` using uppercase headers, 2-space column separators, no ASCII line decorations, and add support for `--no-headers`.
- Standardize error output formatting to use lowercase `error: ` prefixes and replace "failed to" phrasing with passive/direct "cannot" phrasing per DE013 copy guidelines.

## Capabilities

### New Capabilities

- None

### Modified Capabilities

- `cli-formatting`: Update CLI grammar requirements, flag constraints, tabular formatting rules, and error copy guidelines to conform to DE013.

## Non-goals

- Refactoring internal domain logic, storage repositories, or HTTP API handlers.
- Adding interactive terminal prompts or complex TUI capabilities.

## Success Metrics

- 100% of CLI subcommands conform to DE013 grammar, flag, table, and copy rules.
- Verification suite (`make build`, `make test`, `make lint`, `make fmt`, `make license-check`) passes cleanly with zero warnings or errors.

## Impact

- Affected code: `internal/cmd/*.go` and `internal/cmd/*_test.go`.
- CLI consumers: Scripts and automation invoking `identity-saml-provider` will need to update subcommand names (`service-provider create`, `requests prune`, `migrations apply/rollback/show/check`).
