.PHONY: help build build-cli certs clean dev down provider-certs service-certs test run

help:
	@echo "SAML Provider Root Makefile"
	@echo ""
	@echo "Build & Clean:"
	@echo "  build              - Build provider and CLI tools"
	@echo "  build-cli          - Build service-provider-admin CLI only"
	@echo "  clean              - Clean all build artifacts and certificates"
	@echo "  certs              - Generate certificates for both provider and service"
	@echo ""
	@echo "Development:"
	@echo "  dev                - Generate certs and start Docker environment"
	@echo "  down               - Tear down the development environment"
	@echo "  help               - Show this help message"
	@echo "  run                - Run the provider locally"
	@echo "  test               - Run all tests"

build:
	go build -o bin/identity-saml-provider ./cmd/identity-saml-provider
	go build -o bin/service-provider-admin ./cmd/service-provider-admin

build-cli:
	go build -o bin/service-provider-admin ./cmd/service-provider-admin

test:
	go test -v ./...

certs:
	@mkdir -p .local/certs
	@if [ ! -f .local/certs/bridge.key ] || [ ! -f .local/certs/bridge.crt ]; then \
		echo "Generating provider certificates..."; \
		openssl req -x509 -newkey rsa:2048 -keyout .local/certs/bridge.key -out .local/certs/bridge.crt -days 365 -nodes \
			-subj "/CN=localhost"; \
		echo "Certificates generated: .local/certs/bridge.crt and .local/certs/bridge.key"; \
	else \
		echo "Provider certificates already exist"; \
	fi

run: certs
	go run ./cmd/identity-saml-provider

clean:
	rm -rf bin/
	rm -rf .local/

dev:
	@echo "Starting development environment..."
	docker compose -f docker-compose.dev.yml up -d --build --remove-orphans --force-recreate
	@echo "Development services are up and running"
	@echo "Next steps:"
	@echo "1. In one terminal, run the provider: \`make run\`"
	@echo "2. In another terminal, run the example service: \`make service-run\`"
	@echo "3. Visit the client application at http://localhost:8083/hello"

down:
	@echo "Tearing down development environment..."
	docker compose -f docker-compose.dev.yml down
	@echo "Development environment torn down"
