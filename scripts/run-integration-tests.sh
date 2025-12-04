#!/bin/bash
# Run S3 integration tests with MinIO
# Usage: ./scripts/run-integration-tests.sh [--keep-running] [--package PACKAGE]

set -e

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/deployments/docker-compose/docker-compose.test.yml"
KEEP_RUNNING=false
PACKAGE="all"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --keep-running)
            KEEP_RUNNING=true
            shift
            ;;
        --package)
            PACKAGE="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

log_status() {
    echo -e "\n${CYAN}[$(date '+%H:%M:%S')] $1${NC}"
}

log_success() {
    echo -e "${GREEN}$1${NC}"
}

log_error() {
    echo -e "${RED}$1${NC}"
}

log_warning() {
    echo -e "${YELLOW}$1${NC}"
}

check_minio_ready() {
    curl -sf http://localhost:9000/minio/health/live > /dev/null 2>&1
}

start_minio() {
    log_status "Starting MinIO..."
    docker-compose -f "$COMPOSE_FILE" up -d minio
    
    log_status "Waiting for MinIO to be healthy..."
    local max_attempts=30
    local attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if check_minio_ready; then
            log_success "MinIO is ready!"
            
            # Run the init container to create buckets
            log_status "Creating test buckets..."
            docker-compose -f "$COMPOSE_FILE" up minio-init
            sleep 2
            return 0
        fi
        
        echo -n "."
        attempt=$((attempt + 1))
        sleep 1
    done
    
    log_error "MinIO failed to start within timeout"
    return 1
}

stop_minio() {
    log_status "Stopping MinIO..."
    docker-compose -f "$COMPOSE_FILE" down -v
    log_success "MinIO stopped and volumes cleaned"
}

run_integration_tests() {
    log_status "Running integration tests..."
    
    local test_path
    if [ "$PACKAGE" = "all" ]; then
        test_path="./..."
    else
        test_path="./internal/$PACKAGE/..."
    fi
    
    export INTEGRATION_TEST=true
    
    if go test -v -tags=integration -timeout=5m "$test_path"; then
        return 0
    else
        return 1
    fi
}

cleanup() {
    if [ "$KEEP_RUNNING" = false ]; then
        stop_minio
    fi
}

# Trap to ensure cleanup on exit
trap cleanup EXIT

# Main execution
echo ""
echo "========================================"
echo "  Terraform Mirror Integration Tests"
echo "========================================"

# Check if MinIO is already running
if check_minio_ready; then
    log_success "MinIO is already running"
else
    if ! start_minio; then
        exit 1
    fi
fi

# Run tests
if run_integration_tests; then
    log_success "\nAll integration tests passed!"
    exit_code=0
else
    log_error "\nSome integration tests failed"
    exit_code=1
fi

if [ "$KEEP_RUNNING" = true ]; then
    log_warning "MinIO left running. Stop with: docker-compose -f $COMPOSE_FILE down -v"
fi

exit $exit_code
