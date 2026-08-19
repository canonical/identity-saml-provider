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
- Every key in `saml_attribute_mappings` MUST be resolvable. A key
  is resolvable when it is one of the well-known internal field
  names (`subject`, `email`, `name`, `groups`) OR appears as a
  value in `oidc_claim_mappings`. Unresolvable keys identify SAML
  attributes that would never be populated and are rejected at
  configuration time rather than silently omitted at every
  assertion.

Validation SHALL be invoked at service-provider registration time
and at every attribute-mapping update; invalid configurations SHALL
be rejected with an actionable error message identifying the
offending field path.

Validation SHALL NOT inspect or require the presence of any
persistent NameID storage row; runtime-resolution preconditions
(such as the OIDC `sub` claim being present in the session) are
checked at assertion time, not at registration time, because the
session's claims are not available when the configuration is
written.

Validation SHALL NOT verify that any referenced OIDC claim is
actually present in the upstream ID token. Whether the upstream
provider issues `email`, `groups`, or any custom claim is a
per-user, per-session concern handled at assertion time by omitting
the attribute (with a DEBUG-level log). The validator is
structural, not a spell-checker: a typo that is consistently
mirrored across `oidc_claim_mappings` and `saml_attribute_mappings`
satisfies the structural rule and is accepted.

When more than one `saml_attribute_mappings` key fails the
resolvability check, the validator SHALL report the lexicographically
first failing key so the rejection is deterministic across processes
and test runs.

Stored attribute-mapping rows persisted before this requirement
took effect SHALL NOT be re-validated automatically. The new rule
fires on the next register or update write that touches the
configuration.

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

#### Scenario: Persistent format is accepted at registration regardless of session

- **WHEN** an admin submits a mapping with
  `nameid_format: "persistent"` and no other fields
- **THEN** `Validate` SHALL return no error, even though no session
  is available to verify that an upstream OIDC `sub` claim will be
  present at authentication time

#### Scenario: Well-known SAML key is always resolvable

- **WHEN** an admin submits a mapping with
  `oidc_claim_mappings: {}` and
  `saml_attribute_mappings: {"email": {"name": "mail"}}`
- **THEN** `Validate` SHALL return no error, because `email` is a
  well-known internal field

#### Scenario: Custom SAML key with matching OIDC target is resolvable

- **WHEN** an admin submits a mapping with
  `oidc_claim_mappings: {"department": "dept"}` and
  `saml_attribute_mappings: {"dept": {"name": "urn:oid:2.5.4.11", "friendly_name": "ou"}}`
- **THEN** `Validate` SHALL return no error, because `dept` appears
  as a value in `oidc_claim_mappings`

#### Scenario: Reject SAML key that has no resolvable source

- **WHEN** an admin submits a mapping with
  `oidc_claim_mappings: {"sub": "subject"}` and
  `saml_attribute_mappings: {"dept": {"name": "department"}}`
- **THEN** `Validate` SHALL return an error whose `Field` identifies
  `saml_attribute_mappings.dept`

#### Scenario: Mirrored typo across both maps is accepted

- **WHEN** an admin submits a mapping with
  `oidc_claim_mappings: {"email": "emal"}` and
  `saml_attribute_mappings: {"emal": {"name": "mail"}}`
- **THEN** `Validate` SHALL return no error, because the rule is
  structural — `emal` is consistently defined as the bridge field
  between the two maps

#### Scenario: Typo isolated to OIDC target with well-known SAML key is accepted

- **WHEN** an admin submits a mapping with
  `oidc_claim_mappings: {"email": "emal"}` and
  `saml_attribute_mappings: {"email": {"name": "mail"}}`
- **THEN** `Validate` SHALL return no error, because `email` is a
  well-known internal field. At assertion time the `email`
  attribute will be omitted (with a DEBUG log) because no claim
  populates it.

#### Scenario: Multiple unresolvable keys report a deterministic failure

- **WHEN** an admin submits a mapping with
  `oidc_claim_mappings: {"sub": "subject"}` and
  `saml_attribute_mappings: {"zeta": {"name": "z"}, "alpha": {"name": "a"}}`
- **THEN** `Validate` SHALL return an error whose `Field` identifies
  `saml_attribute_mappings.alpha`, regardless of Go's randomized
  map iteration order

#### Scenario: Empty saml_attribute_mappings skips the cross-map check

