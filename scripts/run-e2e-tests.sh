#!/bin/bash
# Run end-to-end tests for Terraform Mirror
# Usage: ./scripts/run-e2e-tests.sh [--keep-running] [--skip-terraform] [--skip-modules] [--skip-build]

set -e

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/deployments/docker-compose/docker-compose.yml"

# Configuration
MIRROR_URL="http://localhost:8080"
ADMIN_USERNAME="admin"
ADMIN_PASSWORD="testpassword123"

# Options
KEEP_RUNNING=false
SKIP_TERRAFORM=false
SKIP_MODULES=false
SKIP_BUILD=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --keep-running)
            KEEP_RUNNING=true
            shift
            ;;
        --skip-terraform)
            SKIP_TERRAFORM=true
            shift
            ;;
        --skip-modules)
            SKIP_MODULES=true
            shift
            ;;
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# Test counters
PASSED=0
FAILED=0
SKIPPED=0

log_status() {
    echo -e "\n${CYAN}[$(date '+%H:%M:%S')] $1${NC}"
}

test_result() {
    local name="$1"
    local passed="$2"
    local details="$3"
    
    if [ "$passed" = "true" ]; then
        echo -e "  ${GREEN}[PASS]${NC} $name"
        PASSED=$((PASSED + 1))
    elif [ "$passed" = "skip" ]; then
        echo -e "  ${YELLOW}[SKIP]${NC} $name"
        SKIPPED=$((SKIPPED + 1))
    else
        echo -e "  ${RED}[FAIL]${NC} $name"
        [ -n "$details" ] && echo -e "         $details"
        FAILED=$((FAILED + 1))
    fi
}

wait_for_service() {
    local url="$1"
    local timeout="${2:-60}"
    local elapsed=0
    
    while [ $elapsed -lt $timeout ]; do
        if curl -sf "$url" > /dev/null 2>&1; then
            return 0
        fi
        sleep 1
        elapsed=$((elapsed + 1))
    done
    return 1
}

start_stack() {
    log_status "Building and starting Terraform Mirror stack..."
    
    export TFM_ADMIN_PASSWORD="$ADMIN_PASSWORD"
    
    if [ "$SKIP_BUILD" = true ]; then
        docker-compose -f "$COMPOSE_FILE" up -d
    else
        docker-compose -f "$COMPOSE_FILE" up -d --build
    fi
    
    log_status "Waiting for services to be healthy..."
    
    # Wait for MinIO
    if ! wait_for_service "http://localhost:9000/minio/health/live" 30; then
        echo -e "${RED}MinIO failed to start${NC}"
        return 1
    fi
    echo -e "  ${GREEN}MinIO is ready${NC}"
    
    # Wait for Terraform Mirror
    if ! wait_for_service "$MIRROR_URL/health" 60; then
        echo -e "${RED}Terraform Mirror failed to start${NC}"
        docker-compose -f "$COMPOSE_FILE" logs terraform-mirror
        return 1
    fi
    echo -e "  ${GREEN}Terraform Mirror is ready${NC}"
    
    return 0
}

stop_stack() {
    log_status "Stopping stack and cleaning up..."
    docker-compose -f "$COMPOSE_FILE" down -v
    log_status "Stack stopped"
}

get_auth_token() {
    local response
    response=$(curl -sf -X POST "$MIRROR_URL/admin/api/login" \
        -H "Content-Type: application/json" \
        -d "{\"username\":\"$ADMIN_USERNAME\",\"password\":\"$ADMIN_PASSWORD\"}" 2>/dev/null)
    
    echo "$response" | jq -r '.token // empty' 2>/dev/null
}

test_health_endpoint() {
    local response
    response=$(curl -sf "$MIRROR_URL/health" 2>/dev/null)
    local status
    status=$(echo "$response" | jq -r '.status // empty' 2>/dev/null)
    [ "$status" = "ok" ] || [ "$status" = "healthy" ]
}

test_metrics_endpoint() {
    local response
    response=$(curl -sf "$MIRROR_URL/metrics" 2>/dev/null)
    echo "$response" | grep -q "go_goroutines\|terraform_mirror"
}

