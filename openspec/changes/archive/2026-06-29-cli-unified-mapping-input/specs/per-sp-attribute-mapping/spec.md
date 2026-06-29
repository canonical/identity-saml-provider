## Purpose

This requirement defines how operators provide attribute mapping settings
to the `sp add` command when registering a SAML service provider. It
exists to give the command a single, unambiguous input path for every
mapping setting (including the NameID format), so that no setting can be
silently dropped by another flag taking priority.

## ADDED Requirements

### Requirement: CLI mapping configuration uses a single input path

The `identity-saml-provider sp add` command SHALL accept the service
provider's attribute mapping settings through a single flag,
`--attribute-mapping-file`. The command MUST NOT expose any additional
flag whose purpose is to set an individual mapping setting (such as the
NameID format) outside of that file.

When the operator supplies `--attribute-mapping-file`, the file MUST be a
JSON document describing the attribute mapping. Any subset of mapping
settings is permitted in that document, including a document that only
specifies the NameID format. The registered service provider stores the
parsed mapping exactly as supplied.

When the operator omits `--attribute-mapping-file`, the registered
service provider has no attribute mapping and behaves as an unmapped
service provider.

**Breaking change**: A previous `--nameid-format` flag is removed. There
is no alias, deprecation period, or compatibility shim. Operators who
need only to set the NameID format MUST supply a mapping file whose
contents are `{"nameid_format": "<value>"}`.

#### Scenario: Register service provider with a full mapping file

- **WHEN** an operator runs `sp add` with `--attribute-mapping-file
  mapping.json`, and `mapping.json` describes the NameID format, the
  SAML attribute settings, and the OIDC claim mappings
- **THEN** the command succeeds
- **AND** the registered service provider's stored attribute mapping
  reflects every setting from `mapping.json`

#### Scenario: Register service provider with a NameID-only mapping file

- **WHEN** an operator runs `sp add` with `--attribute-mapping-file
  nameid.json`, and `nameid.json` is the JSON document
  `{"nameid_format": "persistent"}`
- **THEN** the command succeeds
- **AND** the registered service provider's stored attribute mapping
  records the NameID format as `persistent`
- **AND** the stored attribute mapping carries no SAML attribute settings

#### Scenario: Register service provider without any mapping flag

- **WHEN** an operator runs `sp add` without `--attribute-mapping-file`
- **THEN** the command succeeds
- **AND** the registered service provider has no attribute mapping
  attached

#### Scenario: Mapping file path cannot be read

- **WHEN** an operator runs `sp add` with `--attribute-mapping-file
  missing.json`, and `missing.json` does not exist or is not readable
- **THEN** the command fails
- **AND** the error message identifies the file path
- **AND** no service provider is registered

#### Scenario: Mapping file contents are not valid JSON

- **WHEN** an operator runs `sp add` with `--attribute-mapping-file
  bad.json`, and `bad.json` cannot be parsed as an attribute mapping
  document
- **THEN** the command fails
- **AND** the error message identifies the file path
- **AND** no service provider is registered

#### Scenario: Mapping file contents fail validation

- **WHEN** an operator runs `sp add` with `--attribute-mapping-file
  invalid.json`, and the document parses successfully but the resulting
  mapping fails the documented mapping validation rules (for example, a
  SAML attribute entry with an empty name)
- **THEN** the command fails
- **AND** no service provider is registered

#### Scenario: The removed `--nameid-format` flag is rejected

- **WHEN** an operator runs `sp add` with `--nameid-format persistent`
- **THEN** the command fails
- **AND** no service provider is registered

#### Scenario: Help text lists a single mapping input

- **WHEN** an operator runs `sp add --help`
- **THEN** the output lists `--attribute-mapping-file` as the way to
  supply mapping settings
- **AND** the output does not list `--nameid-format`
