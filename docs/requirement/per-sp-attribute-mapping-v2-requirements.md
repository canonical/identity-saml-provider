# Requirement Analysis: Per-SP Attribute Mapping — Phase 2

## TL;DR

Phase 1 of the per-SP attribute mapping feature delivered the core pipeline: SPs
can be registered with attribute mapping config, raw OIDC claims are preserved
in sessions, and per-SP mappings are applied at assertion time. However, several
capabilities required for production readiness are still missing — most
critically, true persistent NameID support, richer SAML attribute metadata, and
administrative API endpoints for managing mappings post-registration.

This document defines the remaining requirements from a product perspective.

---

## 1. Background

### What Phase 1 Delivered

- Per-SP attribute mapping configuration stored alongside SP registration.
- OIDC claim extraction and mapping to SAML attributes based on SP config.
- NameID format selection (persistent, transient, emailAddress) per SP.
- `lowercase_email` transform option.
- CLI flags for `--nameid-format` and `--attribute-mapping-file` at registration
  time.
- Raw OIDC claims preserved in sessions for dynamic mapping.

### What's Missing

Feedback from integration testing and SP onboarding has identified gaps that
prevent the feature from meeting the needs of real-world SAML Service Providers.

---

## 2. Problem Statements

### P1: Persistent NameID is not truly persistent

When an SP requests `nameid_format=persistent`, the bridge returns the raw OIDC
`sub` claim as the NameID. This violates the SAML 2.0 specification, which
requires persistent NameIDs to be:

- **Opaque**: Not derived from any meaningful user attribute.
- **Pairwise**: Different for each SP, even for the same user.
- **Stable**: The same value on every authentication for a given (SP, user)
  pair.

SPs that rely on persistent NameIDs (e.g., enterprise provisioning systems) will
receive identifiers that are neither opaque nor pairwise, creating privacy and
interoperability issues.

### P2: SAML attribute metadata is incomplete

The current mapping allows administrators to specify the SAML attribute *name*
for each field (e.g., `"email": "mail"`), but many SPs require additional
metadata per attribute:

- **FriendlyName**: A human-readable label (e.g., `"mail"`) required by some SPs
  for attribute matching.
- **NameFormat**: The attribute name format URI (e.g.,
  `urn:oasis:names:tc:SAML:2.0:attrname-format:uri` for OID-based names vs.
  `basic` for simple names). Some SPs reject assertions where the NameFormat
  doesn't match their expectations.

Without per-attribute metadata, administrators cannot fully satisfy the
attribute requirements of SPs like NetSuite, ServiceNow, or Shibboleth-based
systems.

### P3: No way to view or update mappings after registration

Once an SP is registered, there is no API to:

- **Retrieve** the current attribute mapping configuration.
- **Update** the mapping without re-registering the SP entirely.

This forces administrators to delete and re-register SPs to change mappings,
which is disruptive (loses any associated state) and error-prone.

### P4: Internal user model lacks structure

The internal representation of user attributes (between OIDC claim extraction
and SAML attribute building) is an untyped key-value map. This makes the mapping
pipeline harder to reason about, test, and extend — particularly for
multi-valued attributes like groups, which require a workaround encoding.

### P5: Default attribute leakage in mapped assertions

When per-SP SAML attributes are configured, the bridge suppresses default
attribute emission by clearing internal session fields. This approach is fragile
— it depends on knowing which session fields the SAML library uses to generate
default attributes. If the library changes its defaults, unwanted attributes
could appear in mapped assertions.

### P6: No validation of internal field references in mapping config

The mapping configuration has two maps that reference internal user model
fields: `oidc_claim_mappings` (OIDC claim → internal field) and
`saml_attribute_mappings` (internal field → SAML attribute). Neither map is
validated against the known set of internal fields (`subject`, `email`, `name`,
`groups`). A typo like `"emal"` instead of `"email"` is silently accepted and
only surfaces at authentication time as a missing attribute — with no error,
only a DEBUG-level log. Cross-referencing between the two maps is also absent:
an admin can configure a SAML attribute for a field that no OIDC claim ever
populates.