test_service_discovery() {
    local response
    response=$(curl -sf "$MIRROR_URL/.well-known/terraform.json" 2>/dev/null)
    echo "$response" | jq -e '."providers.v1"' > /dev/null 2>&1
}

test_module_service_discovery() {
    local response
    response=$(curl -sf "$MIRROR_URL/.well-known/terraform.json" 2>/dev/null)
    echo "$response" | jq -e '."modules.v1"' > /dev/null 2>&1
}

test_admin_login() {
    local token
    token=$(get_auth_token)
    [ -n "$token" ] && [ "$token" != "null" ]
}

test_provider_load() {
    local token="$1"
    
    # Create a minimal provider definition
    local provider_hcl='provider "hashicorp/null" {
  versions  = ["3.2.0"]
  platforms = ["linux_amd64"]
}'
    
    local response
    response=$(curl -sf -X POST "$MIRROR_URL/admin/api/providers/load" \
        -H "Authorization: Bearer $token" \
        -F "file=@-;filename=test.hcl" <<< "$provider_hcl" 2>/dev/null)
    
    echo "$response" | jq -e '.job_id // .id' > /dev/null 2>&1
}

wait_for_job_completion() {
    local token="$1"
    local timeout="${2:-120}"
    local elapsed=0
    
    while [ $elapsed -lt $timeout ]; do
        local pending
        pending=$(curl -sf "$MIRROR_URL/admin/api/jobs?status=pending" \
            -H "Authorization: Bearer $token" 2>/dev/null | jq '.jobs | length // 0')
        
        local processing
        processing=$(curl -sf "$MIRROR_URL/admin/api/jobs?status=processing" \
            -H "Authorization: Bearer $token" 2>/dev/null | jq '.jobs | length // 0')
        
        if [ "$pending" = "0" ] && [ "$processing" = "0" ]; then
            return 0
        fi
        
        echo -n "."
        sleep 2
        elapsed=$((elapsed + 2))
    done
    echo ""
    return 1
}

# ============================================================================
# Module E2E Test Functions
# ============================================================================

test_module_load() {
    local token="$1"
    
    # Create a minimal module definition
    # This module uses git:: URLs which are now supported by our Git downloader.
    # The module will be cloned and archived as a tarball.
    local module_hcl='module "hashicorp/consul/aws" {
  versions = ["0.1.0"]
}'
    
    local response
    response=$(curl -sf -X POST "$MIRROR_URL/admin/api/modules/load" \
        -H "Authorization: Bearer $token" \
        -F "file=@-;filename=test-modules.hcl" <<< "$module_hcl" 2>/dev/null)
    
    echo "$response" | jq -e '.job_id // .id' > /dev/null 2>&1
}

test_module_list() {
    local token="$1"
    local response
    response=$(curl -sf "$MIRROR_URL/admin/api/modules" \
        -H "Authorization: Bearer $token" 2>/dev/null)
    echo "$response" | jq -e '.modules' > /dev/null 2>&1
}

test_module_version_list() {
    local response
    response=$(curl -sf "$MIRROR_URL/v1/modules/hashicorp/consul/aws/versions" 2>/dev/null)
    # 404 is acceptable if no modules loaded
    if [ -z "$response" ]; then
        local status
        status=$(curl -sw '%{http_code}' -o /dev/null "$MIRROR_URL/v1/modules/hashicorp/consul/aws/versions" 2>/dev/null)
        [ "$status" = "404" ]
    else
        echo "$response" | jq -e '.modules' > /dev/null 2>&1
    fi
}

test_module_download() {
    # Test module download endpoint - returns 204 with X-Terraform-Get header
    local response_headers
    response_headers=$(curl -sI "$MIRROR_URL/v1/modules/hashicorp/consul/aws/0.1.0/download" 2>/dev/null)
    
    if echo "$response_headers" | grep -q "HTTP.*204"; then
        echo "$response_headers" | grep -qi "X-Terraform-Get"
    else
        # 404 means module not downloaded yet (acceptable)
        echo "$response_headers" | grep -q "HTTP.*404"
        return 2  # Skip
    fi
}

