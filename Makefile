.PHONY: help build certs clean dev down provider-certs service-certs

help:
	@echo "SAML Provider Root Makefile"
	@echo ""
	@echo "Build & Clean:"
	@echo "  build              - Build both provider and service"
	@echo "  clean              - Clean all build artifacts and certificates"
	@echo "  certs              - Generate certificates for both provider and service"
	@echo ""
	@echo "Development:"
	@echo "  dev                - Generate certs and start Docker environment"
	@echo "  down               - Tear down the development environment"
	@echo "  help               - Show this help message"

build:
	go build -o bin/identity-saml-provider .

certs:
	@mkdir -p etc/certs
	@if [ ! -f etc/certs/bridge.key ] || [ ! -f etc/certs/bridge.crt ]; then \
		echo "Generating provider certificates..."; \
		openssl req -x509 -newkey rsa:2048 -keyout etc/certs/bridge.key -out etc/certs/bridge.crt -days 365 -nodes \
			-subj "/CN=localhost"; \
		echo "Certificates generated: etc/certs/bridge.crt and etc/certs/bridge.key"; \
	else \
		echo "Provider certificates already exist"; \
	fi

run: certs
	go run .

clean:
	rm -rf bin/
	rm -rf etc/certs/

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
