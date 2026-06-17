# Per-SP Attribute Mapping

## Purpose

Enable per-service-provider SAML attribute mapping configuration so
that each SP can receive a tailored assertion with custom attribute
names, formats, and claim-to-field transformations.

## Requirements

### Requirement: AttributeMapping configuration shape

The system SHALL persist per-SP attribute mapping configuration as a
single nullable JSONB document on the service provider record. The
configuration document SHALL expose four top-level fields:

- `nameid_format` (string, optional) — the requested SAML NameID format.
- `saml_attribute_mappings` (object, optional) — keyed by internal field
  name, valued by `SAMLAttributeDef`.
- `oidc_claim_mappings` (object, optional) — keyed by OIDC claim name,
  valued by internal field name.
- `options` (object, optional) — transform flags such as
  `lowercase_email`.

A null configuration SHALL indicate that no per-SP mapping is configured
and the service provider SHALL receive the bridge's default assertion.

#### Scenario: SP registered without mapping
- **WHEN** a service provider is registered with no
  `attribute_mapping` value
- **THEN** the persisted record SHALL store `NULL` in the
  `attribute_mapping` column
- **AND** `ApplyMapping` SHALL return the input session unchanged

#### Scenario: SP registered with a full mapping
- **WHEN** a service provider is registered with a non-null mapping
  containing all four top-level fields
- **THEN** the JSONB document SHALL round-trip through the database
  without field loss
- **AND** `ApplyMapping` SHALL produce a session reflecting that
  configuration

### Requirement: SAMLAttributeDef per-attribute metadata

Each entry in `saml_attribute_mappings` SHALL be a `SAMLAttributeDef`
object exposing:

- `name` (string, required) — the SAML attribute `Name`.
- `friendly_name` (string, optional) — the SAML attribute
  `FriendlyName`, emitted only when non-empty.
- `name_format` (string, optional) — the SAML attribute `NameFormat`
  URI. When omitted or empty, the effective `NameFormat` SHALL default
  to `urn:oasis:names:tc:SAML:2.0:attrname-format:uri`.

The default `NameFormat` SHALL be exposed as a single named constant in
the domain package so callers and tests reference one source of truth.

#### Scenario: Explicit name format wins over default
- **WHEN** a `SAMLAttributeDef` declares
  `name_format: "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"`
- **THEN** the emitted SAML attribute SHALL carry that exact NameFormat
  value

#### Scenario: Missing name format defaults to URI
- **WHEN** a `SAMLAttributeDef` omits `name_format`
- **THEN** the emitted SAML attribute SHALL carry
  `urn:oasis:names:tc:SAML:2.0:attrname-format:uri`

#### Scenario: Optional friendly name omitted when empty
- **WHEN** a `SAMLAttributeDef` has an empty `friendly_name`
- **THEN** the emitted SAML attribute SHALL omit the `FriendlyName`
  XML attribute rather than emit an empty string

### Requirement: Structural validation of mapping configuration

`AttributeMapping.Validate` SHALL reject configurations that cannot
produce a usable assertion. A nil receiver SHALL be valid (no mapping
configured). The validator SHALL enforce:

- `nameid_format`, when non-empty, MUST be one of `persistent`,
  `transient`, `emailAddress`, `email`, `unspecified`, or a string
  starting with `urn:`.
- Every `SAMLAttributeDef.Name` in `saml_attribute_mappings` MUST be
  non-empty.

Validation SHALL be invoked at service-provider registration time;
invalid configurations SHALL be rejected with an actionable error
message identifying the offending field path.

Semantic cross-map validation (every `saml_attribute_mappings` key must
resolve to a well-known field or an `oidc_claim_mappings` target) is
out of scope for this capability version and is delivered by a later
change.

#### Scenario: Reject empty SAML attribute name
- **WHEN** an admin submits a mapping containing
  `saml_attribute_mappings.email = {name: ""}`
- **THEN** `Validate` SHALL return an error whose `Field` identifies
  `saml_attribute_mappings.email.name`

#### Scenario: Reject unrecognised NameID format
- **WHEN** an admin submits a mapping with
  `nameid_format: "foobar"`
