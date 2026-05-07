# Example SAML Service Provider (SP) - Makefile

.PHONY: \
	help \
	build run clean \
	certs \
	register register-with-email register-with-persistent register-with-transient

# Variables

SAML_PROVIDER_URL ?= http://localhost:8082
SAML_SP_URL       ?= http://localhost:8083

# Help

help:
	@echo "Example SAML Service Provider Makefile"
	@echo ""
	@echo "Build & Run:"
	@echo "  build                    - Build the example SAML SP"
	@echo "  run                      - Run the example SAML SP"
	@echo ""
	@echo "Registration (SAML_PROVIDER_URL=$(SAML_PROVIDER_URL), SAML_SP_URL=$(SAML_SP_URL)):"
	@echo "  register                 - Register SP with default attributes (no mapping)"
	@echo "  register-with-email      - Register SP with emailAddress NameID format (mapping-email.json)"
	@echo "  register-with-persistent - Register SP with persistent NameID format (mapping-persistent.json)"
	@echo "  register-with-transient  - Register SP with transient NameID format (mapping-transient.json)"
	@echo ""
	@echo "Certificates:"
	@echo "  certs                    - Generate certificates for the example SAML SP"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean                    - Remove build artifacts"

# Build & Run

build:
	go build -o bin/example-saml-service

run: certs build
	./bin/example-saml-service

clean:
	@rm -f bin/example-saml-service

# Registration

register:
	@curl -sS --fail -w '\n' -X POST -H "Content-Type: application/json" \
		$(SAML_PROVIDER_URL)/admin/service-providers \
		-d '{"entity_id":"$(SAML_SP_URL)/saml/metadata", "acs_url":"$(SAML_SP_URL)/saml/acs"}'

register-with-email: _check-jq
	@curl -sS --fail -w '\n' -X POST -H "Content-Type: application/json" \
		$(SAML_PROVIDER_URL)/admin/service-providers \
		-d "$$(jq -n --slurpfile m mapping-email.json \
			'{entity_id:"$(SAML_SP_URL)/saml/metadata", acs_url:"$(SAML_SP_URL)/saml/acs", attribute_mapping: $$m[0]}')"

register-with-persistent: _check-jq
	@curl -sS --fail -w '\n' -X POST -H "Content-Type: application/json" \
		$(SAML_PROVIDER_URL)/admin/service-providers \
		-d "$$(jq -n --slurpfile m mapping-persistent.json \
			'{entity_id:"$(SAML_SP_URL)/saml/metadata", acs_url:"$(SAML_SP_URL)/saml/acs", attribute_mapping: $$m[0]}')"

register-with-transient: _check-jq
	@curl -sS --fail -w '\n' -X POST -H "Content-Type: application/json" \
		$(SAML_PROVIDER_URL)/admin/service-providers \
		-d "$$(jq -n --slurpfile m mapping-transient.json \
			'{entity_id:"$(SAML_SP_URL)/saml/metadata", acs_url:"$(SAML_SP_URL)/saml/acs", attribute_mapping: $$m[0]}')"

.PHONY: _check-jq
_check-jq:
	@command -v jq >/dev/null 2>&1 || { echo "Error: jq is required but not installed."; exit 1; }

# Certificates

certs:
	@mkdir -p etc/certs
	@if [ ! -f etc/certs/sp.key ] || [ ! -f etc/certs/sp.crt ]; then \
		echo "Generating SP certificates..."; \
		openssl req -x509 -newkey rsa:2048 -keyout etc/certs/sp.key -out etc/certs/sp.crt \
			-days 365 -nodes \
			-subj "/C=US/ST=State/L=City/O=Organization/CN=localhost"; \
		echo "Certificates generated: etc/certs/sp.crt and etc/certs/sp.key"; \
	else \
		echo "SP certificates already exist"; \
	fi
