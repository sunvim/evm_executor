# Multi-stage build for minimal image size
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o executor ./cmd/executor

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 executor && \
    adduser -D -u 1000 -G executor executor

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/executor .
COPY --from=builder /build/config/config.yaml config/

# Change ownership
RUN chown -R executor:executor /app

# Switch to non-root user
USER executor

# Expose metrics port
EXPOSE 9091

# Run the executor
ENTRYPOINT ["./executor"]
CMD ["-config", "config/config.yaml"]
