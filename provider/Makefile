.PHONY: help build run debug test lint clean certs

help:
	@echo "Available targets:"
	@echo "  make build    - Build the application"
	@echo "  make run      - Run the application"
	@echo "  make debug    - Run the application with debugger"
	@echo "  make test     - Run tests"
	@echo "  make lint     - Run linter"
	@echo "  make clean    - Clean build artifacts"
	@echo "  make certs    - Generate self-signed certificates"

build:
	go build -o ../bin/saml-provider .

run:
	go run .

debug:
	dlv debug .

test:
	go test -v -race ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/
	rm -f bridge.crt bridge.key

certs:
	@if [ ! -f bridge.key ] || [ ! -f bridge.crt ]; then \
		echo "Generating self-signed certificates..."; \
		openssl req -x509 -newkey rsa:2048 -keyout bridge.key -out bridge.crt -days 365 -nodes \
			-subj "/CN=localhost"; \
		echo "Certificates generated: bridge.crt and bridge.key"; \
	else \
		echo "Certificates already exist"; \
	fi

dev: certs run