- **WHEN** an admin submits a mapping with
  `nameid_format: "persistent"`, `oidc_claim_mappings: {}`, and
  `saml_attribute_mappings: {}`
- **THEN** `Validate` SHALL return no error

### Requirement: Persistent NameID resolution

The mapping service SHALL emit an opaque, pairwise, stable NameID
for every `(service-provider, upstream-user)` pair whose mapping
configures `nameid_format: persistent` (or the equivalent
`urn:oasis:names:tc:SAML:2.0:nameid-format:persistent` URN).

The NameID value SHALL satisfy all of the following:

- **Opaque**: the value SHALL be a randomly generated UUID
  (RFC 4122 v4) and SHALL NOT be derived from, equal to, or contain
  any user-attribute value (`Subject`, `Email`, `Name`, raw OIDC
  `sub`, or any custom claim).
- **Pairwise**: two distinct service providers authenticating the
  same upstream user SHALL receive distinct NameID values.
- **Stable**: every authentication for the same
  `(service-provider, upstream-user)` pair SHALL receive the same
  NameID value, including across bridge restarts and database
  failovers.
- **Durable**: the NameID SHALL be persisted before being emitted
  in an assertion, so that a subsequent authentication can recover
  the same value.

The pair SHALL be keyed by `(service-provider EntityID,
RawOIDCClaims["sub"])`. The bridge SHALL NOT use the mapped
`UserAttributes.Subject` for this lookup, because operators can
remap `subject` via `oidc_claim_mappings`; doing so would break
stability and orphan previously-issued NameIDs.

The NameID format URI emitted in the SAML assertion SHALL be
`urn:oasis:names:tc:SAML:2.0:nameid-format:persistent`.

#### Scenario: Same SP and same user receive the same NameID

- **WHEN** an upstream user authenticates twice in succession to a
  service provider configured with `nameid_format: persistent`,
  with the same OIDC `sub` claim on both authentications
- **THEN** both assertions SHALL carry the same `<saml:NameID>`
  value
- **AND** the value SHALL parse as an RFC 4122 UUID

#### Scenario: Different SPs for the same user receive different NameIDs

- **WHEN** the same upstream user authenticates to two distinct
  service providers, both configured with `nameid_format:
  persistent`
- **THEN** the two assertions SHALL carry distinct `<saml:NameID>`
  values

#### Scenario: NameID survives bridge restart

- **WHEN** a user has authenticated once to a persistent-format SP,
  and the bridge process is then restarted, and the user
  authenticates again
- **THEN** the second assertion SHALL carry the same
  `<saml:NameID>` value as the first

#### Scenario: NameID is independent of mapped subject

- **WHEN** a service provider's `oidc_claim_mappings` is changed
  from `{"sub": "subject"}` to `{"email": "subject"}` between two
  authentications for the same upstream user
- **THEN** the persistent NameID emitted on the second
  authentication SHALL equal the one emitted on the first, because
  the lookup key is the OIDC `sub` claim and not the mapped
  `Subject`

#### Scenario: NameID does not contain user attribute values

- **WHEN** a persistent NameID is generated for a user whose OIDC
  `sub`, `email`, and `name` claims are known
- **THEN** the emitted `<saml:NameID>` value SHALL NOT contain any
  of those claim values as a substring

### Requirement: Persistent NameID fails closed on missing inputs or storage errors

The mapping service SHALL refuse to issue a persistent NameID — and
therefore refuse to build the SAML assertion — when it cannot
satisfy the opaque/pairwise/stable contract. Specifically,
`ApplyMapping` SHALL return a typed domain error (and emit no
SAML response) in each of these cases:

- The session's `RawOIDCClaims["sub"]` is absent, empty, or not a
  string.
- The persistent-NameID storage backend returns any error from the
  get-or-create operation.

The service SHALL NOT fall back to `UserAttributes.Email`,
`UserAttributes.Subject`, the raw OIDC `sub`, a freshly generated
UUID, or any other surrogate. Falling back would either expose a
non-opaque value or introduce a NameID that changes across requests,
both of which break the SAML 2.0 contract for persistent
identifiers.

#### Scenario: Missing OIDC sub aborts the assertion

- **WHEN** an SP configured with `nameid_format: persistent`
  presents a session whose `RawOIDCClaims` contains no `sub` claim
- **THEN** `ApplyMapping` SHALL return a typed domain error
  identifying the SP entity ID and the missing canonical subject
