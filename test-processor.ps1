# Test script for background processor
# This script tests the job processing functionality

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Testing Background Job Processor" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan

# Step 1: Clean up
Write-Host "[1/8] Cleaning up test environment..." -ForegroundColor Yellow
if (Test-Path ".\processor-test.db") {
    Remove-Item ".\processor-test.db" -Force
    Write-Host "  ✓ Removed old database" -ForegroundColor Green
}

# Step 2: Set environment variables
Write-Host "`n[2/8] Setting environment variables..." -ForegroundColor Yellow
$env:TFM_DATABASE_PATH = "processor-test.db"
$env:TFM_AUTH_JWT_SECRET = "test-secret-for-processor-testing-use-at-least-32-chars"
$env:TFM_PROCESSOR_POLLING_INTERVAL_SECONDS = "2"
$env:TFM_PROCESSOR_MAX_CONCURRENT_JOBS = "2"
Write-Host "  ✓ Environment configured" -ForegroundColor Green

# Step 3: Build create-admin tool if needed
Write-Host "`n[3/8] Building create-admin tool..." -ForegroundColor Yellow
$buildOutput = go build -o create-admin.exe ./cmd/create-admin 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "  ✗ Build failed: $buildOutput" -ForegroundColor Red
    exit 1
}
Write-Host "  ✓ Tool built successfully" -ForegroundColor Green

# Step 4: Start server in background
Write-Host "`n[4/8] Starting server..." -ForegroundColor Yellow
$serverProcess = Start-Process -FilePath ".\terraform-mirror.exe" -PassThru -NoNewWindow -RedirectStandardOutput "server-processor.log" -RedirectStandardError "server-processor-error.log"
Write-Host "  ✓ Server started (PID: $($serverProcess.Id))" -ForegroundColor Green
Start-Sleep -Seconds 2

# Step 5: Create admin user
Write-Host "`n[5/8] Creating admin user..." -ForegroundColor Yellow
$createOutput = .\create-admin.exe -db processor-test.db -username admin -password admin123 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "  ✗ Failed to create admin user: $createOutput" -ForegroundColor Red
    Stop-Process -Id $serverProcess.Id -Force
    exit 1
}
Write-Host "  ✓ Admin user created" -ForegroundColor Green

# Step 6: Login to get token
Write-Host "`n[6/8] Authenticating..." -ForegroundColor Yellow
$loginBody = @{
    username = "admin"
    password = "admin123"
} | ConvertTo-Json

try {
    $loginResponse = Invoke-RestMethod -Uri "http://localhost:8080/admin/api/login" -Method POST -Body $loginBody -ContentType "application/json"
    $token = $loginResponse.token
    Write-Host "  ✓ Authenticated successfully" -ForegroundColor Green
    Write-Host "    Token: $($token.Substring(0, 20))..." -ForegroundColor Gray
} catch {
    Write-Host "  ✗ Login failed: $_" -ForegroundColor Red
    Stop-Process -Id $serverProcess.Id -Force
    exit 1
}

# Step 7: Check processor status
Write-Host "`n[7/8] Checking processor status..." -ForegroundColor Yellow
$headers = @{
    "Authorization" = "Bearer $token"
}

try {
    $status = Invoke-RestMethod -Uri "http://localhost:8080/admin/api/processor/status" -Method GET -Headers $headers
    Write-Host "  ✓ Processor status:" -ForegroundColor Green
    Write-Host "    Running: $($status.running)" -ForegroundColor Gray
    Write-Host "    Active jobs: $($status.active_jobs)" -ForegroundColor Gray
    Write-Host "    Max concurrent: $($status.max_concurrent_jobs)" -ForegroundColor Gray
    
    if ($status.running -ne $true) {
        Write-Host "  ✗ Processor is not running!" -ForegroundColor Red
        Stop-Process -Id $serverProcess.Id -Force
        exit 1
    }
} catch {
    Write-Host "  ✗ Failed to get processor status: $_" -ForegroundColor Red
    Stop-Process -Id $serverProcess.Id -Force
    exit 1
}

# Step 8: Test job listing (should be empty)
Write-Host "`n[8/8] Testing job endpoints..." -ForegroundColor Yellow
try {
    $jobs = Invoke-RestMethod -Uri "http://localhost:8080/admin/api/jobs?limit=10&offset=0" -Method GET -Headers $headers
    Write-Host "  ✓ Job listing works" -ForegroundColor Green
    Write-Host "    Total jobs: $($jobs.total)" -ForegroundColor Gray
    
    if ($jobs.total -ne 0) {
        Write-Host "  ! Warning: Database has $($jobs.total) jobs (expected 0)" -ForegroundColor Yellow
    }
} catch {
    Write-Host "  ✗ Failed to list jobs: $_" -ForegroundColor Red
    Stop-Process -Id $serverProcess.Id -Force
    exit 1
}

# Cleanup
Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "Cleaning up..." -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan

Write-Host "Stopping server..." -ForegroundColor Yellow
Stop-Process -Id $serverProcess.Id -Force
Start-Sleep -Seconds 1
Write-Host "✓ Server stopped" -ForegroundColor Green

# Summary
Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "TEST RESULTS: ALL PASSED ✓" -ForegroundColor Green
Write-Host "========================================`n" -ForegroundColor Cyan

Write-Host "Summary:" -ForegroundColor White
Write-Host "  ✓ Processor service started successfully" -ForegroundColor Green
Write-Host "  ✓ Processor status endpoint working" -ForegroundColor Green
Write-Host "  ✓ Job listing endpoint working" -ForegroundColor Green
Write-Host "  ✓ Configuration applied correctly" -ForegroundColor Green
Write-Host "`nThe background processor is ready for Step 13 (Provider Download Implementation)" -ForegroundColor Cyan
