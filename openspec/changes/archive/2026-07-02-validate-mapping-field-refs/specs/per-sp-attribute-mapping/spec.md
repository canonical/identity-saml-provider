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
