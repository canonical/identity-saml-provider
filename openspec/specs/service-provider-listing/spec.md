# Service Provider Listing Specification

## Purpose

Lets operators list every registered SAML service provider from the
CLI, with entity IDs and attribute mappings, in text and
machine-readable JSON form. The Juju charm consumes the JSON output
for its `list-service-providers` action.

## Requirements

### Requirement: List all registered service providers

The CLI SHALL provide a `service-provider list` command that returns
every registered service provider. Each entry SHALL contain
`entity_id`, `acs_url`, and `acs_binding`. When a service provider
has an attribute mapping, the entry SHALL also contain
`attribute_mapping` in the same JSON shape the admin API returns. The
command SHALL require no positional arguments or required flags.

#### Scenario: List returns all registered service providers

- **WHEN** an operator runs `service-provider list` and two service
  providers are registered
- **THEN** the output SHALL contain both service providers
- **AND** each entry SHALL contain `entity_id`, `acs_url`, and
  `acs_binding`

#### Scenario: Entry includes attribute_mapping when configured

- **WHEN** an operator runs `service-provider list --format json` and
  a registered service provider has an attribute mapping
- **THEN** that entry SHALL contain `attribute_mapping`
- **AND** its shape SHALL match the shape accepted at registration

#### Scenario: Entry omits attribute_mapping when none is configured

- **WHEN** an operator runs `service-provider list --format json` and
  a registered service provider has no attribute mapping
- **THEN** that entry SHALL NOT contain an `attribute_mapping` field

### Requirement: Deterministic ordering

The command SHALL sort the listed service providers by `entity_id`.
Repeated runs against unchanged data SHALL produce the same order, so
callers can compare or cache output safely.

#### Scenario: Output is sorted by entity_id

- **WHEN** an operator runs `service-provider list` and service
  providers with entity IDs `b.example.com` and `a.example.com` are
  registered
- **THEN** the entry for `a.example.com` SHALL appear before the
  entry for `b.example.com`

#### Scenario: Repeated runs produce identical ordering

- **WHEN** an operator runs `service-provider list` twice without
  changing registrations
- **THEN** both runs SHALL list the service providers in the same
  order

### Requirement: Empty inventory is a success

When no service providers are registered, the command SHALL succeed
and exit with status code 0. In JSON mode, `data` SHALL be an empty
array, not `null`. In text mode, the command SHALL print a friendly
"no service providers registered" message instead of an error.

#### Scenario: JSON mode returns an empty array

- **WHEN** an operator runs `service-provider list --format json` and
  no service providers are registered
- **THEN** the output SHALL be
  `{"status":"success","data":[]}`
- **AND** the process SHALL exit with status code 0

#### Scenario: Text mode prints a friendly empty message

- **WHEN** an operator runs `service-provider list` and no service
  providers are registered
- **THEN** stdout SHALL state that no service providers are
  registered
- **AND** the process SHALL exit with status code 0

### Requirement: Standard JSON envelope

With `--format json`, the command SHALL follow the standard CLI
envelope from the `cli-formatting` capability. Success SHALL produce
`{"status":"success","data":[...]}` on stdout. Failure SHALL produce
`{"status":"error","error":"<message>"}` on stdout and a non-zero
exit code. In text mode, errors SHALL go to stderr.

#### Scenario: Successful JSON run uses the success envelope

- **WHEN** an operator runs `service-provider list --format json`
  and the command succeeds
- **THEN** stdout SHALL be a JSON document with `status` set to
  `success` and `data` holding the list
- **AND** the process SHALL exit with status code 0

#### Scenario: Failed JSON run uses the error envelope

- **WHEN** an operator runs `service-provider list --format json`
  and the command fails
- **THEN** stdout SHALL be a JSON document with `status` set to
  `error` and `error` holding the message
- **AND** the process SHALL exit with a non-zero status code

#### Scenario: Failed text run writes to stderr

- **WHEN** an operator runs `service-provider list` in text mode and
  the command fails
- **THEN** the error message SHALL be written to stderr
- **AND** the process SHALL exit with a non-zero status code

### Requirement: Database configuration from environment

The command SHALL read database connection settings from the
`SAML_PROVIDER_DB_*` environment variables, as the other
`service-provider` subcommands do. Missing or invalid settings SHALL
fail the command with an error before any query runs.

#### Scenario: Missing database settings fail the command

- **WHEN** an operator runs `service-provider list` without valid
  `SAML_PROVIDER_DB_*` settings
- **THEN** the command SHALL fail with a configuration error
- **AND** the process SHALL exit with a non-zero status code

### Requirement: Database failure surfaces an error

When the database is unreachable or the query fails, the command
SHALL fail with a non-zero exit code. The error output SHALL state
that the list operation failed. The command SHALL NOT print a partial
list.

#### Scenario: Unreachable database yields a non-zero exit

- **WHEN** an operator runs `service-provider list --format json`
  and the database is unreachable
- **THEN** stdout SHALL contain the JSON error envelope
- **AND** the process SHALL exit with a non-zero status code

### Requirement: Read-only operation

The command SHALL NOT create, update, or delete any persisted state.
It SHALL only read the service provider inventory.

#### Scenario: Listing leaves registrations unchanged

- **WHEN** an operator runs `service-provider list` any number of
  times
- **THEN** the registered service providers and their attribute
  mappings SHALL remain unchanged

## Backward Compatibility

This capability is additive. It changes no existing command, endpoint, or
output shape. The admin HTTP API keeps requiring `entity_id` for
`GET /admin/service-providers`.
