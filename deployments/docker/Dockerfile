# Stage 1: Build frontend
FROM node:25-alpine AS frontend-builder

WORKDIR /app/web

COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1.24.11-alpine AS backend-builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X github.com/ned1313/terraform-mirror/internal/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/ned1313/terraform-mirror/internal/version.GitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
    -o terraform-mirror \
    ./cmd/terraform-mirror

# Stage 3: Final runtime image
FROM alpine:3.23

# Add ca-certificates for HTTPS and create non-root user
RUN apk --no-cache add ca-certificates && \
    adduser -D -u 1000 appuser && \
    mkdir -p /data && \
    chown -R appuser:appuser /data

WORKDIR /app

# Copy binary from builder
COPY --from=backend-builder /app/terraform-mirror /app/terraform-mirror

# Copy frontend build from frontend builder
COPY --from=frontend-builder /app/web/dist /app/web/dist

USER appuser

# Expose port
EXPOSE 8080

# Health check using built-in command
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/terraform-mirror", "healthcheck"]

ENTRYPOINT ["/app/terraform-mirror"]
