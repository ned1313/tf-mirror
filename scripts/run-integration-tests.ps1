<#
.SYNOPSIS
    Run S3 integration tests with MinIO
.DESCRIPTION
    Starts MinIO in Docker, runs S3 integration tests, then cleans up.
.EXAMPLE
    .\scripts\run-integration-tests.ps1
    .\scripts\run-integration-tests.ps1 -KeepRunning
    .\scripts\run-integration-tests.ps1 -Package storage
#>
param(
    [switch]$KeepRunning,
    [string]$Package = "all"
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
if (-not $ProjectRoot) {
    $ProjectRoot = (Get-Location).Path
}

$ComposeFile = Join-Path $ProjectRoot "deployments\docker-compose\docker-compose.test.yml"

function Write-Status {
    param([string]$Message, [string]$Color = "Cyan")
    Write-Host "`n[$([DateTime]::Now.ToString('HH:mm:ss'))] $Message" -ForegroundColor $Color
}

function Test-MinIOReady {
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:9000/minio/health/live" -UseBasicParsing -TimeoutSec 2 -ErrorAction SilentlyContinue
        return $response.StatusCode -eq 200
    } catch {
        return $false
    }
}

function Start-MinIO {
    Write-Status "Starting MinIO..." "Yellow"
    
    # Start MinIO
    docker-compose -f $ComposeFile up -d minio
    
    # Wait for MinIO to be ready
    Write-Status "Waiting for MinIO to be healthy..."
    $maxAttempts = 30
    $attempt = 0
    while ($attempt -lt $maxAttempts) {
        if (Test-MinIOReady) {
            Write-Status "MinIO is ready!" "Green"
            
            # Run the init container to create buckets
            Write-Status "Creating test buckets..."
            docker-compose -f $ComposeFile up minio-init
            Start-Sleep -Seconds 2
            return $true
        }
        $attempt++
        Write-Host "." -NoNewline
        Start-Sleep -Seconds 1
    }
    
    Write-Status "MinIO failed to start within timeout" "Red"
    return $false
}

function Stop-MinIO {
    Write-Status "Stopping MinIO..." "Yellow"
    docker-compose -f $ComposeFile down -v
    Write-Status "MinIO stopped and volumes cleaned" "Green"
}

function Run-IntegrationTests {
    param([string]$Package)
    
    Write-Status "Running integration tests..." "Cyan"
    
    $testPath = if ($Package -eq "all") {
        "./..."
    } else {
        "./internal/$Package/..."
    }
    
    $env:INTEGRATION_TEST = "true"
    
    try {
        go test -v -tags=integration -timeout=5m $testPath
        $testResult = $LASTEXITCODE
    } finally {
        Remove-Item Env:\INTEGRATION_TEST -ErrorAction SilentlyContinue
    }
    
    return $testResult
}

# Main execution
try {
    Write-Host "`n========================================" -ForegroundColor Cyan
    Write-Host "  Terraform Mirror Integration Tests" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
    
    # Check if MinIO is already running
    if (Test-MinIOReady) {
        Write-Status "MinIO is already running" "Green"
    } else {
        if (-not (Start-MinIO)) {
            exit 1
        }
    }
    
    # Run tests
    $testResult = Run-IntegrationTests -Package $Package
    
    if ($testResult -eq 0) {
        Write-Status "All integration tests passed!" "Green"
    } else {
        Write-Status "Some integration tests failed" "Red"
    }
    
    # Cleanup unless -KeepRunning is specified
    if (-not $KeepRunning) {
        Stop-MinIO
    } else {
        Write-Status "MinIO left running. Stop with: docker-compose -f $ComposeFile down -v" "Yellow"
    }
    
    exit $testResult
    
} catch {
    Write-Status "Error: $_" "Red"
    if (-not $KeepRunning) {
        Stop-MinIO
    }
    exit 1
}
