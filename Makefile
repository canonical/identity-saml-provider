.PHONY: help build certs clean dev down

# Default target
help:
	@echo "SAML Provider Root Makefile"
	@echo "Available targets:"
	@echo "  build            - Build both provider and service"
	@echo "  certs            - Generate certificates for both provider and service"
	@echo "  clean            - Clean all build artifacts and certificates"
	@echo "  dev              - Generate certs and run both provider and service"
	@echo "  down             - Tear down the development environment"
	@echo "  help             - Show this help message"

# Build targets
build:
	cd provider && $(MAKE) build
	cd service && $(MAKE) build
	@echo "Build complete"

# Certificate targets
certs:
	cd provider && $(MAKE) certs
	cd service && $(MAKE) certs
	@echo "All necessary certificates are generated"

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
	docker compose -f docker-compose.dev.yml up -d --build --remove-orphans
	@echo "Development services are up and running"
	@echo "Next steps:"
	@echo "1. In one terminal, run the provider (by going to provider/ and running \`make run\`)"
	@echo "2. In another terminal, run the service (by going to service/ and running \`make run\`)"
	@echo "3. Visit the client application at http://localhost:8083/hello"

down:
	@echo "Tearing down development environment..."
	docker compose -f docker-compose.dev.yml down
	@echo "Development environment torn down"
