# Stage 1: Build frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/web

COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1.23-alpine AS backend-builder

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
    -ldflags="-w -s -X github.com/yourusername/terraform-mirror/internal/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/yourusername/terraform-mirror/internal/version.GitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
    -o terraform-mirror \
    ./cmd/terraform-mirror

# Stage 3: Final runtime image
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy binary from builder
COPY --from=backend-builder /app/terraform-mirror /app/terraform-mirror

# Copy frontend build from frontend builder
COPY --from=frontend-builder /app/web/dist /app/web/dist

# Create directories for data
USER nonroot:nonroot

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/terraform-mirror", "healthcheck"]

ENTRYPOINT ["/app/terraform-mirror"]
