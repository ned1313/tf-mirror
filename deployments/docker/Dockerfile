# Stage 1: Build frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/web

# Copy package files first for better layer caching
COPY web/package*.json ./
RUN npm ci --prefer-offline

COPY web/ ./
RUN npm run build

# Stage 2: Build backend
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS backend-builder

# Build arguments for cross-compilation
ARG TARGETOS
ARG TARGETARCH
ARG BUILD_TIME
ARG GIT_COMMIT

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies - this layer is cached unless go.mod/go.sum change
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy only Go source files needed for build (excluding tests, docs, etc.)
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build the binary with build cache mount
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build \
    -ldflags="-w -s -X github.com/ned1313/terraform-mirror/internal/version.BuildTime=${BUILD_TIME:-unknown} -X github.com/ned1313/terraform-mirror/internal/version.GitCommit=${GIT_COMMIT:-unknown}" \
    -o terraform-mirror \
    ./cmd/terraform-mirror

# Stage 3: Final runtime image
FROM alpine:3.23 AS runtime

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
