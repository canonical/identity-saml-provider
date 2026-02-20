.PHONY: help build build-cli certs k8s-certs k8s-copy-secrets k8s clean run docker down test

help:
	@echo "SAML Provider Root Makefile"
	@echo ""
	@echo "Build & Clean:"
	@echo "  build              - Build provider and CLI tools"
	@echo "  build-cli          - Build service-provider-admin CLI only"
	@echo "  clean              - Clean all build artifacts and certificates"
	@echo "  certs              - Generate certificates for both provider and service"
	@echo "  certs-link         - Link provider certs into k8s for kustomize"
	@echo "  kratos-secrets     - Generate k8s/secrets/kratos.env from KRATOS_OIDC_PROVIDER_CLIENT_*"
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

k8s-certs:
	@mkdir -p k8s/certs
	@if [ ! -f k8s/certs/bridge.key ] || [ ! -f k8s/certs/bridge.crt ]; then \
		echo "Generating provider certificates..."; \
		openssl req -x509 -newkey rsa:2048 -keyout k8s/certs/bridge.key -out k8s/certs/bridge.crt -days 365 -nodes \
			-subj "/CN=localhost"; \
		echo "Certificates generated: k8s/certs/bridge.crt and k8s/certs/bridge.key"; \
	else \
		echo "Provider certificates already exist"; \
	fi

k8s-copy-secrets:
	@mkdir -p k8s/secrets
	@set -a; \
	if [ -f .env ]; then . ./.env; fi; \
	set +a; \
	if [ -z "$$KRATOS_OIDC_PROVIDER_CLIENT_ID" ] || [ -z "$$KRATOS_OIDC_PROVIDER_CLIENT_SECRET" ]; then \
		echo "KRATOS_OIDC_PROVIDER_CLIENT_ID and KRATOS_OIDC_PROVIDER_CLIENT_SECRET must be set (env or .env)"; \
		exit 1; \
	fi; \
	printf "client-id=%s\nclient-secret=%s\n" "$$KRATOS_OIDC_PROVIDER_CLIENT_ID" "$$KRATOS_OIDC_PROVIDER_CLIENT_SECRET" > k8s/secrets/kratos.env; \
	echo "Generated k8s/secrets/kratos.env"

k8s: k8s-certs k8s-copy-secrets
	skaffold dev --default-repo=localhost:32000 --cache-artifacts=false

run: certs
	go run ./cmd/identity-saml-provider

clean:
	rm -rf bin/
	rm -rf .local/
	rm -rf k8s/certs/

docker:
	@echo "Starting docker development environment..."
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
