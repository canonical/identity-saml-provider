## ADDED Requirements

### Requirement: Persistent NameID type validation

`AttributeMapping.Validate` SHALL enforce that `persistent_type`, when
non-empty:

- MUST be one of `pairwise` or `public`.
- MUST NOT be specified when `nameid_format` is explicitly configured to a
  non-persistent format (such as `transient` or `emailAddress`).

#### Scenario: Reject unrecognised persistent NameID type

- **WHEN** an admin submits a mapping with
  `nameid_format: "persistent"` and `persistent_type: "invalid"`
- **THEN** `Validate` SHALL return an error whose `Field` identifies
  `persistent_type`

#### Scenario: Reject persistent_type when nameid_format is non-persistent

- **WHEN** an admin submits a mapping with
  `nameid_format: "transient"` and `persistent_type: "public"`
- **THEN** `Validate` SHALL return an error whose `Field` identifies
  `persistent_type`

#### Scenario: Accept valid persistent NameID types

- **WHEN** an admin submits a mapping with
  `nameid_format: "persistent"` and `persistent_type: "public"`
  (or `"pairwise"`)
- **THEN** `Validate` SHALL return no error

#### Scenario: Accept persistent_type when nameid_format is empty

- **WHEN** an admin submits a mapping with
  `persistent_type: "public"` and an empty `nameid_format`
- **THEN** `Validate` SHALL return no error, because `nameid_format`
  defaults to `persistent`

## MODIFIED Requirements

### Requirement: AttributeMapping configuration shape

The system SHALL persist per-SP attribute mapping configuration as a
single nullable JSONB document on the service provider record. The
configuration document SHALL expose five top-level fields:

- `nameid_format` (string, optional) — the requested SAML NameID format.
- `persistent_type` (string, optional) — the persistent NameID mode (`public`
  or `pairwise`).
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
  containing all five top-level fields
- **THEN** the JSONB document SHALL round-trip through the database
  without field loss
- **AND** `ApplyMapping` SHALL produce a session reflecting that
  configuration

### Requirement: Persistent NameID resolution

The mapping service SHALL emit a persistent NameID for every
`(service-provider, upstream-user)` pair whose mapping configures
`nameid_format: persistent` (or the equivalent
`urn:oasis:names:tc:SAML:2.0:nameid-format:persistent` URN).

The resolution mode SHALL be determined by the configured top-level
`persistent_type` field:

1. **`public` mode (default / empty)**:
   - **Public/Direct**: the NameID value SHALL equal the canonical OIDC `sub`
     claim value directly.
   - **Cross-SP Uniformity**: multiple distinct service providers configured
     with `public` mode (or omitting `persistent_type`) authenticating the
     same user SHALL receive the exact same NameID value.
   - **No Database Dependency**: resolution in `public` mode SHALL NOT query
     or store records in the persistent NameID repository.

2. **`pairwise` mode**:
   - **Opaque**: the value SHALL be a randomly generated UUID (RFC 4122 v4)
     and SHALL NOT be derived from user-attribute values.
   - **Pairwise**: two distinct service providers configured in `pairwise` mode
     for the same user SHALL receive distinct NameID values.
   - **Stable**: every authentication for the same
     `(service-provider, upstream-user)` pair SHALL receive the same NameID.
   - **Durable**: the NameID SHALL be persisted in database storage before
     being emitted.

In all cases, the lookup/value key SHALL be derived from `RawOIDCClaims["sub"]`
and NOT the mapped `UserAttributes.Subject`. The NameID format URI emitted in
the SAML assertion SHALL be
`urn:oasis:names:tc:SAML:2.0:nameid-format:persistent`.

#### Scenario: Same SP and same user receive the same NameID

- **WHEN** an upstream user authenticates twice in succession to a
  service provider configured with `nameid_format: persistent`,
  with the same OIDC `sub` claim on both authentications
- **THEN** both assertions SHALL carry the same `<saml:NameID>`
  value
- **AND** the value SHALL parse as an RFC 4122 UUID when `persistent_type` is
  `pairwise`

#### Scenario: Different SPs for the same user receive different NameIDs

- **WHEN** the same upstream user authenticates to two distinct
  service providers, both configured with `nameid_format: persistent`
  and `persistent_type: "pairwise"`
- **THEN** the two assertions SHALL carry distinct `<saml:NameID>`
  values

#### Scenario: NameID survives bridge restart

- **WHEN** a user has authenticated once to a persistent-format SP in pairwise
  mode, and the bridge process is then restarted, and the user
  authenticates again
- **THEN** the second assertion SHALL carry the same
  `<saml:NameID>` value as the first

#### Scenario: NameID is independent of mapped subject

- **WHEN** a service provider's `oidc_claim_mappings` is changed
  from `{"sub": "subject"}` to `{"email": "subject"}` between two
  authentications for the same upstream user
- **THEN** the persistent NameID emitted on the second
  authentication SHALL equal the one emitted on the first, because
  the key is the canonical OIDC `sub` claim and not the mapped `Subject`