- **AND** the bridge SHALL NOT emit a SAML response for that
  request

#### Scenario: Storage backend error aborts the assertion

- **WHEN** the persistent-NameID storage backend returns any error
  from the get-or-create call
- **THEN** `ApplyMapping` SHALL return a typed domain error wrapping
  that storage error
- **AND** the bridge SHALL NOT emit a SAML response for that
  request
- **AND** subsequent authentications SHALL re-attempt resolution
  rather than caching a fallback value

### Requirement: Persistent NameID storage durability

The system SHALL persist each generated persistent NameID before it
is returned to the caller, so that the same NameID is recoverable on
subsequent authentications.

The storage SHALL guarantee at-most-one persistent NameID per
`(service-provider EntityID, upstream user OIDC sub)` pair, even
under concurrent first-time-authentication requests for the same
pair. The first writer's value SHALL be returned to all concurrent
callers; no caller SHALL receive a value that is not also persisted
for future reads.

When a service provider record is removed from the bridge, all
persistent NameID rows associated with that service provider SHALL
be removed automatically.

The system SHALL NOT remove persistent NameID rows for upstream
users that disappear from the OIDC provider; user-lifecycle cleanup
is explicitly out of scope for this capability version.

#### Scenario: First and second resolution return the same persisted value

- **WHEN** `GetOrCreate` is invoked for a `(SP, user)` pair that has
  no existing row, and is then invoked again for the same pair
- **THEN** the first call SHALL return a freshly generated UUID and
  persist it
- **AND** the second call SHALL return the exact same UUID without
  generating a new one

#### Scenario: Concurrent first-time resolution converges on one value

- **WHEN** two concurrent requests invoke `GetOrCreate` for the same
  `(SP, user)` pair that has no existing row
- **THEN** both calls SHALL return the same UUID value
- **AND** exactly one row SHALL exist in the persistent NameID
  storage for that pair

#### Scenario: Service provider removal cascades to NameIDs

- **WHEN** an operator removes a service provider record from the
  bridge while persistent NameID rows exist for that service
  provider
- **THEN** every persistent NameID row whose `entity_id` matches
  the removed service provider SHALL be removed in the same
  operation

#### Scenario: Upstream user removal does not cascade

- **WHEN** an upstream user is deleted from the OIDC provider
- **THEN** the bridge SHALL NOT remove any persistent NameID rows
  associated with that user
- **AND** the bridge SHALL surface no error from this condition; the
  rows SHALL remain inert until cleanup is addressed by a later
  capability version

### Requirement: Transient NameID resolution

The mapping service SHALL emit a freshly generated RFC 4122 v4 UUID
as the NameID value on every authentication when a service
provider's mapping configures `nameid_format: transient` (or the
equivalent SAML URN). The value SHALL NOT be persisted, and the
format URI in the assertion SHALL be
`urn:oasis:names:tc:SAML:2.0:nameid-format:transient`.

#### Scenario: Transient NameID changes per authentication

- **WHEN** the same upstream user authenticates twice to a
  service provider configured with `nameid_format: transient`
- **THEN** the two assertions SHALL carry distinct `<saml:NameID>`
  values
- **AND** each value SHALL parse as an RFC 4122 UUID

### Requirement: Email-address NameID resolution

The mapping service SHALL emit `UserAttributes.Email` as the NameID
value when a service provider's mapping configures
`nameid_format: emailAddress` (or `email`, or the equivalent SAML
URN), after any `Options.LowercaseEmail` transform has been
applied. The format URI in the assertion SHALL be
`urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress`.

When `UserAttributes.Email` is empty for an SP configured with this
format, the mapping service SHALL return a typed domain error and
the bridge SHALL NOT emit a SAML response — silently emitting an
empty NameID would violate the SAML 2.0 NameID schema.

#### Scenario: Email NameID uses lowercased address

- **WHEN** an SP with `nameid_format: emailAddress` and
  `options.lowercase_email: true` authenticates a user whose
  `UserAttributes.Email` was extracted as `"Alice@Example.com"`
- **THEN** the assertion SHALL carry
  `<saml:NameID Format="urn:oasis:names:tc:SAML:1.1:nameid-format:
  emailAddress">alice@example.com</saml:NameID>`

#### Scenario: Missing email aborts the assertion

- **WHEN** an SP with `nameid_format: emailAddress` authenticates a
  user whose `UserAttributes.Email` is empty
