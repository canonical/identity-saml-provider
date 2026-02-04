# Build stage
FROM golang:1.24-alpine AS builder

# Install dependencies
RUN apk add --no-cache git openssl

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o saml-provider .

# Generate certificates
RUN openssl req -x509 -newkey rsa:2048 -keyout bridge.key -out bridge.crt -days 365 -nodes \
    -subj "/CN=localhost"

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS connections
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary and certificates from builder
COPY --from=builder /app/saml-provider .
COPY --from=builder /app/bridge.key .
COPY --from=builder /app/bridge.crt .

# Set default environment variables
ENV HYDRA_PUBLIC_URL=http://localhost:4444

# Expose the service port
EXPOSE 8082

# Run the application
CMD ["./saml-provider"]
