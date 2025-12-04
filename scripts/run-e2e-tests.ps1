<#
.SYNOPSIS
    Run end-to-end tests for Terraform Mirror
.DESCRIPTION
    Starts the full Terraform Mirror stack (app + MinIO), runs E2E tests
    including provider loading and Terraform CLI integration, then cleans up.
.EXAMPLE
    .\scripts\run-e2e-tests.ps1
    .\scripts\run-e2e-tests.ps1 -KeepRunning
    .\scripts\run-e2e-tests.ps1 -SkipTerraformTests
#>
param(
    [switch]$KeepRunning,
    [switch]$SkipTerraformTests,
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
if (-not $ProjectRoot) {
    $ProjectRoot = (Get-Location).Path
}

# Configuration
$MirrorUrl = "http://localhost:8080"
$AdminUsername = "admin"
$AdminPassword = "testpassword123"
$ComposeFile = Join-Path $ProjectRoot "deployments\docker-compose\docker-compose.yml"

function Write-Status {
    param([string]$Message, [string]$Color = "Cyan")
    Write-Host "`n[$([DateTime]::Now.ToString('HH:mm:ss'))] $Message" -ForegroundColor $Color
}

function Write-TestResult {
    param([string]$TestName, [bool]$Passed, [string]$Details = "")
    $status = if ($Passed) { "[PASS]" } else { "[FAIL]" }
    $color = if ($Passed) { "Green" } else { "Red" }
    Write-Host "  $status $TestName" -ForegroundColor $color
    if ($Details -and -not $Passed) {
        Write-Host "         $Details" -ForegroundColor DarkGray
    }
}

function Test-ServiceReady {
    param([string]$Url, [int]$TimeoutSeconds = 60)
    
    $stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    while ($stopwatch.Elapsed.TotalSeconds -lt $TimeoutSeconds) {
        try {
            $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 2 -ErrorAction SilentlyContinue
            if ($response.StatusCode -eq 200) {
                return $true
            }
        } catch {
            # Service not ready yet
        }
        Start-Sleep -Seconds 1
    }
    return $false
}

function Start-Stack {
    Write-Status "Building and starting Terraform Mirror stack..." "Yellow"
    
    # Set environment for docker-compose
    $env:TFM_ADMIN_PASSWORD = $AdminPassword
    
    if ($SkipBuild) {
        docker-compose -f $ComposeFile up -d
    } else {
        docker-compose -f $ComposeFile up -d --build
    }
    
    Write-Status "Waiting for services to be healthy..."
    
    # Wait for MinIO
    if (-not (Test-ServiceReady -Url "http://localhost:9000/minio/health/live" -TimeoutSeconds 30)) {
        Write-Status "MinIO failed to start" "Red"
        return $false
    }
    Write-Host "  MinIO is ready" -ForegroundColor Green
    
    # Wait for Terraform Mirror
    if (-not (Test-ServiceReady -Url "$MirrorUrl/health" -TimeoutSeconds 60)) {
        Write-Status "Terraform Mirror failed to start" "Red"
        docker-compose -f $ComposeFile logs terraform-mirror
        return $false
    }
    Write-Host "  Terraform Mirror is ready" -ForegroundColor Green
    
    return $true
}

function Stop-Stack {
    Write-Status "Stopping stack and cleaning up..." "Yellow"
    docker-compose -f $ComposeFile down -v
    Write-Status "Stack stopped" "Green"
}

function Get-AuthToken {
    try {
        $body = @{
            username = $AdminUsername
            password = $AdminPassword
        } | ConvertTo-Json
        
        $response = Invoke-RestMethod -Uri "$MirrorUrl/admin/api/login" -Method Post -Body $body -ContentType "application/json"
        return $response.token
    } catch {
        Write-Status "Failed to authenticate: $_" "Red"
        return $null
    }
}

function Test-ServiceDiscovery {
    try {
        $response = Invoke-RestMethod -Uri "$MirrorUrl/.well-known/terraform.json" -Method Get
        return ($null -ne $response.'providers.v1')
    } catch {
        return $false
    }
}

function Test-AdminLogin {
    $token = Get-AuthToken
    return ($null -ne $token -and $token.Length -gt 0)
}

function Test-ProviderLoad {
    param([string]$Token)
    
    # Create a minimal provider definition for testing
    $providerHcl = @'
# Test provider definition - small provider for quick testing
provider "hashicorp/null" {
  versions  = ["3.2.0"]
  platforms = ["linux_amd64"]
}
'@
    
    try {
        # Create multipart form data
        $boundary = [System.Guid]::NewGuid().ToString()
        $LF = "`r`n"
        
        $bodyLines = @(
            "--$boundary",
            "Content-Disposition: form-data; name=`"file`"; filename=`"test-providers.hcl`"",
            "Content-Type: text/plain",
            "",
            $providerHcl,
            "--$boundary--"
        ) -join $LF
        
        $headers = @{
            "Authorization" = "Bearer $Token"
            "Content-Type" = "multipart/form-data; boundary=$boundary"
        }
        
        $response = Invoke-RestMethod -Uri "$MirrorUrl/admin/api/providers/load" -Method Post -Body $bodyLines -Headers $headers
        
        # Response should contain a job ID
        return ($null -ne $response.job_id -or $null -ne $response.id)
    } catch {
        Write-Host "Provider load error: $_" -ForegroundColor DarkGray
        return $false
    }
}

function Test-ProviderVersionList {
    try {
        $response = Invoke-RestMethod -Uri "$MirrorUrl/v1/providers/hashicorp/null/versions" -Method Get -ErrorAction SilentlyContinue
        # May be empty initially if provider hasn't been loaded yet
        return $true
    } catch {
        # 404 is acceptable if no providers loaded
        if ($_.Exception.Response.StatusCode -eq 404) {
            return $true
        }
        return $false
    }
}

function Test-HealthEndpoint {
    try {
        $response = Invoke-RestMethod -Uri "$MirrorUrl/health" -Method Get
        return ($response.status -eq "ok" -or $response.status -eq "healthy")
    } catch {
        return $false
    }
}

function Test-MetricsEndpoint {
    try {
        $response = Invoke-WebRequest -Uri "$MirrorUrl/metrics" -UseBasicParsing -Method Get
        $content = $response.Content
        return ($content -match "go_goroutines" -or $content -match "terraform_mirror")
    } catch {
        return $false
    }
}

function Wait-ForJobCompletion {
    param([string]$Token, [int]$TimeoutSeconds = 120)
    
    $headers = @{ "Authorization" = "Bearer $Token" }
    $stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    
    while ($stopwatch.Elapsed.TotalSeconds -lt $TimeoutSeconds) {
        try {
            $jobs = Invoke-RestMethod -Uri "$MirrorUrl/admin/api/jobs?status=pending" -Headers $headers -Method Get
            $pendingCount = if ($jobs.jobs) { $jobs.jobs.Count } else { 0 }
            
            $processingJobs = Invoke-RestMethod -Uri "$MirrorUrl/admin/api/jobs?status=processing" -Headers $headers -Method Get
            $processingCount = if ($processingJobs.jobs) { $processingJobs.jobs.Count } else { 0 }
            
            if ($pendingCount -eq 0 -and $processingCount -eq 0) {
                return $true
            }
            
            Write-Host "." -NoNewline
            Start-Sleep -Seconds 2
        } catch {
            Start-Sleep -Seconds 2
        }
    }
    Write-Host ""
    return $false
}

function Test-TerraformInit {
    param([string]$TempDir)
    
    # Check if terraform is available
    $terraformPath = Get-Command terraform -ErrorAction SilentlyContinue
    if (-not $terraformPath) {
        Write-Host "  [SKIP] Terraform CLI not found" -ForegroundColor Yellow
        return $null
    }
    
    # Create a test Terraform configuration
    $tfConfig = @"
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
"@
    
    # Create terraformrc to use our mirror
    $terraformRc = @"
provider_installation {
  network_mirror {
    url = "$MirrorUrl/v1/providers/"
  }
}
"@
    
    try {
        # Create temp directory for test
        $testDir = Join-Path $TempDir "tf-test"
        New-Item -ItemType Directory -Path $testDir -Force | Out-Null
        
        # Write config files
        Set-Content -Path (Join-Path $testDir "main.tf") -Value $tfConfig
        $rcFile = Join-Path $TempDir ".terraformrc"
        Set-Content -Path $rcFile -Value $terraformRc
        
        # Set environment
        $env:TF_CLI_CONFIG_FILE = $rcFile
        $env:TF_LOG = ""
        
        # Run terraform init
        Push-Location $testDir
        try {
            $initOutput = & terraform init 2>&1
            $initExitCode = $LASTEXITCODE
            
            if ($initExitCode -eq 0) {
                return $true
            } else {
                Write-Host "Terraform init output: $initOutput" -ForegroundColor DarkGray
                return $false
            }
        } finally {
            Pop-Location
            Remove-Item Env:\TF_CLI_CONFIG_FILE -ErrorAction SilentlyContinue
        }
    } catch {
        Write-Host "Terraform test error: $_" -ForegroundColor DarkGray
        return $false
    }
}

# Main execution
$testResults = @{
    Passed = 0
    Failed = 0
    Skipped = 0
}

try {
    Write-Host "`n========================================" -ForegroundColor Cyan
    Write-Host "  Terraform Mirror E2E Tests" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
    
    # Start the stack
    if (-not (Start-Stack)) {
        Write-Status "Failed to start stack" "Red"
        exit 1
    }
    
    Write-Status "Running E2E tests..." "Cyan"
    
    # Test 1: Health endpoint
    $healthPassed = Test-HealthEndpoint
    Write-TestResult "Health endpoint" $healthPassed
    if ($healthPassed) { $testResults.Passed++ } else { $testResults.Failed++ }
    
    # Test 2: Metrics endpoint
    $metricsPassed = Test-MetricsEndpoint
    Write-TestResult "Metrics endpoint" $metricsPassed
    if ($metricsPassed) { $testResults.Passed++ } else { $testResults.Failed++ }
    
    # Test 3: Service discovery
    $discoveryPassed = Test-ServiceDiscovery
    Write-TestResult "Service discovery (/.well-known/terraform.json)" $discoveryPassed
    if ($discoveryPassed) { $testResults.Passed++ } else { $testResults.Failed++ }
    
    # Test 4: Admin login
    $loginPassed = Test-AdminLogin
    Write-TestResult "Admin login" $loginPassed
    if ($loginPassed) { $testResults.Passed++ } else { $testResults.Failed++ }
    
    # Test 5: Provider version list (may be empty)
    $versionListPassed = Test-ProviderVersionList
    Write-TestResult "Provider version list endpoint" $versionListPassed
    if ($versionListPassed) { $testResults.Passed++ } else { $testResults.Failed++ }
    
    # Only continue with provider tests if login worked
    if ($loginPassed) {
        $token = Get-AuthToken
        
        # Test 6: Provider load
        $loadPassed = Test-ProviderLoad -Token $token
        Write-TestResult "Provider load (upload HCL)" $loadPassed
        if ($loadPassed) { $testResults.Passed++ } else { $testResults.Failed++ }
        
        if ($loadPassed) {
            # Test 7: Wait for job completion
            Write-Host "  Waiting for provider download job to complete..."
            $jobCompleted = Wait-ForJobCompletion -Token $token -TimeoutSeconds 120
            Write-TestResult "Job completion" $jobCompleted
            if ($jobCompleted) { $testResults.Passed++ } else { $testResults.Failed++ }
            
            # Test 8: Terraform CLI integration (optional)
            if (-not $SkipTerraformTests -and $jobCompleted) {
                $tempDir = Join-Path $env:TEMP "tf-mirror-e2e-$(Get-Random)"
                New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
                
                try {
                    $terraformPassed = Test-TerraformInit -TempDir $tempDir
                    if ($null -eq $terraformPassed) {
                        $testResults.Skipped++
                    } elseif ($terraformPassed) {
                        Write-TestResult "Terraform init with mirror" $true
                        $testResults.Passed++
                    } else {
                        Write-TestResult "Terraform init with mirror" $false "Provider may not have finished downloading"
                        $testResults.Failed++
                    }
                } finally {
                    Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
                }
            } elseif ($SkipTerraformTests) {
                Write-Host "  [SKIP] Terraform CLI tests (--SkipTerraformTests)" -ForegroundColor Yellow
                $testResults.Skipped++
            }
        }
    }
    
    # Summary
    Write-Host "`n========================================" -ForegroundColor Cyan
    Write-Host "  Test Results" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "  Passed:  $($testResults.Passed)" -ForegroundColor Green
    Write-Host "  Failed:  $($testResults.Failed)" -ForegroundColor $(if ($testResults.Failed -gt 0) { "Red" } else { "Green" })
    Write-Host "  Skipped: $($testResults.Skipped)" -ForegroundColor Yellow
    Write-Host ""
    
    # Cleanup
    if (-not $KeepRunning) {
        Stop-Stack
    } else {
        Write-Status "Stack left running at $MirrorUrl" "Yellow"
        Write-Host "  Admin UI: $MirrorUrl/admin" -ForegroundColor DarkGray
        Write-Host "  MinIO Console: http://localhost:9001" -ForegroundColor DarkGray
        Write-Host "  Stop with: docker-compose -f `"$ComposeFile`" down -v" -ForegroundColor DarkGray
    }
    
    # Exit with appropriate code
    if ($testResults.Failed -gt 0) {
        exit 1
    }
    exit 0
    
} catch {
    Write-Status "Error: $_" "Red"
    if (-not $KeepRunning) {
        Stop-Stack
    }
    exit 1
}
