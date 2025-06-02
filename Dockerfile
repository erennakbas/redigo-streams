# Multi-stage build for Go application
FROM golang:1.21-alpine AS builder

# Install git and other dependencies
RUN apk add --no-cache git protobuf protobuf-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Generate protobuf files
RUN protoc --go_out=. --go_opt=paths=source_relative pkg/proto/message.proto

# Build the application
RUN go build -o /app/strego ./cmd/...

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder stage
COPY --from=builder /app/redigo-streams .
COPY --from=builder /app/examples/ ./examples/

# Expose port (if needed)
EXPOSE 8080

# Default command
CMD ["./redigo-streams"] 