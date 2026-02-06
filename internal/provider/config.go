package provider

// Config defines the configuration for the SAML provider
type Config struct {
	// Bridge Configuration
	BridgeBasePort string `envconfig:"SAML_PROVIDER_BRIDGE_BASE_PORT" default:"8082"`
	BridgeBaseURL  string `envconfig:"SAML_PROVIDER_BRIDGE_BASE_URL" default:"http://localhost:8082"`

	// Ory Hydra Configuration
	HydraPublicURL string `envconfig:"SAML_PROVIDER_HYDRA_PUBLIC_URL" default:"http://localhost:4444"`
	ClientID       string `envconfig:"SAML_PROVIDER_OIDC_CLIENT_ID" default:"service-bridge-client"`
	ClientSecret   string `envconfig:"SAML_PROVIDER_OIDC_CLIENT_SECRET" default:"secret"`

	// Service Configuration
	ServiceACS      string `envconfig:"SAML_PROVIDER_SERVICE_ACS" default:"http://localhost:8083/saml/acs"`
	ServiceEntityID string `envconfig:"SAML_PROVIDER_SERVICE_ENTITY_ID" default:"http://localhost:8083/saml/metadata"`

	// Database Configuration
	DBHost     string `envconfig:"SAML_PROVIDER_DB_HOST" default:"localhost"`
	DBPort     string `envconfig:"SAML_PROVIDER_DB_PORT" default:"5432"`
	DBName     string `envconfig:"SAML_PROVIDER_DB_NAME" default:"saml_provider"`
	DBUser     string `envconfig:"SAML_PROVIDER_DB_USER" default:"saml_provider"`
	DBPassword string `envconfig:"SAML_PROVIDER_DB_PASSWORD" default:"saml_provider"`

	// Certificate Configuration
	SAMLCertPath string `envconfig:"SAML_PROVIDER_CERT_PATH" default:".local/certs/bridge.crt"`
	SAMLKeyPath  string `envconfig:"SAML_PROVIDER_KEY_PATH" default:".local/certs/bridge.key"`
}
