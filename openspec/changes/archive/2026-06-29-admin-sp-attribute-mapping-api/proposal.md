## Why

Today, once a Service Provider is registered with the bridge there is no
way to look at its attribute mapping or change it. Operators who need to
add a FriendlyName, fix a misspelled OIDC claim, or switch a NameID
format must delete and re-register the SP, which is disruptive
(persistent NameID rows for that SP are cascade-deleted) and easy to get
wrong. There is also no read endpoint, so operators have no way to
confirm what the bridge actually has stored for a given SP.

Phase 2d of the per-SP attribute mapping plan fills this gap with a
small, focused admin HTTP surface — `GET`, `PUT`, and `DELETE` — so
operators can view, replace, and clear an SP's mapping without
re-registration and without database access.

## What Changes

- Add `GET /admin/service-providers?entity_id=<id>` that returns the
  SP's `entity_id`, `acs_url`, `acs_binding`, and the currently
  persisted `attribute_mapping` (or `null` when none is configured).
- Add `PUT /admin/service-providers/attribute-mapping?entity_id=<id>`
  that accepts an `AttributeMapping` JSON body, validates it, and
  replaces whatever mapping the SP currently has.
- Add `DELETE /admin/service-providers/attribute-mapping?entity_id=<id>`
  that clears the SP's mapping, reverting the SP to default (unmapped)
  assertion behaviour — semantically identical to an SP registered
  without `--attribute-mapping-file`.
- Use `entity_id` as a **query parameter** (not a path parameter) on
  every new endpoint, because SAML EntityIDs are URLs containing `/`,
  `:`, and `?` that are unsafe to embed in a URL path.
- Define the precise difference between `PUT {}` (configured mapping
  with all defaults) and `DELETE` (no mapping at all) as part of the
  contract.
- Return `404` for unknown entity IDs, `400` for missing `entity_id` /
  invalid JSON / validation failures, and `200` on success.
- Extend the service and repository layers with `UpdateAttributeMapping`
  and `ClearAttributeMapping` operations, with defence-in-depth
  re-validation at the service layer.
- Document the new endpoints in the README and in the existing admin
  API surface.

## Non-goals

- **Authentication / authorisation on the admin API.** The existing
  `POST /admin/service-providers` endpoint has no auth today; this
  change deliberately keeps the same posture and defers auth to a
  later capability, so this proposal does not introduce auth headers,
  tokens, or RBAC.
- **SP deletion endpoint.** Deleting an SP entirely is out of scope;
  this change only manages the `attribute_mapping` field on an
  existing SP.
- **Listing endpoint** (`GET /admin/service-providers` with no
  `entity_id`). Out of scope; a list/paginate surface is a separate
  concern.
- **Partial / patch updates.** `PUT` is a full replacement of the
  mapping document; field-level patching is not in scope.
- **Hot-reload of in-flight authentications.** Updates take effect on
  the next assertion build; in-flight flows that have already passed
  attribute mapping are not retroactively rewritten.
- **YAML or other config-file formats over the wire.** The API is
  JSON-only.
- **Phase 1 mapping compatibility.** The bridge is pre-production and
  Phase 1 JSONB layout is already unsupported by `AttributeMapping`.

## Capabilities

### New Capabilities

None — this change extends the existing capability rather than adding
a new one.

### Modified Capabilities

- `per-sp-attribute-mapping`: adds admin-API requirements for
  retrieving, updating, and clearing an SP's attribute mapping after
  registration, plus the contract distinction between `PUT {}` and
  `DELETE`.

## Impact

- **HTTP surface**: three new routes registered in
  `internal/handler/routes.go` under `/admin/service-providers...`.
- **Handlers**: new handlers in `internal/handler/admin.go`
  (`HandleGetServiceProvider`, `HandleUpdateAttributeMapping`,
  `HandleDeleteAttributeMapping`) plus a response DTO so
  `domain.ServiceProvider` does not need JSON tags.
- **Service layer**: `service.ServiceProviderService` gains
  `UpdateAttributeMapping` and `ClearAttributeMapping`. Mocks
  regenerated via `make generate`.
- **Repository layer**:
  `repository.ServiceProviderRepository` gains an update operation
  for the `attribute_mapping` JSONB column (accepting `nil` for
  clear, or split into two methods — to be decided in `design.md`).
  Postgres implementation in `internal/repository/postgres/`.
- **Domain**: no domain changes — the existing
  `AttributeMapping.Validate()` is reused.
- **Specs**: delta added to
  `openspec/specs/per-sp-attribute-mapping/spec.md` covering the new
  admin-API requirements and the `PUT {}` vs `DELETE` contract.
- **Docs**: README admin-API section updated; `docs/authentication-flow/`
  unchanged (these endpoints are off the runtime authentication path).
- **Database**: no migration required.
- **Dependencies**: no new dependencies.
- **Observability**: handlers log via the standard structured logger
  and inherit the existing tracing middleware; no new metrics
  introduced.

## Success Metrics

- Operators can change an SP's attribute mapping (e.g., add a
  `friendly_name`, switch NameID format) without re-registering the SP
  and without losing the SP's persistent NameID rows.
- `GET → PUT → GET` round-trip on the same SP returns a body
  byte-equivalent (after canonical JSON ordering) to what was sent,
  for every supported `AttributeMapping` shape.
- `DELETE` on an SP that had a mapping leaves the SP serving the
  exact same assertion as an SP that was registered without a
  mapping, verified by an XML-level assertion comparison test.
- 100% of error cases (missing `entity_id`, invalid JSON, validation
  failure, unknown SP) return a typed JSON error with the documented
  status code, verified by handler tests.
- No regression in existing `POST /admin/service-providers`
  behaviour, verified by the existing handler tests continuing to
  pass.