#### Scenario: NameID does not contain user attribute values

- **WHEN** a pairwise persistent NameID is generated for a user whose OIDC
  `sub`, `email`, and `name` claims are known
- **THEN** the emitted `<saml:NameID>` value SHALL NOT contain any
  of those claim values as a substring

#### Scenario: Shared mode emits OIDC sub directly

- **WHEN** an upstream user with OIDC claim `sub: "user-12345"` authenticates to
  an SP configured with `nameid_format: persistent` and
  `persistent_type: "public"` (or `persistent_type` omitted)
- **THEN** the assertion SHALL carry `"user-12345"` as the `<saml:NameID>`
- **AND** no database query SHALL be executed against the persistent NameID
  repository

#### Scenario: Shared mode returns identical NameIDs across multiple SPs

- **WHEN** the same upstream user with OIDC claim `sub: "user-12345"`
  authenticates to two distinct SPs both configured with
  `nameid_format: persistent` and `persistent_type: "public"` (or omitted)
- **THEN** both assertions SHALL carry `<saml:NameID>` equal to `"user-12345"`

### Requirement: Persistent NameID fails closed on missing inputs or storage errors

The mapping service SHALL refuse to issue a persistent NameID — and
therefore refuse to build the SAML assertion — when it cannot satisfy
the persistent NameID contract. Specifically, `ApplyMapping` SHALL
return a typed domain error (and emit no SAML response) in each of these
cases:

- The session's `RawOIDCClaims["sub"]` is absent, empty, or not a string
  (applies to both `public` mode default and `pairwise` mode).
- In `pairwise` mode, the persistent-NameID storage backend returns any error
  from the get-or-create operation.

In `pairwise` mode, the service SHALL NOT fall back to `UserAttributes.Email`,
`UserAttributes.Subject`, the raw OIDC `sub`, a freshly generated UUID, or
any other surrogate upon storage error. In `public` mode, emitting the raw OIDC
`sub` is the configured behavior rather than an error fallback; however, an
absent or non-string `sub` claim SHALL fail closed and return an error without
emitting an assertion.

#### Scenario: Missing OIDC sub aborts the assertion

- **WHEN** an SP configured with `nameid_format: persistent` (in either `public`
  mode default or `pairwise` mode) presents a session whose `RawOIDCClaims`
  contains no `sub` claim
- **THEN** `ApplyMapping` SHALL return a typed domain error identifying the SP
  entity ID and the missing canonical subject
- **AND** the bridge SHALL NOT emit a SAML response for that request

#### Scenario: Storage backend error aborts the assertion

- **WHEN** an SP is configured in `pairwise` mode and the persistent-NameID
  storage backend returns any error from the get-or-create call
- **THEN** `ApplyMapping` SHALL return a typed domain error wrapping that
  storage error
- **AND** the bridge SHALL NOT emit a SAML response for that request
- **AND** subsequent authentications SHALL re-attempt resolution rather than
  caching a fallback value

### Requirement: Persistent NameID storage durability

The system SHALL persist each generated persistent NameID before it
is returned to the caller when configured in `pairwise` mode,
so that the same NameID is recoverable on subsequent authentications.

In `public` mode (the default when `persistent_type` is omitted or empty),
resolution SHALL NOT interact with or persist records in the persistent
NameID repository.

When configured in `pairwise` mode, the storage SHALL guarantee
at-most-one persistent NameID per `(service-provider EntityID, upstream
user OIDC sub)` pair, even under concurrent first-time-authentication
requests for the same pair. The first writer's value SHALL be returned
to all concurrent callers; no caller SHALL receive a value that is not also
persisted for future reads.

When a service provider record is removed from the bridge, all
persistent NameID rows associated with that service provider SHALL
be removed automatically.

The system SHALL NOT remove persistent NameID rows for upstream
users that disappear from the OIDC provider; user-lifecycle cleanup
is explicitly out of scope for this capability version.

#### Scenario: First and second resolution return the same persisted value

- **WHEN** a user authenticates for the first time to an SP configured in
  `pairwise` mode
- **THEN** the generated UUID SHALL be written to the database before the
  SAML response is returned

#### Scenario: Concurrent first-time resolution converges on one value

- **WHEN** two concurrent requests arrive for the same user and SP in `pairwise`
  mode
- **THEN** the database SHALL ensure a single UUID is persisted and returned
  to both callers

#### Scenario: Service provider removal cascades to NameIDs

- **WHEN** a service provider record is deleted
- **THEN** associated pairwise `persistent_nameids` rows SHALL be removed

#### Scenario: Upstream user removal does not cascade

- **WHEN** an upstream user account is removed from the OIDC provider
- **THEN** existing `persistent_nameids` rows SHALL remain untouched

#### Scenario: Shared mode bypasses persistent storage

- **WHEN** a user authenticates to an SP configured in `public` mode (or with `persistent_type` omitted)
- **THEN** no database query or insert SHALL be executed against the
  `persistent_nameids` repository
