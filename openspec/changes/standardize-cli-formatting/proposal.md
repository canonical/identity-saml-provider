# Proposal: Standardize CLI formatting

## Why

Currently, the subcommands (`sp`, `janitor`, `migrate`) within
`identity-saml-provider` implement divergent output formatting, individual
`--format` flags, and inconsistent error handling. When executing commands
with `--format json`, errors can result in raw text messages written to
output streams, breaking downstream automated JSON parsers such as `jq` or
CI/CD pipelines.

Standardizing CLI flag definitions, response envelopes, and stream routing
across all subcommands creates a predictable, robust interface for both
interactive terminal users and programmatic consumers.

## What Changes

- **Global `--format` Flag**: Centralize the `--format` persistent flag
  on the root CLI command with allowed options `text` (default) and `json`.
  In accordance with Canonical CLI standards (DE013), only the long flag
  `--format` is provided (no short `-f` alias). Remove redundant format flags
  from individual subcommands (`sp`, `janitor`, `migrate`).
- **Standardized Response Envelopes**: In JSON mode (`--format json`), all
  command outputs are formatted into a consistent envelope containing
  `status`, `data` (on success), or `error` (on failure).
- **Centralized Command Runner**: Introduce a generic `RunHandler[T any]`
  execution wrapper. Subcommands return `(T, error)` directly without writing
  output or calling formatters manually. `RunHandler` automatically formats
  success payloads or errors into standard envelopes based on `--format`.
- **Stream Routing Strategy**:
  - In JSON mode (`--format json`), write all structured JSON envelopes (both
    success and error) to `stdout` so automated pipelines always receive
    valid JSON from `stdout`. Route diagnostic logs, telemetry, and background
    warnings to `stderr`.
  - In Text mode (`--format text`), write success messages to `stdout` and
    error messages to `stderr` following standard UNIX redirection
    conventions.
- **Centralized Error Interceptor**: Intercept Cobra execution and flag errors
  at the root command level to ensure failed executions automatically output
  the standard JSON error envelope when in JSON mode, with a non-zero exit
  code (`1`).
- **Simplified Formatter Interfaces**: Refactor subcommand formatters so they
  only deal with successful payload data formatting in text mode.

## Non-goals

- Adding new CLI subcommands or altering business logic in existing
  subcommands (`sp add`, `janitor pending-requests`, `migrate`).
- Introducing complex error taxonomies or error codes in the JSON error
  envelope (simple string messages are used initially).
- Changing log output formats of logging frameworks (Zap, Goose logs)
  redirected to `stderr`.

## Success Metrics

- 100% of CLI subcommands support the global `--format` flag.
- 100% of JSON output from any CLI subcommand adheres to the
  `{ "status": "success", "data": ... }` or
  `{ "status": "error", "error": ... }` envelope structure.
- Piping `identity-saml-provider <command> --format json` directly to `jq`
  never fails due to malformed JSON or unformatted error strings on `stdout`.

## Capabilities

### New Capabilities

- `cli-formatting`: Defines the standardized CLI output formatting rules,
  global `--format` flag, response envelope schema, stream routing, and error
  interceptor contracts.

### Modified Capabilities

None.

## Impact

- **Affected Code**: `internal/cmd/root.go`, `internal/cmd/runner.go`,
  `internal/cmd/sp.go`, `internal/cmd/sp_add.go`, `internal/cmd/janitor.go`,
  `internal/cmd/janitor_pending_requests.go`, `internal/cmd/migrate.go`,
  `internal/cmd/*_formatter.go`.
- **APIs/CLI**: Global `--format` flag replaces local persistent subcommand
  flags. Response JSON structure updated to envelope format across all
  subcommands.
