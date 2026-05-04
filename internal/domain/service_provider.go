package domain

// ServiceProvider represents a registered SAML Service Provider.
type ServiceProvider struct {
	EntityID         string
	ACSURL           string
	ACSBinding       string
	AttributeMapping *AttributeMapping // per-SP attribute mapping config (nil = use defaults)
}
