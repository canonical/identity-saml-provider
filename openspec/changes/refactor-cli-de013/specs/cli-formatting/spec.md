## ADDED Requirements

### Requirement: Command Naming and Grammar Hierarchy

The CLI SHALL ensure every command represents an explicit action verb in accordance with DE013 standards, organized under domain noun groups:
- `identity-saml-provider serve` for starting the HTTP bridge server
- `identity-saml-provider version` for displaying version information
- `identity-saml-provider service-provider create` for registering a SAML service provider
- `identity-saml-provider requests prune` for cleaning up expired pending requests
- `identity-saml-provider migrations [apply|rollback|show|check]` for database migration lifecycle management

#### Scenario: Registering a service provider with the new command structure
- **WHEN** a user executes `identity-saml-provider service-provider create --entity-id <id> --acs-url <url>`
- **THEN** the CLI registers the service provider and outputs the registration summary

#### Scenario: Cleaning up expired pending requests with the new command structure
- **WHEN** a user executes `identity-saml-provider requests prune --batch-size 100`
- **THEN** the CLI prunes expired pending requests in batches of 100

#### Scenario: Managing migrations with the new command structure
- **WHEN** a user executes `identity-saml-provider migrations apply --dsn <dsn>`
- **THEN** the CLI applies all pending database migrations

### Requirement: Subcommand Flag Standards

In accordance with DE013 flag standards, subcommands SHALL NOT offer dual short and long flags for the same option. Subcommand options SHALL use descriptive long flags without single-character short flag aliases.

#### Scenario: User provides options using long flags on subcommands
- **WHEN** a user executes `identity-saml-provider service-provider create --entity-id <id> --acs-url <url>`
- **THEN** the CLI parses the options using long flag names and executes successfully

#### Scenario: User attempts to use single-character short flag aliases
- **WHEN** a user executes `identity-saml-provider service-provider create -e <id> -a <url>`
- **THEN** the CLI rejects the unknown short flags `-e` and `-a` and exits with a non-zero status code

### Requirement: Tabular Output Formatting

When rendering tabular data in text mode (such as `identity-saml-provider migrations show`), the CLI SHALL format output according to DE013 tabular standards:
- Use two spaces for column delimiters
- Render column headers in uppercase and bold font when outputting to an interactive terminal (TTY)
- Exclude ASCII line decorations (such as `====` or `--`)
- Support the `--no-headers` flag to suppress column headers

#### Scenario: Displaying migration status with default headers
- **WHEN** a user executes `identity-saml-provider migrations show --dsn <dsn>` in text mode
- **THEN** the output contains uppercase column headers separated by two spaces with no ASCII decoration lines

#### Scenario: Displaying migration status with --no-headers
- **WHEN** a user executes `identity-saml-provider migrations show --dsn <dsn> --no-headers` in text mode
- **THEN** the output contains only the data rows with column headers omitted

## MODIFIED Requirements

### Requirement: Centralized Execution Error Interception

The CLI execution handler SHALL intercept all command errors and Cobra flag parsing errors at the root level. When `--format json` is active, the CLI SHALL suppress default Cobra error formatting and emit the standardized JSON error envelope to stdout. When `--format text` is active, the CLI SHALL emit error messages prefixed with `error: ` (lowercase) using passive, direct phrasing ("cannot ...") without contractions.

#### Scenario: Missing required flags in JSON mode
- **WHEN** a user executes a command without supplying required flags in `--format json`
- **THEN** the CLI outputs `{"status": "error", "error": "<flag error>"}` to stdout and exits with status code 1

#### Scenario: Error formatting in text mode
- **WHEN** a command fails in `--format text` mode
- **THEN** the CLI writes `error: cannot ...` to stderr and exits with status code 1
