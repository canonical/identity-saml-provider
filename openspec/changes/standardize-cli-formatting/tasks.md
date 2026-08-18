# Tasks: Standardize CLI formatting

## 1. Core Framework & Global Flag Setup

- [x] 1.1 Add global persistent `--format` flag (without short `-f` alias,
  per DE013) to `rootCmd` in `internal/cmd/root.go` supporting `text` and
  `json` formats. Unit test in `root_test.go`.
- [x] 1.2 Create `ResponseEnvelope[T]` struct and standard envelope JSON encoder
  helpers in `internal/cmd/formatter.go`. Unit test in `formatter_test.go`.
- [x] 1.3 Implement generic command runner `RunHandler[T any]` and error
  interceptor in `internal/cmd/runner.go` to automate envelope creation,
  error handling, and stream routing. Unit test in `runner_test.go`.

## 2. Refactor Subcommand Formatters & Flags

- [x] 2.1 Refactor `internal/cmd/sp.go` and `internal/cmd/sp_add.go` to use
  `RunHandler` and remove manual formatter calls and local format flags.
  Unit test in `sp_formatter_test.go` and `sp_test.go`.
- [x] 2.2 Refactor `internal/cmd/janitor.go` and
  `internal/cmd/janitor_pending_requests.go` to use `RunHandler` and remove
  manual formatter calls and local format flags. Unit test in
  `janitor_formatter_test.go` and `janitor_test.go`.
- [x] 2.3 Refactor `internal/cmd/migrate.go` to use `RunHandler` across
  subcommands (`up/down`, `status`, `check`) and remove local format flags.
  Unit test in `migrate_formatter_test.go` and `migrate_test.go`.

## 3. Verification Suite & Integration Testing

- [x] 3.1 Run `make build` to verify binary compilation without warnings.
- [x] 3.2 Run `make test` to verify all table-driven unit tests pass.
- [x] 3.3 Run `make lint`, `make fmt`, and `make license-check` to enforce
  codebase quality and license headers.

## 4. Documentation & Rollout

- [x] 4.1 Update CLI command help strings and documentation references to
  reflect the unified global `--format` flag and JSON envelope contracts.

## Implementation Notes

Record any deviations or notes during implementation in this section.
