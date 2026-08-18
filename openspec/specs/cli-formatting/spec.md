# CLI Formatting

## Purpose

Provides a standardized CLI formatting framework across all Identity SAML
Provider subcommands, ensuring consistent response envelopes, unified global
formatting flags, and reliable stream routing for both human and machine
consumers.

## Requirements

### Requirement: Global Output Format Flag

The CLI root command SHALL provide a global persistent output format flag
`--format` supporting `text` and `json` values, defaulting to `text`. In
accordance with DE013 flag standards, no short flag (`-f`) SHALL be provided.
Individual subcommands SHALL inherit this flag and MUST NOT declare
independent format flags.

#### Scenario: User specifies output format globally

- **WHEN** a user executes any subcommand with `--format json`
- **THEN** the CLI processes the command using JSON output formatting rules

#### Scenario: User specifies an unsupported output format

- **WHEN** a user executes a subcommand with an invalid format option (e.g.
  `--format xml`)
- **THEN** the CLI returns an error indicating the format option is unsupported
  and exits with a non-zero status code

### Requirement: Standardized JSON Response Envelope

When executing commands with `--format json`, all subcommand outputs SHALL be
wrapped in a standardized JSON response envelope. On successful execution, the
envelope SHALL contain `"status": "success"` and a `"data"` key holding the
result payload. On failed execution, the envelope SHALL contain `"status":
"error"` and an `"error"` key holding the error message string.

#### Scenario: Successful command execution in JSON mode

- **WHEN** a subcommand executes successfully with `--format json`
- **THEN** the output emitted to stdout is a valid JSON document containing
  `{"status": "success", "data": ...}` and the process exits with status
  code 0

#### Scenario: Failed command execution in JSON mode

- **WHEN** a subcommand execution fails or receives invalid arguments with
  `--format json`
- **THEN** the output emitted to stdout is a valid JSON document containing
  `{"status": "error", "error": "<message>"}` and the process exits with status
  code 1

### Requirement: Stream Routing Strategy

The CLI SHALL route outputs based on the active output format. In JSON mode
(`--format json`), all structured JSON response envelopes (success and error)
SHALL be written to stdout, and diagnostic logs or operational warnings SHALL
be written to stderr. In Text mode (`--format text`), successful command
summaries SHALL be written to stdout, and error messages SHALL be written to
stderr.

#### Scenario: JSON mode stream separation

- **WHEN** a command encounters an error or succeeds while running with
  `--format json`
- **THEN** the structured JSON output is written to stdout and background
  diagnostic logs are written to stderr

#### Scenario: Text mode stream separation

- **WHEN** a command fails while running with `--format text`
- **THEN** the human-readable error message is written to stderr and the
  process exits with status code 1

### Requirement: Centralized Execution Error Interception

The CLI execution handler SHALL intercept all command errors and Cobra flag
parsing errors at the root level. When `--format json` is active, the CLI
SHALL suppress default Cobra error formatting and emit the standardized JSON
error envelope to stdout.

#### Scenario: Missing required flags in JSON mode

- **WHEN** a user executes a command without supplying required flags in
  `--format json` mode
- **THEN** the CLI outputs `{"status": "error", "error": "<flag error>"}` to
  stdout and exits with status code 1