### P7: Overlapping and confusing CLI flags for SP registration

The `sp add` CLI currently exposes two separate flags for attribute mapping
configuration: `--attribute-mapping-file` (a JSON file containing the full
`AttributeMapping` struct) and `--nameid-format` (a standalone flag for the
NameID format). These overlap and create confusion:

- **Silent priority**: When both flags are provided, `--attribute-mapping-file`
  wins and `--nameid-format` is silently ignored — no error or warning is
  emitted.
- **Conceptual duplication**: `--nameid-format` sets
  `AttributeMapping.NameIDFormat`, which is already a field inside the mapping
  file. The flag is a duplicate entry point for the same config value.
- **Doesn't scale**: Phase 2 adds richer config (`SAMLAttributeDef` with
  `friendly_name`, `name_format`; cross-map validation; `options`).
  Cherry-picking individual fields as CLI flags creates an unbounded surface —
  if `--nameid-format` gets its own flag, so should `--lowercase-email`, etc.
- **False separation**: Users perceive NameID format and attribute mapping as
  independent concepts, but `NameIDFormat` is part of `AttributeMapping` and
  interacts deeply with other mapping fields (e.g., persistent NameID requires
  canonical subject extraction from `oidc_claim_mappings`).

---

## 3. Functional Requirements

| ID | Requirement | Acceptance Criteria |
| --- | --- | --- |
| FR-1 | **Structured internal user model**: The bridge must use a structured representation for user attributes with typed fields for subject, email, name, groups, and an extensible map for custom claims. Multi-valued fields (e.g., groups) must be serialized as multiple distinct `<saml:AttributeValue>` elements under a single `<saml:Attribute>` element, per SAML 2.0 core specification. | Multi-valued fields (groups) are natively supported as `[]string`. Custom OIDC claims are captured without workaround encodings. A group list `["admin", "users"]` produces one `<saml:Attribute>` with two `<saml:AttributeValue>` children. |
| FR-2 | **Rich SAML attribute definitions**: Each SAML attribute in the mapping config must support `name` (required), `friendly_name` (optional), and `name_format` (optional, with a sensible default). | An SP can request `urn:oid:0.9.2342.19200300.100.1.3` as the attribute Name with `mail` as FriendlyName and `uri` as NameFormat. Attributes without explicit NameFormat default to `urn:oasis:names:tc:SAML:2.0:attrname-format:uri`. |
| FR-3 | **True persistent NameID**: When `nameid_format=persistent`, the bridge must generate an opaque, randomly-generated identifier unique to each (SP, user) pair. The identifier must be stable across sessions and stored durably. | Same user authenticating to the same SP always receives the same NameID. Different SPs receive different NameIDs for the same user. The NameID is a random UUID, not derived from any user attribute. |
| FR-4 | **Retrieve SP configuration**: Administrators must be able to retrieve the full configuration of a registered SP, including its attribute mapping, via the admin API. | `GET /admin/service-providers/{entity_id}` returns the SP's entity ID, ACS URL, ACS binding, and attribute mapping (if configured). Returns 404 for unknown SPs. |
| FR-5 | **Update attribute mapping**: Administrators must be able to update the attribute mapping of an existing SP without re-registering it. | `PUT /admin/service-providers/{entity_id}/attribute-mapping` accepts a new mapping config, validates it, and persists it. Returns 400 for invalid config, 404 for unknown SPs. |
| FR-6 | **Robust assertion control**: The bridge must have full control over which attributes appear in SAML assertions for mapped SPs, without relying on suppression of library defaults. An SP is considered "mapped" only when it has non-empty `saml_attribute_mappings`; an SP with only `nameid_format` set (and no `saml_attribute_mappings`) is treated as unmapped for attribute purposes and receives default attributes. | Mapped SPs (non-empty `saml_attribute_mappings`) receive only the attributes defined in their mapping config. No default attributes leak into the assertion. SPs with only `nameid_format` and no `saml_attribute_mappings` receive default attributes with the configured NameID format. Unmapped SPs (no `AttributeMapping` at all) continue to receive the same assertions as today. |
| FR-7 | **Semantic validation of internal field references**: The bridge must validate that internal field names used in `oidc_claim_mappings` targets and `saml_attribute_mappings` keys are consistent and resolvable. Every `saml_attribute_mappings` key must either be a well-known field (`subject`, `email`, `name`, `groups`) or appear as a target value in `oidc_claim_mappings`. | Configuration with a misspelled field (e.g., `"emal"` instead of `"email"`) is rejected at registration/update time with an actionable error message identifying the unresolvable field. Custom fields (e.g., `"dept"`) are accepted when consistently defined in both maps. |
| FR-8 | **Unified CLI mapping input**: The `sp add` CLI must use `--attribute-mapping-file` as the sole entry point for all mapping configuration, including NameID format. The separate `--nameid-format` flag must be removed. | `sp add --attribute-mapping-file mapping.json` applies the full config (including `nameid_format`). Passing `--nameid-format` produces a clear error directing the user to include `nameid_format` in the mapping file. A minimal `{"nameid_format": "persistent"}` file is valid for the common "just set NameID format" case. |

