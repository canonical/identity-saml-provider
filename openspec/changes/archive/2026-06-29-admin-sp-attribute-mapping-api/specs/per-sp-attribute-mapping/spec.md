## Purpose

These deltas extend the `per-sp-attribute-mapping` capability with an
admin HTTP surface for managing an SP's attribute mapping after
registration. Today the only entry point to write a mapping is the
`POST /admin/service-providers` registration call; once an SP exists,
its mapping is invisible and immutable from outside the process. These
requirements add three endpoints — `GET`, `PUT`, and `DELETE` — so
operators can view the persisted mapping, replace it in place, and
revert an SP to default (unmapped) behaviour without deleting and
re-registering the SP (which would also cascade-delete the SP's
persistent NameID rows).

The change is additive: the existing `POST` registration endpoint and
the runtime assertion-mapping behaviour are unchanged. No
authentication is introduced — the new endpoints match the existing
`POST /admin/service-providers` posture and will be wrapped in auth by
a later capability.

## ADDED Requirements

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

All three admin attribute-mapping endpoints (`GET`, `PUT`, `DELETE`)
SHALL return errors using the same JSON envelope already used by
`POST /admin/service-providers`, so admin-API clients see one
consistent error model across the surface.

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
