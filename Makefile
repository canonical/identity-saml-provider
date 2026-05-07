# SAML Provider – Root Makefile

.PHONY: \
	help \
	build clean \
	certs \
	dev docker dev-down \
	example-sp-register example-sp-run example-sp-clean \
	fmt lint \
	generate \
	k8s k8s-certs k8s-secrets \
	migrate-up migrate-down migrate-status migrate-check \
	run \
	test

# Variables

# VERSION is derived from git. `--dirty` is included when uncommitted changes exist
VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || echo "v0.0.0")

# LDFLAGS injects the version into the binary at build time. Use this in CI/builds.
LDFLAGS := -ldflags "-X github.com/canonical/identity-saml-provider/internal/version.Version=$(VERSION)"

# DB_PORT for local development (override with: make run DB_PORT=5432)
DB_PORT ?= 15432

# DSN for database migrations (override with: make migrate-up DSN="...")
DSN ?= "host=localhost port=$(DB_PORT) user=saml_provider password=saml_provider dbname=saml_provider sslmode=disable"

# Macros

# gen-certs generates a self-signed certificate in the given directory.
# Usage: $(call gen-certs,<output-dir>,<key-name>,<cert-name>,<subject>)
define gen-certs
	@mkdir -p $(1)
	@if [ ! -f $(1)/$(2) ] || [ ! -f $(1)/$(3) ]; then \
		echo "Generating certificates in $(1)..."; \
		openssl req -x509 -newkey rsa:2048 -keyout $(1)/$(2) -out $(1)/$(3) -days 365 -nodes \
			-subj "$(4)"; \
		echo "Certificates generated: $(1)/$(3) and $(1)/$(2)"; \
	else \
		echo "Certificates already exist in $(1)"; \
	fi
endef

# Help

help:
	@echo "SAML Provider Root Makefile"
	@echo ""
	@echo "Build & Clean:"
	@echo "  build                  - Build the identity-saml-provider binary"
	@echo "  clean                  - Clean all build artifacts and certificates"
	@echo "  certs                  - Generate certificates for the provider"
	@echo ""
	@echo "Development:"
	@echo "  dev                    - Generate certs and start Docker environment"
	@echo "  dev-down               - Tear down the development environment"
	@echo "  fmt                    - Format Go source files"
	@echo "  generate               - Run go generate for all packages (e.g. mockgen)"
	@echo "  help                   - Show this help message"
	@echo "  lint                   - Run golangci-lint (install: https://golangci-lint.run)"
	@echo "  run                    - Run the provider locally (migrate + serve)"
	@echo "  test                   - Run all tests (verbose: make test V=1)"
	@echo ""
	@echo "Example SP (delegates to test/example-sp):"
	@echo "  example-sp-register    - Register the example SP with the provider"
	@echo "  example-sp-run         - Run the example SAML service provider"
	@echo "  example-sp-clean       - Clean the example SP build artifacts"
	@echo ""
	@echo "Kubernetes:"
	@echo "  k8s                    - Start skaffold dev with k8s certs and secrets"
	@echo "  k8s-certs              - Generate certificates for k8s deployment"
	@echo "  k8s-secrets            - Generate k8s/secrets/kratos.env from KRATOS_OIDC_PROVIDER_CLIENT_*"
	@echo ""
	@echo "Migrations:"
	@echo "  migrate-up             - Apply all pending migrations"
	@echo "  migrate-down           - Roll back the last migration"
	@echo "  migrate-status         - Show migration status"
	@echo "  migrate-check          - Check if there are pending migrations"

# Build & Clean

build:
	@echo "Building with version: $(VERSION)"
	go build $(LDFLAGS) -o bin/identity-saml-provider ./cmd/identity-saml-provider

clean:
	rm -rf bin/
	rm -rf .local/
	rm -rf k8s/certs/
	rm -rf k8s/secrets/

certs:
	$(call gen-certs,.local/certs,bridge.key,bridge.crt,/CN=localhost)

# Development

dev: certs docker

docker:
	@echo "Starting docker development environment..."
	docker compose -f docker-compose.dev.yml up -d --build --remove-orphans --force-recreate
	@echo "Development services are up and running"
	@echo "Next steps:"
	@echo "1. In one terminal, run the provider: \`make run\`"
	@echo "2. In another terminal, register and run the example SP:"
	@echo "     make example-sp-register && make example-sp-run"
	@echo "3. Visit the client application at http://localhost:8083/hello"

dev-down:
	@echo "Tearing down development environment..."
	docker compose -f docker-compose.dev.yml down
	@echo "Development environment torn down"

run: certs
	@echo "Running migrations..."
	go run $(LDFLAGS) ./cmd/identity-saml-provider migrate up --dsn $(DSN)
	@echo "Running with version: $(VERSION)"
	SAML_PROVIDER_DB_PORT=$(DB_PORT) go run $(LDFLAGS) ./cmd/identity-saml-provider serve

# Code Quality

fmt:
	go fmt ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Error: golangci-lint not found. Install: https://golangci-lint.run/welcome/install/"; exit 1; }
	golangci-lint run ./...

test:
	go test $(if $(V),-v) ./...

generate:
	go generate ./...

# Example SP (delegates to test/example-sp)

example-sp-register:
	$(MAKE) -C test/example-sp register

example-sp-run:
	$(MAKE) -C test/example-sp run

example-sp-clean:
	$(MAKE) -C test/example-sp clean

# Kubernetes

k8s-certs:
	$(call gen-certs,k8s/certs,bridge.key,bridge.crt,/CN=localhost)

k8s-secrets:
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

k8s: k8s-certs k8s-secrets
	skaffold dev --default-repo=localhost:32000 --cache-artifacts=false

# Migrations

migrate-up:
	@go run $(LDFLAGS) ./cmd/identity-saml-provider migrate up --dsn $(DSN)

migrate-down:
	@go run $(LDFLAGS) ./cmd/identity-saml-provider migrate down --dsn $(DSN)

migrate-status:
	@go run $(LDFLAGS) ./cmd/identity-saml-provider migrate status --dsn $(DSN)

migrate-check:
	@go run $(LDFLAGS) ./cmd/identity-saml-provider migrate check --dsn $(DSN)
