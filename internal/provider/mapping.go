package provider

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// SPMapping defines the OIDC-to-SAML attribute mapping for a specific Service Provider
type SPMapping struct {
	// NameIDFormat is the SAML NameID format URN to use in the assertion
	// e.g., "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"
	NameIDFormat string `yaml:"nameid_format"`

	// NameIDSource is the OIDC claim to use as the NameID value
	// e.g., "email", "sub", "preferred_username"
	NameIDSource string `yaml:"nameid_source"`

	// AttributeMap maps OIDC claim names to SAML attribute URIs
	// Key: OIDC claim name (e.g., "email", "name", "groups")
	// Value: SAML attribute URI (e.g., "urn:oid:0.9.2342.19200300.100.1.3")
	AttributeMap map[string]string `yaml:"attribute_map"`

	// Options holds optional transformation settings
	Options *SPMappingOptions `yaml:"options"`
}

// SPMappingOptions holds optional transformation and processing options
type SPMappingOptions struct {
	// LowercaseEmail converts the email claim to lowercase before mapping
	LowercaseEmail bool `yaml:"lowercase_email"`
}

// MappingConfig holds the default mapping and per-SP mappings
type MappingConfig struct {
	DefaultMapping   *SPMapping            `yaml:"default_mapping"`
	ServiceProviders map[string]*SPMapping `yaml:"service_providers"`
	logger           *zap.SugaredLogger
	configFilePath   string
}

// NewMappingConfig creates a new MappingConfig with defaults
func NewMappingConfig(logger *zap.SugaredLogger) *MappingConfig {
	return &MappingConfig{
		DefaultMapping:   getDefaultMapping(),
		ServiceProviders: make(map[string]*SPMapping),
		logger:           logger,
	}
}

// LoadFromFile loads the mapping configuration from a YAML file
func (mc *MappingConfig) LoadFromFile(filePath string) error {
	if filePath == "" {
		mc.logger.Info("No mapping config file specified, using defaults")
		return nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read mapping config file %q: %w", filePath, err)
	}

	if err := yaml.Unmarshal(data, mc); err != nil {
		return fmt.Errorf("failed to parse mapping config file %q: %w", filePath, err)
	}

	mc.configFilePath = filePath

	// Validate and backfill
	if mc.DefaultMapping == nil {
		mc.DefaultMapping = getDefaultMapping()
	}
	if mc.ServiceProviders == nil {
		mc.ServiceProviders = make(map[string]*SPMapping)
	}

	// Ensure all SP mappings have defaults for unset fields
	for spID, mapping := range mc.ServiceProviders {
		if mapping == nil {
			mapping = getDefaultMapping()
			mc.ServiceProviders[spID] = mapping
		}
		mc.ensureMappingDefaults(mapping)
	}

	mc.logger.Infow("Loaded mapping config from file", "path", filePath, "num_sps", len(mc.ServiceProviders))
	return nil
}

// GetMapping returns the mapping for a given Service Provider entity ID
// If the SP has a specific mapping, it is returned; otherwise the default is used
func (mc *MappingConfig) GetMapping(spEntityID string) *SPMapping {
	if mapping, ok := mc.ServiceProviders[spEntityID]; ok {
		return mapping
	}
	return mc.DefaultMapping
}

// ensureMappingDefaults fills in defaults for unset fields in a mapping
func (mc *MappingConfig) ensureMappingDefaults(mapping *SPMapping) {
	if mapping.NameIDFormat == "" {
		mapping.NameIDFormat = mc.DefaultMapping.NameIDFormat
	}
	if mapping.NameIDSource == "" {
		mapping.NameIDSource = mc.DefaultMapping.NameIDSource
	}
	if mapping.AttributeMap == nil && len(mapping.AttributeMap) == 0 {
		mapping.AttributeMap = mc.DefaultMapping.AttributeMap
	}
	if mapping.Options == nil {
		mapping.Options = mc.DefaultMapping.Options
	}
}

// getDefaultMapping returns the default OIDC-to-SAML mapping
func getDefaultMapping() *SPMapping {
	return &SPMapping{
		NameIDFormat: "urn:oasis:names:tc:SAML:2.0:nameid-format:transient",
		NameIDSource: "sub",
		AttributeMap: map[string]string{
			"email":  "urn:oid:0.9.2342.19200300.100.1.3", // mail
			"name":   "urn:oid:2.16.840.1.113730.3.1.241", // displayName
			"groups": "urn:oid:1.2.840.113556.1.4.221",    // memberOf
		},
		Options: &SPMappingOptions{
			LowercaseEmail: false,
		},
	}
}

// ClaimsMap is a convenience type for working with OIDC claims
type ClaimsMap map[string]interface{}

// GetString retrieves a string claim value, with optional lowercasing
func (cm ClaimsMap) GetString(claim string, lowercase bool) string {
	if v, ok := cm[claim]; ok {
		if str, ok := v.(string); ok {
			if lowercase {
				return strings.ToLower(str)
			}
			return str
		}
	}
	return ""
}

// GetStringSlice retrieves a string slice claim value (for groups, etc.)
func (cm ClaimsMap) GetStringSlice(claim string) []string {
	if v, ok := cm[claim]; ok {
		switch val := v.(type) {
		case []string:
			return val
		case []interface{}:
			result := make([]string, 0, len(val))
			for _, item := range val {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return []string{}
}
