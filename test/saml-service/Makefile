.PHONY: help build run certs clean register register-with-email register-with-persistent register-with-transient

help:
	@echo "Example SAML Service Makefile"
	@echo ""
	@echo "Build & Run:"
	@echo "  build                    - Build the example service"
	@echo "  run                      - Run the example service"
	@echo ""
	@echo "Registration:"
	@echo "  register                 - Register SP with default attributes (no mapping)"
	@echo "  register-with-email      - Register SP with emailAddress NameID format (mapping-email.json)"
	@echo "  register-with-persistent - Register SP with persistent NameID format (mapping-persistent.json)"
	@echo "  register-with-transient  - Register SP with transient NameID format (mapping-transient.json)"
	@echo ""
	@echo "Certificates:"
	@echo "  certs                    - Generate certificates for the service"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean                    - Remove build artifacts"

build:
	go build -o bin/example-saml-service

run: certs
	go run .

register:
	curl -X POST -H "Content-Type: application/x-www-form-urlencoded" http://localhost:8082/admin/service-providers -d "entity_id=http://localhost:8083/saml/metadata" -d "acs_url=http://localhost:8083/saml/acs"

register-with-email:
	curl -s -X POST -H "Content-Type: application/json" http://localhost:8082/admin/service-providers \
		-d "$$(jq -n --slurpfile m mapping-email.json '{entity_id:"http://localhost:8083/saml/metadata", acs_url:"http://localhost:8083/saml/acs", attribute_mapping: $$m[0]}')"

register-with-persistent:
	curl -s -X POST -H "Content-Type: application/json" http://localhost:8082/admin/service-providers \
		-d "$$(jq -n --slurpfile m mapping-persistent.json '{entity_id:"http://localhost:8083/saml/metadata", acs_url:"http://localhost:8083/saml/acs", attribute_mapping: $$m[0]}')"

register-with-transient:
	curl -s -X POST -H "Content-Type: application/json" http://localhost:8082/admin/service-providers \
		-d "$$(jq -n --slurpfile m mapping-transient.json '{entity_id:"http://localhost:8083/saml/metadata", acs_url:"http://localhost:8083/saml/acs", attribute_mapping: $$m[0]}')"

certs:
	@mkdir -p etc/certs
	@if [ ! -f etc/certs/myservice.key ] || [ ! -f etc/certs/myservice.crt ]; then \
		echo "Generating service certificates..."; \
		openssl req -x509 -newkey rsa:2048 -keyout etc/certs/myservice.key -out etc/certs/myservice.crt \
			-days 365 -nodes \
			-subj "/C=US/ST=State/L=City/O=Organization/CN=localhost"; \
		echo "Certificates generated: etc/certs/myservice.crt and etc/certs/myservice.key"; \
	else \
		echo "Service certificates already exist"; \
	fi

clean:
	@rm -f bin/example-saml-service
