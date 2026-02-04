.PHONY: help build run test lint clean certs

help:
	@echo "Available targets:"
	@echo "  make build    - Build the application"
	@echo "  make run      - Run the application"
	@echo "  make test     - Run tests"
	@echo "  make lint     - Run linter"
	@echo "  make clean    - Clean build artifacts"
	@echo "  make certs    - Generate self-signed certificates"

build:
	go build -o ../bin/saml-provider .

run:
	go run main.go

test:
	go test -v -race ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/
	rm -f bridge.crt bridge.key

certs:
	openssl req -x509 -newkey rsa:2048 -keyout bridge.key -out bridge.crt -days 365 -nodes \
		-subj "/CN=localhost"
	@echo "Certificates generated: bridge.crt and bridge.key"

dev: certs run