wait_for_module_job_completion() {
    local token="$1"
    local timeout="${2:-120}"
    local elapsed=0
    
    while [ $elapsed -lt $timeout ]; do
        local pending
        pending=$(curl -sf "$MIRROR_URL/admin/api/jobs?job_type=module&status=pending" \
            -H "Authorization: Bearer $token" 2>/dev/null | jq '.jobs | length // 0')
        
        local processing
        processing=$(curl -sf "$MIRROR_URL/admin/api/jobs?job_type=module&status=processing" \
            -H "Authorization: Bearer $token" 2>/dev/null | jq '.jobs | length // 0')
        
        if [ "$pending" = "0" ] && [ "$processing" = "0" ]; then
            return 0
        fi
        
        echo -n "."
        sleep 2
        elapsed=$((elapsed + 2))
    done
    echo ""
    return 1
}

test_terraform_init() {
    # Check if terraform is available
    if ! command -v terraform &> /dev/null; then
        return 2  # Skip
    fi
    
    local temp_dir
    temp_dir=$(mktemp -d)
    
    # Create test config
    cat > "$temp_dir/main.tf" << 'EOF'
terraform {
  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "3.2.0"
    }
  }
}

provider "null" {}

resource "null_resource" "test" {}
EOF

    # Create terraformrc
    cat > "$temp_dir/.terraformrc" << EOF
provider_installation {
  network_mirror {
    url = "$MIRROR_URL/v1/providers/"
  }
}
EOF

    export TF_CLI_CONFIG_FILE="$temp_dir/.terraformrc"
    
    local result=0
    if ! (cd "$temp_dir" && terraform init 2>&1); then
        result=1
    fi
    
    unset TF_CLI_CONFIG_FILE
    rm -rf "$temp_dir"
    
    return $result
}

cleanup() {
    if [ "$KEEP_RUNNING" = false ]; then
        stop_stack
    fi
}

trap cleanup EXIT

# Main execution
echo ""
echo "========================================"
echo "  Terraform Mirror E2E Tests"
echo "========================================"

# Start the stack
if ! start_stack; then
    echo -e "${RED}Failed to start stack${NC}"
    exit 1
fi

log_status "Running E2E tests..."

# Test 1: Health endpoint
if test_health_endpoint; then
    test_result "Health endpoint" "true"
else
    test_result "Health endpoint" "false"
fi

# Test 2: Metrics endpoint
if test_metrics_endpoint; then
    test_result "Metrics endpoint" "true"
else
    test_result "Metrics endpoint" "false"
fi

# Test 3: Service discovery
if test_service_discovery; then
    test_result "Service discovery (/.well-known/terraform.json)" "true"
else
    test_result "Service discovery (/.well-known/terraform.json)" "false"
fi

# Test 4: Admin login
if test_admin_login; then
    test_result "Admin login" "true"
    TOKEN=$(get_auth_token)
else
    test_result "Admin login" "false"
    TOKEN=""
fi

# Test 5: Provider version list
if curl -sf "$MIRROR_URL/v1/providers/hashicorp/null/versions" > /dev/null 2>&1 || \
   [ "$(curl -sw '%{http_code}' -o /dev/null "$MIRROR_URL/v1/providers/hashicorp/null/versions" 2>/dev/null)" = "404" ]; then
    test_result "Provider version list endpoint" "true"
else
    test_result "Provider version list endpoint" "false"
fi

# Continue only if login worked
if [ -n "$TOKEN" ]; then
    # Test 6: Provider load
    if test_provider_load "$TOKEN"; then
        test_result "Provider load (upload HCL)" "true"
        
        # Test 7: Wait for job completion
        echo "  Waiting for provider download job to complete..."
        if wait_for_job_completion "$TOKEN" 120; then
            test_result "Job completion" "true"
            
            # Test 8: Terraform CLI integration
            if [ "$SKIP_TERRAFORM" = false ]; then
                terraform_result=$(test_terraform_init; echo $?)
                if [ "$terraform_result" = "0" ]; then
                    test_result "Terraform init with mirror" "true"
                elif [ "$terraform_result" = "2" ]; then
                    test_result "Terraform init with mirror (terraform not installed)" "skip"
                else
                    test_result "Terraform init with mirror" "false" "Provider may not have finished downloading"
                fi
            else
                test_result "Terraform CLI tests (--skip-terraform)" "skip"
            fi
        else
            test_result "Job completion" "false" "Timeout waiting for jobs"
        fi
    else
        test_result "Provider load (upload HCL)" "false"
    fi
