# Design: Custom Assertion Maker

## Context

Currently, to suppress the default SAML attributes for SPs with custom
attribute mappings, the bridge explicitly clears out the standard fields in the
session (e.g., `UserEmail`, `UserCommonName`, etc.). This approach implicitly
relies on the internal implementation of `crewjam/saml`'s
`DefaultAssertionMaker`. If the upstream library begins emitting new default
attributes based on other session fields, or changes its logic, those
attributes will leak into our custom mapped assertions.

To fix this fragility, we need to implement a custom `AssertionMaker` that
assumes full control over the assertion construction for mapped SPs, while
delegating to the `DefaultAssertionMaker` for unmapped SPs.

## Goals / Non-Goals

**Goals:**

- Eliminate reliance on the `crewjam/saml` library's `DefaultAssertionMaker`
  for SPs with custom attribute mappings.
- Implement `SPAssertionMaker`, which builds the SAML assertion directly to
  ensure only explicitly mapped attributes are included.
- Maintain exact backward compatibility for SPs without custom mappings.

**Non-Goals:**

- Forking or modifying `crewjam/saml`.
- Altering the user-facing database schema or configuration API.
- Re-architecting the mapping service logic itself (the `UserAttributes`
  construction and mapping rules remain unchanged).

## Decisions

### 1. Custom `SPAssertionMaker` over Field-Clearing

**Rationale:** The `saml.AssertionMaker` interface is the library's designated
extension point for controlling assertion generation. By implementing this
interface, we isolate our application from the internal behavior of
`DefaultAssertionMaker`.

**Alternatives Considered:**

- Maintain the current field-clearing approach: Fragile and tightly coupled to
  the library's internals.

### 2. Delegation to `DefaultAssertionMaker` for Unmapped SPs

**Rationale:** To ensure absolute backward compatibility and minimize risk, the
custom `SPAssertionMaker` will look up the SP's configuration. If no
`AttributeMapping` or `SAMLAttributeMappings` are configured, it will
immediately delegate the `MakeAssertion` call to the underlying
`DefaultAssertionMaker`.

**Alternatives Considered:**

- Re-implement the default logic entirely within the custom maker: Unnecessary
  duplication of library code and increased maintenance burden.

### 3. Structural Elements in `buildCustomAssertion`

**Rationale:** The custom maker must construct a complete and valid SAML
`<Assertion>` element. It will mirror the structure produced by
`DefaultAssertionMaker` but strictly control the `<saml:AttributeStatement>`.

The required elements are:

- `<saml:Issuer>`: Sourced from IDP metadata.
- `<saml:Subject>`: Containing `<saml:NameID>` (from `session.NameID` and
  `session.NameIDFormat`) and `<saml:SubjectConfirmation>`.
- `<saml:Conditions>`: Sourced from `NotBefore` and `NotOnOrAfter` timing
  windows.
- `<saml:AuthnStatement>`: Proving authentication occurred.
- `<saml:AttributeStatement>`: Populated *exclusively* from
  `session.CustomAttributes`.

**Alternatives Considered:**

- Modifying the XML after generation: Inefficient and error-prone compared to
  generating it correctly from the start.

## Risks / Trade-offs

- **Risk: Missed Mandatory Assertion Elements** → **Mitigation:** The
  implementation of `buildCustomAssertion` will be carefully audited against
  `DefaultAssertionMaker.MakeAssertion` to ensure all required SAML 2.0 elements
  (Issuer, Subject, Conditions, AuthnStatement, AudienceRestriction) are present
  and correctly formed.
- **Risk: SP Lookup Failure during Assertion Making** → **Mitigation:** The SP
  entity ID is known from the request. If the lookup fails unexpectedly (e.g.,
  DB error), the assertion maker will fail closed (return an error) rather than
  delegating to the default maker, preventing unintended attribute leakage.