- **THEN** `ApplyMapping` SHALL return a typed domain error
  identifying the SP entity ID
- **AND** the bridge SHALL NOT emit a SAML response

### Requirement: Persistent NameID observability

For every persistent-NameID resolution, the bridge SHALL emit a
structured log record at INFO level identifying the service provider
entity ID and the canonical OIDC subject used as the lookup key.
For every storage call, the bridge SHALL open a tracing span named
`repo.persistent_nameid.get_or_create` whose attributes include the
service provider entity ID.

Failure paths (missing canonical subject, storage error) SHALL be
logged at ERROR level with the same key fields plus the underlying
error.

These signals exist so that operators can audit persistent NameID
issuance, correlate NameIDs back to the originating authentication,
and diagnose stability regressions without inspecting database
contents.

#### Scenario: Successful resolution emits INFO log and span

- **WHEN** a persistent NameID is successfully resolved for an
  authentication
- **THEN** the bridge SHALL emit one INFO log record with at least
  the keys `entityID` and `canonicalSubject`
- **AND** a `repo.persistent_nameid.get_or_create` span SHALL be
  recorded for the storage call with at least the
  `entityID` attribute

#### Scenario: Failure emits ERROR log

- **WHEN** persistent NameID resolution fails because the storage
  backend returned an error
- **THEN** the bridge SHALL emit one ERROR log record carrying the
  service provider entity ID, canonical subject, and the wrapped
  underlying error

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

### Requirement: Admin endpoint to retrieve a service provider's configuration

The bridge SHALL expose `GET /admin/service-providers` that returns
the persisted configuration of a single service provider, addressed
by an `entity_id` query parameter.

The endpoint SHALL accept the EntityID as a query parameter rather
than a path parameter, because SAML EntityIDs are typically URLs
containing `/`, `:`, and `?` that are unsafe to embed in a URL path.
Standard percent-encoding applies to the query value.

A successful response SHALL be `200 OK` with a JSON body containing,
at minimum, the SP's `entity_id`, `acs_url`, `acs_binding`, and the
current `attribute_mapping`. When the SP has no attribute mapping
configured (the JSONB column is `NULL`), the response SHALL omit the
`attribute_mapping` field rather than emit `null`, so the wire shape
clearly conveys "no mapping configured".

When the `attribute_mapping` is present, its JSON shape SHALL match
exactly the shape accepted by `POST /admin/service-providers` and by
`PUT /admin/service-providers/attribute-mapping`, so callers can
round-trip the document without re-shaping it.

The endpoint SHALL NOT require authentication in this capability
version; it inherits the same posture as the existing
`POST /admin/service-providers` endpoint. A subsequent capability
will introduce authentication for the entire `/admin/` surface.

#### Scenario: GET returns full configuration for a mapped SP

- **WHEN** an operator issues `GET /admin/service-providers?entity_id=<id>`
  for a service provider that was registered with a non-null
  `attribute_mapping`
- **THEN** the response status SHALL be `200 OK`
- **AND** the response body SHALL contain `entity_id`, `acs_url`,
  `acs_binding`, and `attribute_mapping` whose JSON shape matches
  what was persisted

#### Scenario: GET omits attribute_mapping when none is configured

- **WHEN** an operator issues `GET /admin/service-providers?entity_id=<id>`
  for a service provider registered without an attribute mapping
- **THEN** the response status SHALL be `200 OK`
- **AND** the response body SHALL contain `entity_id`, `acs_url`,
  and `acs_binding`
- **AND** the response body SHALL NOT contain an `attribute_mapping`
  field (the field SHALL be omitted, not emitted as `null`)

#### Scenario: GET returns 400 when entity_id is missing

- **WHEN** an operator issues `GET /admin/service-providers` with no
  `entity_id` query parameter, or with an empty value
- **THEN** the response status SHALL be `400 Bad Request`
- **AND** the response body SHALL identify the missing
  `entity_id` query parameter

#### Scenario: GET returns 404 for an unknown service provider

- **WHEN** an operator issues `GET /admin/service-providers?entity_id=<id>`
  for an `entity_id` that has never been registered
- **THEN** the response status SHALL be `404 Not Found`

### Requirement: Admin endpoint to replace a service provider's attribute mapping

The bridge SHALL expose `PUT /admin/service-providers/attribute-mapping`,
addressed by an `entity_id` query parameter, that fully replaces the
addressed SP's `attribute_mapping` document with the JSON body
provided in the request.

