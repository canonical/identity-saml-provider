# Proposal

## Why

Operators cannot see which service providers the bridge has registered.
The admin API serves a single SP by `entity_id` and rejects calls
without one, so there is no way to enumerate registrations. This blocks
the Juju charm action `list-service-providers`, requested in
canonical/identity-saml-provider-operator#168, which the application
onboarding workflow needs.

## What Changes

- Add a CLI subcommand `identity-saml-provider service-provider list`
  that prints every registered service provider with its `entity_id`,
  `acs_url`, `acs_binding`, and `attribute_mapping` when one is
  configured.
- Emit the standard CLI JSON envelope with `--format json`:
  `{"status":"success","data":[...]}` on success and
  `{"status":"error","error":...}` with a non-zero exit code on
  failure.
- Print a human-readable list in text mode. An empty inventory prints
  a friendly "no service providers registered" line and still exits
  with status code 0.
- Read database settings from the `SAML_PROVIDER_DB_*` environment
  variables, as the existing `service-provider create` command does.
- Sort the output by `entity_id` so repeated runs give stable results.
- Extend the repository and service layers with a list operation.

## Non-goals

- **New HTTP admin endpoint.** The admin API keeps requiring
  `entity_id`. A CLI command serves the charm through its existing
  pebble exec integration (`version`, `migrations apply`) and avoids
  a new unauthenticated admin route.
- **Pagination or filtering.** Deployments hold a small number of
  SPs, so the command returns the full list.
- **SP deletion or mutation from the CLI.** This command is read-only.
- **Changes to the admin HTTP API.** `GET /admin/service-providers`
  behaviour is unchanged.
- **Authentication on the CLI command.** The command needs database
  credentials, which already limit its use to trusted operators.

## Capabilities

### New Capabilities

- `service-provider-listing`: list all registered SAML service
  providers from the CLI, with their entity IDs and attribute mappings,
  in text and machine-readable JSON form.

### Modified Capabilities

None. The `cli-formatting` capability already covers the JSON envelope,
the `--format` flag, and stream routing; this change only follows them.

## Impact

- **CLI**: new file `internal/cmd/service_provider_list.go`, registered
  under the existing `service-provider` command. No new flags.
- **Formatter**: new list result type and text formatter in
  `internal/cmd/service_provider_formatter.go`, matching the admin
  API's `GetServiceProviderResponse` JSON shape.
- **Service layer**: `ServiceProviderService` gains `List`. Mocks
  regenerated via `make generate`.
- **Repository layer**: `ServiceProviderRepository` gains `List`,
  implemented in `internal/repository/postgres/service_provider.go`
  with `ORDER BY entity_id`.
- **HTTP API**: unchanged.
- **Database**: no migration; reads the existing `service_providers`
  table.
- **Dependencies**: no new dependencies.
- **Downstream**: the Juju charm will exec this command via pebble and
  parse the JSON envelope for its `list-service-providers` action
  (operator issue #168).
- **Docs**: README CLI section updated with the new command.

## Success Metrics

- The Juju charm action `list-service-providers` returns every
  registered SP and its entity ID by running this command via pebble.
- `service-provider list --format json` output parses as the standard
  envelope on every run, success or failure, verified by CLI tests.
- A register-then-list round trip returns the exact SP set, verified
  by tests against the service and repository layers.
- An empty database yields exit code 0 and a friendly text line,
  verified by formatter tests.
- No regression in existing CLI commands, verified by the existing
  test suite.