- **THEN** `Validate` SHALL return an error whose `Field` identifies
  `nameid_format`

#### Scenario: Accept full URN NameID format
- **WHEN** an admin submits a mapping with
  `nameid_format: "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"`
- **THEN** `Validate` SHALL return no error

#### Scenario: Nil mapping is valid
- **WHEN** `Validate` is invoked on a nil `*AttributeMapping`
- **THEN** it SHALL return no error

### Requirement: Structured internal user model

The bridge SHALL represent the user, between OIDC claim extraction and
SAML attribute emission, as a `UserAttributes` value with typed fields:

- `Subject` (string)
- `Email` (string)
- `Name` (string)
- `Groups` (`[]string`) — multi-valued, native slice, NOT encoded as a
  single delimited string
- `Custom` (`map[string]string`) — extensible bucket for fields
  populated through custom `oidc_claim_mappings` targets

`UserAttributes` SHALL expose a `GetField(name string) string` helper
that returns the value of a well-known field by name, falling back to
the `Custom` map for unknown names. The helper SHALL return an empty
string when no value is present.

#### Scenario: Get well-known field
- **WHEN** `GetField("email")` is called on a `UserAttributes` with
  `Email = "alice@example.com"`
- **THEN** it SHALL return `"alice@example.com"`

#### Scenario: Get custom field via fallback
- **WHEN** `GetField("dept")` is called on a `UserAttributes` whose
  `Custom` map contains `"dept": "eng"`
- **THEN** it SHALL return `"eng"`

#### Scenario: Get missing field
- **WHEN** `GetField("name")` is called on a `UserAttributes` with no
  `Name` value and no matching `Custom` entry
- **THEN** it SHALL return the empty string

### Requirement: Build user attributes from OIDC claims

The mapping service SHALL construct a `UserAttributes` value from the
session's raw OIDC claims and the SP's `oidc_claim_mappings`
configuration. When `oidc_claim_mappings` is empty, the default mapping
`{"sub": "subject", "email": "email", "name": "name",
"groups": "groups"}` SHALL apply.

For each `(oidc_claim, internal_field)` pair:

- Single-valued claims SHALL populate the matching well-known field
  (`Subject`, `Email`, `Name`) or the `Custom` map when the internal
  field is not well-known.
- The `groups` internal field SHALL be populated as `[]string` from a
  multi-valued OIDC claim (JSON array of strings).
- Claims missing from the raw OIDC token SHALL fall back to the
  equivalent session field when one exists; otherwise the corresponding
  `UserAttributes` field SHALL be left at its zero value.

#### Scenario: Default mapping extracts well-known claims
- **WHEN** `BuildUserAttributes` runs against a session whose
  `RawOIDCClaims` contains `sub`, `email`, and `name`, with empty
  `oidc_claim_mappings`
- **THEN** the returned `UserAttributes` SHALL have `Subject`, `Email`,
  and `Name` populated from those claims

#### Scenario: Custom claim populates Custom map
- **WHEN** `oidc_claim_mappings` contains `"department": "dept"` and
  `RawOIDCClaims` contains `"department": "eng"`
- **THEN** the returned `UserAttributes.Custom["dept"]` SHALL equal
  `"eng"`

#### Scenario: Groups extracted as native slice
- **WHEN** `RawOIDCClaims["groups"]` is the JSON array
  `["admin", "users"]`
- **THEN** the returned `UserAttributes.Groups` SHALL equal
  `["admin", "users"]` with no null-byte encoding

#### Scenario: Missing claim leaves field empty
- **WHEN** `RawOIDCClaims` contains no `email` claim and the session
  has no `UserEmail`
- **THEN** the returned `UserAttributes.Email` SHALL be the empty
  string

### Requirement: Emit SAML attributes from the mapping

When `saml_attribute_mappings` is non-empty, the mapping service SHALL
emit one `<saml:Attribute>` per entry, populated from the corresponding
`UserAttributes` field. Each emitted attribute SHALL set `Name`,
`FriendlyName` (when non-empty), and `NameFormat`
(via `SAMLAttributeDef.EffectiveNameFormat`).