`PUT` SHALL behave as a full replacement of the `attribute_mapping`
field only. It SHALL NOT change `entity_id`, `acs_url`, `acs_binding`,
or any other field on the SP; those fields are immutable through
this endpoint.

The request body SHALL be a JSON object conforming to the same
`AttributeMapping` shape accepted by `POST /admin/service-providers`.
The same validation rules SHALL be applied to the body before any
write occurs; an invalid mapping SHALL be rejected with `400 Bad
Request` and an actionable error message identifying the offending
field path, and the persisted mapping SHALL be left unchanged.

A successful update SHALL persist the new mapping atomically and
return `200 OK` with a brief success envelope identifying that the
attribute mapping was updated.

A subsequent `GET /admin/service-providers?entity_id=<id>` SHALL
return an `attribute_mapping` that is byte-equivalent — after
canonical JSON key ordering — to the body of the preceding successful
`PUT`. This round-trip property is the operator's contract for "what
I sent is what is stored".

The endpoint SHALL distinguish between an absent mapping and a
present-but-empty mapping (see the dedicated requirement below). A
`PUT` with body `{}` SHALL be accepted as a configured-empty mapping
and SHALL NOT be silently coerced into a clear operation.

#### Scenario: PUT replaces an existing mapping and round-trips through GET

- **WHEN** an operator issues `PUT /admin/service-providers/attribute-mapping?entity_id=<id>`
  with a valid non-empty `AttributeMapping` body on a service
  provider that already has a mapping
- **THEN** the response status SHALL be `200 OK`
- **AND** a subsequent `GET /admin/service-providers?entity_id=<id>`
  SHALL return an `attribute_mapping` whose JSON content matches the
  PUT body after canonical key ordering

#### Scenario: PUT installs a mapping on a previously unmapped SP

- **WHEN** an operator issues `PUT /admin/service-providers/attribute-mapping?entity_id=<id>`
  with a valid `AttributeMapping` body on a service provider that
  has no mapping configured
- **THEN** the response status SHALL be `200 OK`
- **AND** the SP's `attribute_mapping` column SHALL no longer be
  `NULL`
- **AND** the subsequent `GET` response SHALL contain the new
  `attribute_mapping`

#### Scenario: PUT rejects an invalid mapping with 400 and preserves prior state

- **WHEN** an operator issues `PUT /admin/service-providers/attribute-mapping?entity_id=<id>`
  with a body that fails `AttributeMapping` validation (for example,
  `nameid_format: "foobar"`, or a `SAMLAttributeDef` with empty
  `name`)
- **THEN** the response status SHALL be `400 Bad Request`
- **AND** the response body SHALL identify the offending field path
- **AND** the SP's persisted `attribute_mapping` SHALL be unchanged
  from before the request

#### Scenario: PUT rejects an invalid JSON body with 400

- **WHEN** an operator issues `PUT /admin/service-providers/attribute-mapping?entity_id=<id>`
  with a body that is not valid JSON
- **THEN** the response status SHALL be `400 Bad Request`
- **AND** the SP's persisted `attribute_mapping` SHALL be unchanged

#### Scenario: PUT returns 400 when entity_id is missing

- **WHEN** an operator issues `PUT /admin/service-providers/attribute-mapping`
  with no `entity_id` query parameter, or an empty value
- **THEN** the response status SHALL be `400 Bad Request`
- **AND** no SP state SHALL be modified

#### Scenario: PUT returns 404 for an unknown service provider

- **WHEN** an operator issues `PUT /admin/service-providers/attribute-mapping?entity_id=<id>`
  with a valid body for an `entity_id` that has never been registered
- **THEN** the response status SHALL be `404 Not Found`
- **AND** no SP SHALL be created as a side effect

### Requirement: Admin endpoint to clear a service provider's attribute mapping

The bridge SHALL expose `DELETE /admin/service-providers/attribute-mapping`,
addressed by an `entity_id` query parameter, that clears the
addressed SP's attribute mapping and reverts the SP to default
(unmapped) assertion behaviour.

A successful clear SHALL set the SP's `attribute_mapping` to "no
mapping configured" (the JSONB column SHALL become `NULL`) and SHALL
NOT modify any other field on the SP. After a successful `DELETE`,
the SP SHALL produce assertions indistinguishable from an SP that
was registered without an `attribute_mapping`.

