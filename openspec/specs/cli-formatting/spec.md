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
error envelope to stdout. When `--format text` is active, the CLI SHALL emit
error messages prefixed with `error: ` (lowercase) using passive, direct
phrasing ("cannot ...") without contractions.

#### Scenario: Missing required flags in JSON mode

- **WHEN** a user executes a command without supplying required flags in
  `--format json`
- **THEN** the CLI outputs `{"status": "error", "error": "<flag error>"}` to
  stdout and exits with status code 1

#### Scenario: Error formatting in text mode

- **WHEN** a command fails in `--format text` mode
- **THEN** the CLI writes `error: cannot ...` to stderr and exits with status
  code 1

### Requirement: Command Naming and Grammar Hierarchy

The CLI SHALL ensure every command represents an explicit action verb in
accordance with DE013 standards, organized under domain noun groups:
- `identity-saml-provider serve` for starting the HTTP bridge server
- `identity-saml-provider version` for displaying version information
- `identity-saml-provider service-provider create` for registering a SAML
  service provider
- `identity-saml-provider requests prune` for cleaning up expired pending
  requests
- `identity-saml-provider migrations [apply|rollback|show|check]` for
  database migration lifecycle management

#### Scenario: Registering a service provider with the new command structure

- **WHEN** a user executes `identity-saml-provider service-provider create
  --entity-id <id> --acs-url <url>`
- **THEN** the CLI registers the service provider and outputs the registration
  summary

#### Scenario: Cleaning up expired pending requests with the new command structure

- **WHEN** a user executes `identity-saml-provider requests prune
  --batch-size 100`
- **THEN** the CLI prunes expired pending requests in batches of 100

#### Scenario: Managing migrations with the new command structure

- **WHEN** a user executes `identity-saml-provider migrations apply
  --dsn <dsn>`
- **THEN** the CLI applies all pending database migrations

### Requirement: Subcommand Flag Standards

In accordance with DE013 flag standards, subcommands SHALL NOT offer dual short
and long flags for the same option. Subcommand options SHALL use descriptive
long flags without single-character short flag aliases.

#### Scenario: User provides options using long flags on subcommands

- **WHEN** a user executes `identity-saml-provider service-provider create
  --entity-id <id> --acs-url <url>`
- **THEN** the CLI parses the options using long flag names and executes
  successfully

#### Scenario: User attempts to use single-character short flag aliases

- **WHEN** a user executes `identity-saml-provider service-provider create
  -e <id> -a <url>`
- **THEN** the CLI rejects the unknown short flags `-e` and `-a` and exits with
  a non-zero status code

### Requirement: Tabular Output Formatting

When rendering tabular data in text mode (such as
`identity-saml-provider migrations show`), the CLI SHALL format output
according to DE013 tabular standards:
- Use two spaces for column delimiters
- Render column headers in uppercase and bold font when outputting to an
  interactive terminal (TTY)
- Exclude ASCII line decorations (such as `====` or `--`)
- Support the `--no-headers` flag to suppress column headers

#### Scenario: Displaying migration status with default headers

- **WHEN** a user executes `identity-saml-provider migrations show --dsn <dsn>`
  in text mode
- **THEN** the output contains uppercase column headers separated by two spaces
  with no ASCII decoration lines

#### Scenario: Displaying migration status with --no-headers

- **WHEN** a user executes `identity-saml-provider migrations show --dsn <dsn>
  --no-headers` in text mode
- **THEN** the output contains only the data rows with column headers omitted