- Single-valued fields SHALL produce one `<saml:AttributeValue>`.
- The `groups` internal field SHALL produce one `<saml:Attribute>`
  containing one `<saml:AttributeValue>` per element of
  `UserAttributes.Groups`.
- A field whose source value is empty (or, for `groups`, whose slice is
  empty) SHALL be omitted from the assertion, with a DEBUG-level log
  recording the omission.

#### Scenario: Single-valued attribute emission
- **WHEN** `saml_attribute_mappings.email` is `{name: "mail",
  friendly_name: "mail"}` and `UserAttributes.Email` is
  `"alice@example.com"`
- **THEN** the assertion SHALL contain one `<saml:Attribute Name="mail"
  FriendlyName="mail" NameFormat="urn:oasis:names:tc:SAML:2.0:
  attrname-format:uri">` with one `<saml:AttributeValue>` of
  `"alice@example.com"`

#### Scenario: Multi-valued groups emission
- **WHEN** `saml_attribute_mappings.groups` is `{name: "memberOf"}`
  and `UserAttributes.Groups` is `["admin", "users"]`
- **THEN** the assertion SHALL contain exactly one
  `<saml:Attribute Name="memberOf">` element with two
  `<saml:AttributeValue>` children, one per group

#### Scenario: Empty source value omits attribute
- **WHEN** `saml_attribute_mappings.name` is `{name: "cn"}` and
  `UserAttributes.Name` is empty
- **THEN** the assertion SHALL NOT contain a `<saml:Attribute
  Name="cn">` element
- **AND** the system SHALL emit a DEBUG log identifying the omitted
  attribute and entity ID

### Requirement: Apply transform options

When `options.lowercase_email` is `true`, the mapping service SHALL
lowercase `UserAttributes.Email` before SAML attributes are built. The
transformation SHALL apply only to the `Email` field and SHALL NOT
mutate the session's stored claims.

#### Scenario: Lowercase email transform
- **WHEN** `options.lowercase_email` is `true` and `UserAttributes.Email`
  is `"Alice@Example.com"`
- **THEN** any emitted SAML attribute sourced from `Email` SHALL carry
  the value `"alice@example.com"`

### Requirement: Suppress default attributes when mapping is active

When `saml_attribute_mappings` is non-empty, the mapping service SHALL
prevent the underlying SAML library from emitting its default
attributes for that session, so the resulting assertion contains
exactly the attributes declared in the configuration. The mechanism for
suppression is an implementation detail of this capability version
(currently field-clearing on the session) and is replaced by a
dedicated custom assertion maker in a later change.

When `saml_attribute_mappings` is empty (including the case where only
`nameid_format` is configured), the service SHALL leave default
attribute emission undisturbed.

#### Scenario: Mapped SP receives only configured attributes
- **WHEN** `saml_attribute_mappings` contains only `{email: {name:
  "mail"}}` and `UserAttributes` has both `Email` and `Name` populated
- **THEN** the assertion SHALL contain one `<saml:Attribute Name="mail">`
- **AND** it SHALL NOT contain any default attribute sourced from
  `Name` (for example `cn` or `displayName`)

#### Scenario: SP with only NameID format receives defaults
- **WHEN** the mapping contains `nameid_format: "transient"` and an
  empty `saml_attribute_mappings`
- **THEN** the assertion SHALL contain the bridge's standard default
  attributes
- **AND** the NameID SHALL use the configured format

### Requirement: Persistence is pre-production breaking

The system MUST persist attribute mapping configuration using the
Phase 2 JSONB layout exclusively and MUST NOT accept the Phase 1
layout (`saml_attributes: map[string]string`, `oidc_claims:
map[string]string`). Operators SHALL re-register any service providers
persisted under the Phase 1 layout.

#### Scenario: Phase 1 row fails to load
- **WHEN** the database contains an `attribute_mapping` row written
  under the Phase 1 layout
- **THEN** loading that service provider SHALL return an error and the
  service provider SHALL NOT be served from cache as if valid