A successful clear SHALL return `200 OK` with a brief success
envelope identifying that the attribute mapping was cleared.

Clearing a mapping SHALL NOT remove the SP, SHALL NOT affect the SP's
persistent NameID rows (those continue to be owned by the SP record
and are cascade-deleted only when the SP itself is deleted), and
SHALL NOT touch any other configuration field.

#### Scenario: DELETE clears an existing mapping

- **WHEN** an operator issues `DELETE /admin/service-providers/attribute-mapping?entity_id=<id>`
  on a service provider that has an attribute mapping configured
- **THEN** the response status SHALL be `200 OK`
- **AND** a subsequent `GET /admin/service-providers?entity_id=<id>`
  SHALL NOT include an `attribute_mapping` field in its response
- **AND** an authentication for that SP after the DELETE SHALL
  produce the same assertion that would be produced for an SP
  registered without an `attribute_mapping`

#### Scenario: DELETE is idempotent on an unmapped SP

- **WHEN** an operator issues `DELETE /admin/service-providers/attribute-mapping?entity_id=<id>`
  on a service provider that has no attribute mapping configured
- **THEN** the response status SHALL be `200 OK`
- **AND** the SP's configuration SHALL be unchanged

#### Scenario: DELETE preserves persistent NameID rows

- **WHEN** an operator issues `DELETE /admin/service-providers/attribute-mapping?entity_id=<id>`
  on a service provider that has existing persistent NameID rows
- **THEN** every persistent NameID row associated with that SP
  SHALL still exist after the DELETE completes

#### Scenario: DELETE returns 400 when entity_id is missing

- **WHEN** an operator issues `DELETE /admin/service-providers/attribute-mapping`
  with no `entity_id` query parameter, or an empty value
- **THEN** the response status SHALL be `400 Bad Request`
- **AND** no SP state SHALL be modified

#### Scenario: DELETE returns 404 for an unknown service provider

- **WHEN** an operator issues `DELETE /admin/service-providers/attribute-mapping?entity_id=<id>`
  for an `entity_id` that has never been registered
- **THEN** the response status SHALL be `404 Not Found`

### Requirement: PUT with an empty object is distinct from DELETE

The admin attribute-mapping endpoints SHALL treat `PUT
/admin/service-providers/attribute-mapping?entity_id=<id>` with body
`{}` as a **configured-empty** mapping (a non-null `AttributeMapping`
with all fields at their zero values), not as a clear operation.

After such a `PUT`, the SP's persisted state SHALL be a non-null
`AttributeMapping` document and a subsequent `GET` SHALL include an
`attribute_mapping` field (whose JSON value is `{}`). After a
`DELETE`, the SP's persisted state SHALL be the absence of any
mapping document and a subsequent `GET` SHALL omit the
`attribute_mapping` field entirely.

This distinction is part of the public contract so that:

