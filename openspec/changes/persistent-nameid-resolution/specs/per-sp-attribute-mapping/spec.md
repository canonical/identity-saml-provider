## Purpose

These deltas extend the `per-sp-attribute-mapping` capability with a
contract for the persistent NameID format, plus explicit scenarios
for the previously-implicit `transient` and `emailAddress` formats.
They turn `nameid_format: persistent` from "best-effort, derived from
the user's claims" into a true SAML 2.0 persistent identifier:
opaque, pairwise per service provider, durable across sessions,
and fail-closed when the bridge cannot resolve it.

## ADDED Requirements

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

## MODIFIED Requirements

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

Validation SHALL NOT inspect or require the presence of any
persistent NameID storage row; runtime-resolution preconditions
(such as the OIDC `sub` claim being present in the session) are
checked at assertion time, not at registration time, because the
session's claims are not available when the configuration is
written.

Semantic cross-map validation (every `saml_attribute_mappings` key
must resolve to a well-known field or an `oidc_claim_mappings`
target) is out of scope for this capability version and is
delivered by a later change.

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
