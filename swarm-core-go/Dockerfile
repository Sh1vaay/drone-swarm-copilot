# Stage 1: Build
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies & git if needed
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked, stripped production binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o swarm-server ./cmd/swarm-server

# Stage 2: Minimal Distroless / Alpine Runtime
FROM alpine:3.20

# Install runtime security certs
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup

WORKDIR /app

# Copy binary and static web assets with non-root ownership
COPY --from=builder --chown=appuser:appgroup /app/swarm-server .
COPY --from=builder --chown=appuser:appgroup /app/web ./web

# Switch to non-root user
USER appuser:appgroup

# Expose HTTP port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Run the executable
ENTRYPOINT ["./swarm-server"]