- Operators have an unambiguous way to express both intents ("I want
  an explicit, configured mapping that happens to be empty" vs "I
  want this SP to behave as if it were never mapped").
- Future fields added to `AttributeMapping` whose zero value is
  meaningful do not silently collapse the two states.

The runtime assertion behaviour of a configured-empty mapping SHALL
remain governed by the existing `per-sp-attribute-mapping`
requirements (notably "Suppress default attributes when mapping is
active" — an empty `saml_attribute_mappings` does not suppress
defaults, and an empty `nameid_format` falls through to the bridge's
default NameID handling). The distinction enforced here is about
**persisted state and wire shape**, not about which attributes appear
in the SAML assertion today.

#### Scenario: PUT empty object persists a configured-empty mapping

- **WHEN** an operator issues `PUT /admin/service-providers/attribute-mapping?entity_id=<id>`
  with body `{}`
- **THEN** the response status SHALL be `200 OK`
- **AND** a subsequent `GET /admin/service-providers?entity_id=<id>`
  SHALL include an `attribute_mapping` field whose JSON value is
  `{}`

#### Scenario: DELETE persists the absence of a mapping

- **WHEN** an operator issues `DELETE /admin/service-providers/attribute-mapping?entity_id=<id>`
- **THEN** a subsequent `GET /admin/service-providers?entity_id=<id>`
  SHALL NOT include an `attribute_mapping` field at all

#### Scenario: PUT {} followed by DELETE yields the unmapped state

- **WHEN** an operator issues `PUT /admin/service-providers/attribute-mapping?entity_id=<id>`
  with body `{}`, then issues `DELETE` for the same SP
- **THEN** after the `DELETE`, the SP SHALL be in the unmapped state
- **AND** a subsequent `GET` SHALL omit the `attribute_mapping`
  field

### Requirement: Admin attribute-mapping endpoints use a consistent error envelope

The bridge SHALL ensure all three admin attribute-mapping endpoints
(`GET`, `PUT`, `DELETE`) return errors using the same JSON envelope
already used by `POST /admin/service-providers`, so admin-API clients
see one consistent error model across the surface.

Error responses SHALL carry the appropriate HTTP status code (`400`
for malformed input or validation failure, `404` for unknown SP,
`500` for unexpected internal failure) and a JSON body containing,
at minimum, the status code and a human-readable message. Validation
errors from `AttributeMapping.Validate` SHALL surface the offending
field path in the message so operators can self-correct.

Success responses SHALL also be JSON. `GET` SHALL return the
configuration body. `PUT` and `DELETE` SHALL return a brief envelope
containing a `status` and a `message` field identifying the
performed operation.

#### Scenario: Validation error includes the offending field path

- **WHEN** any admin attribute-mapping endpoint rejects a request
  because `AttributeMapping.Validate` returned a field-scoped error
- **THEN** the response body SHALL include the field path identified
  by `Validate` (for example, `saml_attribute_mappings.email.name`
  or `nameid_format`)

#### Scenario: Unexpected internal failure surfaces as 500 without leaking internals

- **WHEN** any admin attribute-mapping endpoint encounters an
  unexpected internal error (for example, the database is
  unavailable)
- **THEN** the response status SHALL be `500 Internal Server Error`
- **AND** the response body SHALL be a JSON error envelope
- **AND** the response body SHALL NOT include raw database error
  text, stack traces, or SQL fragments

### Requirement: CLI mapping configuration uses a single input path

The `identity-saml-provider service-provider create` command SHALL accept the service
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

- **WHEN** an operator runs `service-provider create` with `--attribute-mapping-file
  mapping.json`, and `mapping.json` describes the NameID format, the
  SAML attribute settings, and the OIDC claim mappings
- **THEN** the command succeeds
- **AND** the registered service provider's stored attribute mapping
  reflects every setting from `mapping.json`

#### Scenario: Register service provider with a NameID-only mapping file

- **WHEN** an operator runs `service-provider create` with `--attribute-mapping-file
  nameid.json`, and `nameid.json` is the JSON document
  `{"nameid_format": "persistent"}`
- **THEN** the command succeeds
- **AND** the registered service provider's stored attribute mapping
  records the NameID format as `persistent`
- **AND** the stored attribute mapping carries no SAML attribute settings

#### Scenario: Register service provider without any mapping flag

- **WHEN** an operator runs `service-provider create` without `--attribute-mapping-file`
- **THEN** the command succeeds
- **AND** the registered service provider has no attribute mapping
  attached

#### Scenario: Mapping file path cannot be read

- **WHEN** an operator runs `service-provider create` with `--attribute-mapping-file
  missing.json`, and `missing.json` does not exist or is not readable
- **THEN** the command fails
- **AND** the error message identifies the file path
- **AND** no service provider is registered

#### Scenario: Mapping file contents are not valid JSON

- **WHEN** an operator runs `service-provider create` with `--attribute-mapping-file
  bad.json`, and `bad.json` cannot be parsed as an attribute mapping
  document
- **THEN** the command fails
- **AND** the error message identifies the file path
- **AND** no service provider is registered

#### Scenario: Mapping file contents fail validation

- **WHEN** an operator runs `service-provider create` with `--attribute-mapping-file
  invalid.json`, and the document parses successfully but the resulting
  mapping fails the documented mapping validation rules (for example, a
  SAML attribute entry with an empty name)
- **THEN** the command fails
- **AND** no service provider is registered

#### Scenario: The removed `--nameid-format` flag is rejected

- **WHEN** an operator runs `service-provider create` with `--nameid-format persistent`
- **THEN** the command fails
- **AND** no service provider is registered

#### Scenario: Help text lists a single mapping input

- **WHEN** an operator runs `service-provider create --help`
- **THEN** the output lists `--attribute-mapping-file` as the way to
  supply mapping settings
- **AND** the output does not list `--nameid-format`
