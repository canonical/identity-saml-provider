# Proposal: Custom Assertion Maker

## Why

The current field-clearing workaround used to suppress default SAML attributes
is fragile. If the underlying `crewjam/saml` library adds new default
attributes in the future, they would leak into mapped assertions. A proper
custom `AssertionMaker` provides full control over assertion construction,
ensuring long-term robustness and strict adherence to the per-SP attribute
mapping configuration without relying on internal library implementation
details.

## What Changes

- Implement a custom `SPAssertionMaker` that implements the
  `saml.AssertionMaker` interface to construct SAML assertions directly.
- Replace the fragile field-clearing mechanism for SPs with attribute mappings.
- Delegate assertion creation to `DefaultAssertionMaker` only for SPs without
  SAML mapping configurations.

## Capabilities

### New Capabilities

None

### Modified Capabilities

None (This change focuses on the internal implementation of the existing
"Suppress default attributes when mapping is active" requirement and does not
alter the product's observable behavior or contract).

## Impact

- **Affected Code**: `internal/handler/assertion_maker.go`,
  `internal/service/attribute_mapping.go`, and SAML IdP initialization.
- **Systems**: SAML assertion generation flow.

## Non-goals

- Changing the user-facing admin API, CLI, or database schema.
- Forking or modifying the upstream `crewjam/saml` library.
- Introducing new SAML attribute mapping features or syntax.

## Success Metrics

- **Robustness**: Mapped SPs receive assertions exactly matching their
  configuration without any leaked default attributes, verified by test coverage
  independent of `DefaultAssertionMaker` internals.
- **Backward Compatibility**: Unmapped SPs continue to receive the standard
  default attributes, and no existing authentication flows regress.
