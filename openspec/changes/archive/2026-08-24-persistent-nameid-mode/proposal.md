# Proposal: Configurable Persistent NameID Mode (Public Default vs Pairwise)

## Why

When configuring Service Providers (SPs) with SAML NameID format `persistent`,
the bridge previously issued a generated pairwise UUID stored in the database
per `(SP, user)` pair. However, most SAML integrations and federated service
provider groups require a persistent identifier that is consistent across all
service providers (a "public" persistent NameID) so user identities can be
correlated across applications without relying on email addresses or database
lookup tables.

To align with standard OpenID Connect subject identifier terminology (`public`
vs `pairwise`), the bridge now defaults persistent NameIDs to `"public"` (emitting
the canonical upstream OIDC `sub` directly) while allowing operators to
explicitly configure `"pairwise"` for SPs requiring privacy-preserving per-SP
UUIDs.

## What Changes

- Add a new top-level configuration field `persistent_type` to
  `AttributeMapping` for SPs using `nameid_format: "persistent"`.
- Support two persistent NameID modes (conforming to OIDC subject types):
  - `"public"` (default / empty): Directly emits the canonical upstream subject
    identifier (OIDC `sub`) as the SAML NameID value across all SPs, bypassing
    per-SP database lookup and storage.
  - `"pairwise"`: Generates and stores an opaque RFC 4122 v4 UUID per `(SP Entity
    ID, user)` pair in the database.
- Update structural validation (`AttributeMapping.Validate`) to accept and
  validate `persistent_type` (`"public"` or `"pairwise"`), and reject
  `persistent_type` if configured alongside a non-persistent `nameid_format`
  (e.g., `transient` or `emailAddress`).
- Verify admin API JSON round-tripping (`GET`, `POST`, `PUT`
  `/admin/service-providers`) for `persistent_type` in unit tests.
- Update documentation to explain when and how to use public vs pairwise
  persistent NameIDs.

## Capabilities

### New Capabilities

<!-- None -->

### Modified Capabilities

- `per-sp-attribute-mapping`: Update requirements to support configurable
  persistent NameID resolution mode with `"public"` as default and `"pairwise"`
  as opt-in, update the AttributeMapping shape definition to 5 top-level fields,
  and scope fail-closed storage durability rules.

## Non-goals

- Dynamic switching or automatic migration of existing pairwise persistent
  UUIDs to public subjects for an SP already in production.
- Supporting arbitrary claim mappings (such as mapping email to persistent
  NameID) under the persistent format—`public` strictly maps to the canonical
  OIDC `sub`.
- User consent workflows or affiliation group mapping for sharing pairwise IDs
  across a subset of SPs.

## Success Metrics

- Unconfigured/default persistent SPs (`nameid_format: "persistent"`) automatically
  emit the canonical upstream subject ID in the SAML assertion's `<saml:NameID>`.
- Operators requiring per-SP pairwise isolation can explicitly configure
  `persistent_type: "pairwise"`.

## Impact

- **Breaking Change**: SPs configured with `nameid_format: "persistent"` that
  previously defaulted to pairwise UUID generation will now emit the raw OIDC
  `sub` unless explicitly updated with `persistent_type: "pairwise"`.
- **Domain Model**: `AttributeMapping` in `internal/domain/attribute_mapping.go`.
- **Service Layer**: `MappingService.resolveNameID` in
  `internal/service/attribute_mapping.go`.
- **Admin Handlers & Tests**: `internal/handler/admin_test.go`.
- **Documentation**: `docs/authentication-flow/authentication-flow.md` and
  `docs/design/per-sp-attribute-mapping-v2-design.md`.