---

## 4. Non-Functional Requirements

| ID | Requirement |
| --- | --- |
| NFR-1 | **Pre-production scope**: This project has not been released to production. Phase 1 backward compatibility (e.g., migrating existing Phase 1 mapping data, supporting the Phase 1 `map[string]string` JSONB format) is not required. Phase 1 data may be discarded during migration. SPs without mapping config must still produce identical assertions to unmapped behavior. |
| NFR-2 | **Performance**: Persistent NameID lookup must not add perceptible latency to authentication flows. Admin API endpoints are not on the hot path and have no strict latency requirements. |
| NFR-3 | **Data durability**: Persistent NameIDs, once generated, must survive server restarts, migrations, and scaling events. Loss of persistent NameIDs would break user identity continuity for affected SPs. |
| NFR-4 | **Observability**: Persistent NameID creation and mapping resolution decisions must be observable through structured logging and tracing. |
| NFR-5 | **Validation**: Invalid mapping configurations must be rejected at configuration time (registration or update) with clear, actionable error messages. This includes: (a) structural validation (required fields, valid formats), and (b) semantic validation — `oidc_claim_mappings` targets and `saml_attribute_mappings` keys must reference either a well-known internal field (`subject`, `email`, `name`, `groups`) or a custom field that is consistently defined across both maps. A `saml_attribute_mappings` key that is neither a well-known field nor a target in `oidc_claim_mappings` must be rejected, since the field would never be populated. |
| NFR-6 | **Data cleanup**: The persistent NameID table must use a foreign key with `ON DELETE CASCADE` referencing the service providers table, ensuring that if an SP is ever deleted (e.g., via direct DB operation), all associated persistent NameIDs are automatically removed. No SP deletion API is included in this phase. Note: cleanup of persistent NameIDs for deleted *users* is not addressed — the bridge has no user lifecycle awareness (see Out of Scope). Orphaned rows from deleted upstream users are inert and do not affect correctness. |

---

## 5. User Stories

### Persistent NameID

> As an **administrator onboarding an enterprise SP**,
> I want to configure persistent NameID format for the SP,
> so that the SP receives a stable, opaque user identifier that doesn't expose
> internal user data and is unique to that SP.

### Rich SAML Attributes

> As an **administrator integrating with a legacy SP**,
> I want to specify FriendlyName and NameFormat for each SAML attribute,
> so that the SP can match attributes correctly based on both OID names and
> friendly names.

### Admin API

> As an **administrator managing SP configurations**,
> I want to view and update an SP's attribute mapping without re-registering it,
> so that I can iterate on SP configuration safely and without downtime.

### Unified CLI Mapping Input

> As an **administrator registering an SP via CLI**,
> I want a single flag for all mapping configuration,
> so that I don't have to guess which flag takes priority or worry about silent
> overrides between `--attribute-mapping-file` and `--nameid-format`.

---

## 6. Edge Cases

