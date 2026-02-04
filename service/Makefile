.PHONY: build run debug certs clean help

# Default target
help:
	@echo "SAML Service Makefile"
	@echo "Available targets:"
	@echo "  build     - Build the SAML service binary"
	@echo "  run       - Build and run the SAML service"
	@echo "  debug     - Run the SAML service with debugger"
	@echo "  certs     - Generate self-signed certificates for the service"
	@echo "  clean     - Remove generated binaries and certificates"
	@echo "  help      - Show this help message"

build:
	go build -o ../bin/saml-service

run:
	go run .

debug:
	dlv debug .

# Generate self-signed certificates
# Creates myservice.crt and myservice.key valid for 365 days
certs:
	@if [ ! -f myservice.key ] || [ ! -f myservice.crt ]; then \
		echo "Generating self-signed certificates..."; \
		openssl req -x509 -newkey rsa:2048 -keyout myservice.key -out myservice.crt \
			-days 365 -nodes \
			-subj "/C=US/ST=State/L=City/O=Organization/CN=localhost"; \
		echo "Certificates generated: myservice.crt and myservice.key"; \
	else \
		echo "Certificates already exist"; \
	fi

# Clean build artifacts and certificates
clean:
	rm -f saml-service myservice.crt myservice.key
