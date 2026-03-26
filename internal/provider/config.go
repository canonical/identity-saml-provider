package provider

// Config defines the configuration for the SAML provider
type Config struct {
	// Bridge Configuration
	BridgeBasePort string `envconfig:"SAML_PROVIDER_BRIDGE_BASE_PORT" default:"8082"`
	BridgeBaseURL  string `envconfig:"SAML_PROVIDER_BRIDGE_BASE_URL" default:"http://localhost:8082"`

	// Observability Configuration
	TracingEnabled   bool    `envconfig:"SAML_PROVIDER_TRACING_ENABLED" default:"false"`
	OtelHTTPEndpoint string  `envconfig:"SAML_PROVIDER_OTEL_HTTP_ENDPOINT" default:""`
	OtelGRPCEndpoint string  `envconfig:"SAML_PROVIDER_OTEL_GRPC_ENDPOINT" default:""`
	OtelSampler      string  `envconfig:"SAML_PROVIDER_OTEL_SAMPLER" default:"parentbased_traceidratio"`
	OtelSamplerRatio float64 `envconfig:"SAML_PROVIDER_OTEL_SAMPLER_RATIO" default:"0.1"`

	// Ory Hydra Configuration
	HydraPublicURL             string `envconfig:"SAML_PROVIDER_HYDRA_PUBLIC_URL" default:"http://localhost:4444"`
	HydraInsecureSkipTLSVerify bool   `envconfig:"SAML_PROVIDER_HYDRA_INSECURE_SKIP_TLS_VERIFY" default:"false"`
	HydraCACertPath            string `envconfig:"SAML_PROVIDER_HYDRA_CA_CERT_PATH" default:""`
	ClientID                   string `envconfig:"SAML_PROVIDER_OIDC_CLIENT_ID" default:"service-bridge-client"`
	ClientSecret               string `envconfig:"SAML_PROVIDER_OIDC_CLIENT_SECRET" default:"secret"`

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

	// SP Mapping Configuration
	MappingConfigPath string `envconfig:"SAML_PROVIDER_MAPPING_CONFIG_PATH" default:""`
}
