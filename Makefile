.PHONY: help build build-cli certs k8s-certs k8s-copy-secrets k8s clean run docker down test test-integration migrate-up migrate-down migrate-status migrate-check

# VERSION is derived from git. `--dirty` is included when uncommitted changes exist
VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || echo "v0.0.0")

# LDFLAGS injects the version into the binary at build time. Use this in CI/builds.
LDFLAGS := -ldflags "-X github.com/canonical/identity-saml-provider/internal/version.Version=$(VERSION)"

# DB_PORT for local development (override with: make run DB_PORT=5432)
DB_PORT ?= 15432

# DSN for database migrations (override with: make migrate-up DSN="...")
DSN ?= "host=localhost port=$(DB_PORT) user=saml_provider password=saml_provider dbname=saml_provider sslmode=disable"

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
	@echo "  run                - Run the provider locally (migrate + serve)"
	@echo "  test               - Run all tests"
	@echo "  test-integration   - Run integration tests (requires Docker)"
	@echo ""
	@echo "Migrations:"
	@echo "  migrate-up         - Apply all pending migrations"
	@echo "  migrate-down       - Roll back the last migration"
	@echo "  migrate-status     - Show migration status"
	@echo "  migrate-check      - Check if there are pending migrations"

build:
	@echo "Building with version: $(VERSION)"
	go build $(LDFLAGS) -o bin/identity-saml-provider ./cmd/identity-saml-provider
	go build $(LDFLAGS) -o bin/service-provider-admin ./cmd/service-provider-admin

build-cli:
	@echo "Building CLI with version: $(VERSION)"
	go build $(LDFLAGS) -o bin/service-provider-admin ./cmd/service-provider-admin

test:
	go test -v ./...

test-integration:
	go test -v -tags=integration -count=1 ./...

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
	@echo "Running migrations..."
	go run $(LDFLAGS) ./cmd/identity-saml-provider migrate up --dsn $(DSN)
	@echo "Running with version: $(VERSION)"
	SAML_PROVIDER_DB_PORT=$(DB_PORT) go run $(LDFLAGS) ./cmd/identity-saml-provider serve

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
	@echo "2. In another terminal, register and run the example service: \`cd test/saml-service && make register && make run\`"
	@echo "3. Visit the client application at http://localhost:8083/hello"

down:
	@echo "Tearing down development environment..."
	docker compose -f docker-compose.dev.yml down
	@echo "Development environment torn down"

migrate-up:
	@go run $(LDFLAGS) ./cmd/identity-saml-provider migrate up --dsn $(DSN)

migrate-down:
	@go run $(LDFLAGS) ./cmd/identity-saml-provider migrate down --dsn $(DSN)

migrate-status:
	@go run $(LDFLAGS) ./cmd/identity-saml-provider migrate status --dsn $(DSN)

migrate-check:
	@go run $(LDFLAGS) ./cmd/identity-saml-provider migrate check --dsn $(DSN)
