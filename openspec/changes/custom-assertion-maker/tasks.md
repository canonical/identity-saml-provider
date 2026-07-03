# Tasks: Custom Assertion Maker

## 1. Domain / Handler Layer Preparation

- [x] 1.1 Create `internal/handler/assertion_maker.go` and define the
  `SPAssertionMaker` struct. It should hold dependencies:
  `service.ServiceProviderService`, a logger, and an
  embedded or composite `saml.DefaultAssertionMaker`.
- [x] 1.2 Implement the constructor `NewSPAssertionMaker(spService
  service.ServiceProviderService, logger logging.Logger) *SPAssertionMaker`.

## 2. Core Implementation

- [x] 2.1 Implement `MakeAssertion(req *saml.IdpAuthnRequest, session
  *saml.Session) error` on `SPAssertionMaker` to satisfy the
  `saml.AssertionMaker` interface.
- [x] 2.2 Add SP lookup logic in `MakeAssertion` using the EntityID from the
  request metadata. If lookup fails, return an error (fail closed).
- [x] 2.3 Add conditional logic in `MakeAssertion`: If the SP has no
  `AttributeMapping` or empty `SAMLAttributeMappings`, delegate to
  `m.defaultMaker.MakeAssertion(req, session)`.
- [x] 2.4 Implement `buildCustomAssertion(req *saml.IdpAuthnRequest, session
  *saml.Session)` as a helper to manually construct the SAML assertion
  elements: `Issuer`, `Subject` (with `NameID` and `SubjectConfirmation`),
  `Conditions`, `AuthnStatement`, and `AttributeStatement` (populated *only*
  from `session.CustomAttributes`). Call this from `MakeAssertion` when custom
  mappings exist.

## 3. Integration & Cleanup

- [x] 3.1 Wire `SPAssertionMaker` into the App: Modify `internal/app/app.go` to
  instantiate `handler.NewSPAssertionMaker` and inject it into the
  `samlIDP.AssertionMaker` during SAML initialization.
- [x] 3.2 Remove the existing field-clearing workaround from
  `MappingService.ApplyMapping` in `internal/service/attribute_mapping.go`.
  `ApplyMapping` should now *only* populate `CustomAttributes` and the `NameID`
  / `NameIDFormat`.

## 4. Testing

- [x] 4.1 Write unit tests for `SPAssertionMaker.MakeAssertion` in
  `internal/handler/assertion_maker_test.go` covering:
  - [x] Delegation to default maker for an SP with no mapping.
  - [x] Construction of custom assertion for an SP with custom attributes,
    verifying elements and no leakage of unused built-in claims.
  - [x] SP lookup failure handling.
- [x] 4.2 Verify `MappingService` unit tests in
  `internal/service/attribute_mapping_test.go` continue to pass, updating
  assertions to reflect the removal of the field-clearing workaround.

## 5. Verification Suite

- [x] 5.1 `make fmt` - Ensure all code is correctly formatted.
- [x] 5.2 `make lint` - Ensure no linting errors are introduced.
- [x] 5.3 `make test` - Verify all tests pass, including the new
  `SPAssertionMaker` tests and updated `MappingService` tests.
- [x] 5.4 `make license-check` - Ensure all newly created Go files have the
  required AGPL-3.0 license header.
- [x] 5.5 `make generate` - Re-run code generation (e.g., mocks) if interfaces
  were modified or added.
- [x] 5.6 `make build` - Verify the application successfully compiles.

## 6. Documentation & Rollout

- [x] 6.1 No user-facing documentation updates are required, as this is an
  internal refactoring that does not change the API or observable behavior.

## Implementation Notes

- No additional implementation notes.
