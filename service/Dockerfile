# Build stage
FROM golang:1.25-alpine AS builder

# Install dependencies
RUN apk add --no-cache git openssl

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o saml-service main.go

# Generate certificates
RUN openssl req -x509 -newkey rsa:2048 -keyout myservice.key -out myservice.crt -days 365 -nodes \
    -subj "/C=US/ST=State/L=City/O=Organization/CN=localhost"

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS connections
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary and certificates from builder
COPY --from=builder /app/saml-service .
COPY --from=builder /app/myservice.key .
COPY --from=builder /app/myservice.crt .

# Expose the service port
EXPOSE 8083

# Run the application
CMD ["./saml-service"]
