.PHONY: help build run certs clean

help:
	@echo "Example SAML Service Makefile"
	@echo ""
	@echo "Build & Run:"
	@echo "  build              - Build the example service"
	@echo "  run                - Run the example service"
	@echo ""
	@echo "Certificates:"
	@echo "  certs              - Generate certificates for the service"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean              - Remove build artifacts"

build:
	go build -o bin/example-saml-service

run: certs
	go run . --cert etc/certs/myservice.crt --key etc/certs/myservice.key

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
