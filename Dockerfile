# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build bridge binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o bridge ./cmd/bridge

# Build encrypt-value tool
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o encrypt-value ./cmd/encrypt-value

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /build/bridge .
COPY --from=builder /build/encrypt-value .

# Copy default config
COPY --from=builder /build/config.example.yaml ./config.example.yaml

# Create directories
RUN mkdir -p /app/config /app/certs

# Expose HTTP port for metrics and health checks
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
  CMD wget --spider -q http://localhost:8080/health || exit 1

# Run bridge
CMD ["./bridge", "-config", "/app/config/config.yaml"]