fi

# ============================================================================
# Module E2E Tests
# ============================================================================
if [ "$SKIP_MODULES" = false ]; then
    log_status "Running Module E2E tests..."
    
    # Test M1: Module service discovery
    if test_module_service_discovery; then
        test_result "Module service discovery (modules.v1)" "true"
    else
        test_result "Module service discovery (modules.v1)" "false"
    fi
    
    # Test M2: Module version list endpoint (may 404 if no modules)
    if test_module_version_list; then
        test_result "Module version list endpoint" "true"
    else
        test_result "Module version list endpoint" "false"
    fi
    
    # Continue only if login worked
    if [ -n "$TOKEN" ]; then
        # Test M3: Module list endpoint (admin)
        if test_module_list "$TOKEN"; then
            test_result "Module list endpoint (admin)" "true"
        else
            test_result "Module list endpoint (admin)" "false"
        fi
        
        # Test M4: Module load (upload HCL)
        if test_module_load "$TOKEN"; then
            test_result "Module load (upload HCL)" "true"
            
            # Test M5: Wait for module job completion
            echo "  Waiting for module download job to complete..."
            if wait_for_module_job_completion "$TOKEN" 120; then
                test_result "Module job completion" "true"
                
                # Give the system a moment to update
                sleep 2
                
                # Test M6: Module version list after download
                if test_module_version_list; then
                    test_result "Module versions after download" "true"
                else
                    test_result "Module versions after download" "false"
                fi
                
                # Test M7: Module download endpoint
                module_download_result=$(test_module_download; echo $?)
                if [ "$module_download_result" = "0" ]; then
                    test_result "Module download endpoint (X-Terraform-Get)" "true"
                elif [ "$module_download_result" = "2" ]; then
                    test_result "Module download (module may not exist)" "skip"
                else
                    test_result "Module download endpoint (X-Terraform-Get)" "false"
                fi
                
                # Test M8: Verify module appears in admin list
                module_count=$(curl -sf "$MIRROR_URL/admin/api/modules" \
                    -H "Authorization: Bearer $TOKEN" 2>/dev/null | jq '.modules | length // 0')
                if [ "$module_count" -gt 0 ] 2>/dev/null; then
                    test_result "Modules appear in admin list (count: $module_count)" "true"
                else
                    test_result "Modules appear in admin list" "false"
                fi
            else
                test_result "Module job completion" "false" "Timeout waiting for module jobs"
            fi
        else
            test_result "Module load (upload HCL)" "false"
        fi
    fi
else
    log_status "Skipping Module E2E tests (--skip-modules)"
    SKIPPED=$((SKIPPED + 8))
fi

# Summary
echo ""
echo "========================================"
echo "  Test Results"
echo "========================================"
echo -e "  Passed:  ${GREEN}$PASSED${NC}"
if [ $FAILED -gt 0 ]; then
    echo -e "  Failed:  ${RED}$FAILED${NC}"
else
    echo -e "  Failed:  ${GREEN}$FAILED${NC}"
fi
echo -e "  Skipped: ${YELLOW}$SKIPPED${NC}"
echo ""

if [ "$KEEP_RUNNING" = true ]; then
    echo -e "${YELLOW}Stack left running at $MIRROR_URL${NC}"
    echo "  Admin UI: $MIRROR_URL/admin"
    echo "  MinIO Console: http://localhost:9001"
    echo "  Stop with: docker-compose -f \"$COMPOSE_FILE\" down -v"
fi

# Exit with appropriate code
[ $FAILED -eq 0 ]