| Case | Expected Behavior |
| --- | --- |
| SP requests `persistent` NameID but user has never authenticated to this SP | A new persistent ID is generated, stored, and returned. |
| Two concurrent authentication requests for the same (SP, user) pair with `persistent` NameID | Both requests receive the same persistent ID (no duplicates). |
| SP mapping references an OIDC claim not present in the ID token | The corresponding attribute is omitted from the assertion (no error). |
| SP mapping specifies a SAML attribute with empty `name` | Rejected at registration/update time with a validation error. |
| `oidc_claim_mappings` maps an OIDC claim to a misspelled well-known field (e.g., `"email": "emal"`) | The value is treated as a custom field. If `saml_attribute_mappings` references `"email"` (the well-known field), `attrs.Email` is empty and the SAML attribute is silently omitted. Validation should reject this: `"emal"` is not a well-known field and `"email"` in `saml_attribute_mappings` has no corresponding OIDC source mapping to `"emal"`. |
| `saml_attribute_mappings` references a field that is neither well-known nor a target in `oidc_claim_mappings` | Rejected at configuration time — the field would never be populated, so the SAML attribute would always be omitted. |
| `oidc_claim_mappings` maps a claim to a custom field (e.g., `"department": "dept"`) and `saml_attribute_mappings` references `"dept"` | Valid — custom fields are allowed when consistently defined across both maps. The value flows through `Custom["dept"]` → SAML attribute. |
| Administrator updates mapping while a user has an active session | Mapping updates take effect on the next assertion generation. Active authentication flows that have not yet reached assertion generation may use the updated mapping. |
| SP is deleted (via direct DB operation) | All associated persistent NameIDs are automatically cleaned up via `ON DELETE CASCADE`. No SP deletion API exists in this phase. |
| `sp add` with removed `--nameid-format` flag | CLI returns an error explaining the flag has been removed and directs the user to use `--attribute-mapping-file` with `{"nameid_format": "persistent"}` instead. |
| `sp add` with `--attribute-mapping-file` containing only `{"nameid_format": "persistent"}` | Valid — registers SP with persistent NameID format and no custom attribute mappings. Because `saml_attribute_mappings` is empty, the SP is treated as unmapped for attribute purposes: default attributes are preserved, only the NameID format changes. This is equivalent to the old `--nameid-format persistent` shortcut. |

---

## 7. Out of Scope

- **YAML config file support**: Configuration is managed through the database,
  admin API, and CLI.
- **Dynamic attribute release policies** (consent-based filtering per user).
- **Complex transforms** beyond `lowercase_email` (regex, scripting, computed
  attributes).
- **Multi-IdP support** (mapping from multiple upstream OIDC providers).
- **SAML metadata-driven attribute negotiation** (reading SP metadata
  `<AttributeConsumingService>`).
- **Hot-reload of configuration** (changes require re-authentication, not server
  restart).
- **Admin API authentication/authorization** (existing endpoint has none; not in
  scope for this feature).
- **User lifecycle cleanup**: The bridge has no user management or awareness of
  upstream user deletions. If a user is deleted from the OIDC provider, their
  `persistent_nameids` rows remain in the database. These orphaned rows are
  inert (never queried if the user no longer authenticates) and have negligible
  storage impact. A periodic cleanup mechanism (e.g., TTL-based purge of rows
  not accessed in N months) may be considered in a future phase.

---

## 8. Success Criteria

| Criterion | Measurement |
| --- | --- |
| Persistent NameID correctness | Same user + same SP = same NameID across 100 consecutive authentications. Different SPs = different NameIDs. NameID is a valid UUID. |
| SAML attribute completeness | Assertions contain `FriendlyName` and `NameFormat` matching the SP's mapping config, verified by XML inspection. |
| No attribute leakage | Mapped SPs receive exactly the attributes in their config — no additional default attributes present in the assertion XML. |
| Unmapped SP behavior | SPs without mapping config produce identical assertions to unmapped behavior. |
| Admin API usability | GET returns current config; PUT → GET round-trip preserves all fields. |
| CLI single input path | All mapping config (including NameID format) is provided via `--attribute-mapping-file` only. No silent flag priority conflicts. Minimal JSON `{"nameid_format": "persistent"}` works for the common case. |
