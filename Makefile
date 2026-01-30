.PHONY: help build build-provider build-service run run-provider run-service certs certs-provider certs-service clean dev

# Default target
help:
	@echo "SAML Provider Root Makefile"
	@echo "Available targets:"
	@echo "  build            - Build both provider and service"
	@echo "  build-provider   - Build only the provider"
	@echo "  build-service    - Build only the service"
	@echo "  run              - Build and run both provider and service"
	@echo "  run-provider     - Build and run only the provider"
	@echo "  run-service      - Build and run only the service"
	@echo "  certs            - Generate certificates for both provider and service"
	@echo "  certs-provider   - Generate certificates for the provider"
	@echo "  certs-service    - Generate certificates for the service"
	@echo "  clean            - Clean all build artifacts and certificates"
	@echo "  dev              - Generate certs and run both provider and service"
	@echo "  help             - Show this help message"

# Build targets
build: build-provider build-service
	@echo "Successfully built provider and service"

build-provider:
	@echo "Building provider..."
	cd provider && $(MAKE) build

build-service:
	@echo "Building service..."
	cd service && $(MAKE) build

# Run targets
run: certs run-provider run-service

run-provider: certs-provider build-provider
	@echo "Running provider..."
	cd provider && $(MAKE) run

run-service: certs-service build-service
	@echo "Running service..."
	cd service && $(MAKE) run

# Certificate targets
certs: certs-provider certs-service

certs-provider:
	@echo "Generating provider certificates..."
	cd provider && $(MAKE) certs

certs-service:
	@echo "Generating service certificates..."
	cd service && $(MAKE) certs

# Clean target
clean:
	@echo "Cleaning provider..."
	cd provider && $(MAKE) clean
	@echo "Cleaning service..."
	cd service && $(MAKE) clean
	rm -rf bin/
	@echo "Cleanup complete"

# Development target - generates certs and runs both services
dev: certs
	@echo "Starting development environment..."
	@echo "Provider will run on http://localhost:8082"
	@echo "Service will run on http://localhost:8083"
	@echo ""
	@echo "Starting provider in background..."
	cd provider && go run main.go &
	@echo "Starting service..."
	cd service && go run main.go
