# Simple Authentication Test
Write-Host "=== Simple Authentication Test ===" -ForegroundColor Cyan

# Configuration
$BaseUrl = "http://localhost:8080"
$Username = "admin"
$Password = "TestPassword123!"
$DbPath = "terraform-mirror-dev.db"

# Step 1: Clean up
Write-Host "`n1. Cleaning up old database..." -ForegroundColor Yellow
if (Test-Path $DbPath) {
    Remove-Item $DbPath -Force
    Write-Host "   Database removed" -ForegroundColor Gray
}

# Step 2: Build tools
Write-Host "`n2. Building tools..." -ForegroundColor Yellow
go build -o create-admin.exe ./cmd/create-admin
if ($LASTEXITCODE -ne 0) {
    Write-Host "   Failed to build create-admin tool!" -ForegroundColor Red
    exit 1
}
Write-Host "   Tools built successfully" -ForegroundColor Gray

# Step 3: Start server
Write-Host "`n3. Starting server..." -ForegroundColor Yellow
$env:TFM_DATABASE_PATH = $DbPath
$env:TFM_AUTH_JWT_SECRET = "test-secret-key-at-least-32-characters-long"
$serverProcess = Start-Process -FilePath ".\terraform-mirror.exe" `
    -ArgumentList "serve" `
    -PassThru `
    -RedirectStandardOutput "server-output.log" `
    -RedirectStandardError "server-error.log"

Start-Sleep -Seconds 2

# Check if server is running
try {
    $health = Invoke-RestMethod -Uri "$BaseUrl/health" -Method Get -TimeoutSec 5
    Write-Host "   Server is running: $($health.status)" -ForegroundColor Green
} catch {
    Write-Host "   Server failed to start!" -ForegroundColor Red
    Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue
    Get-Content "server-error.log"
    exit 1
}

# Step 4: Create admin user
Write-Host "`n4. Creating admin user..." -ForegroundColor Yellow
$createResult = & .\create-admin.exe -db $DbPath -username $Username -password $Password
Write-Host "   $createResult" -ForegroundColor Gray

# Give the server a moment
Start-Sleep -Seconds 1

# Step 5: Test login with correct credentials
Write-Host "`n5. Testing login with correct credentials..." -ForegroundColor Yellow
try {
    $loginBody = @{
        username = $Username
        password = $Password
    } | ConvertTo-Json

    $loginResponse = Invoke-RestMethod -Uri "$BaseUrl/admin/api/login" `
        -Method Post `
        -Body $loginBody `
        -ContentType "application/json"
    
    Write-Host "   ✓ Login successful!" -ForegroundColor Green
    Write-Host "     Token: $($loginResponse.token.Substring(0, 30))..." -ForegroundColor Gray
    Write-Host "     User: $($loginResponse.user.username) (ID: $($loginResponse.user.id))" -ForegroundColor Gray
    Write-Host "     Expires: $($loginResponse.expires_at)" -ForegroundColor Gray
    
    $token = $loginResponse.token
} catch {
    Write-Host "   ✗ Login failed!" -ForegroundColor Red
    Write-Host "   Status: $($_.Exception.Response.StatusCode)" -ForegroundColor Red
    Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Red
    $token = $null
}

# Step 6: Test login with wrong password
Write-Host "`n6. Testing login with wrong password..." -ForegroundColor Yellow
try {
    $loginBody = @{
        username = $Username
        password = "WrongPassword123!"
    } | ConvertTo-Json

    Invoke-RestMethod -Uri "$BaseUrl/admin/api/login" `
        -Method Post `
        -Body $loginBody `
        -ContentType "application/json" | Out-Null
    
    Write-Host "   ✗ Should have been rejected!" -ForegroundColor Red
} catch {
    if ($_.Exception.Response.StatusCode -eq 401) {
        Write-Host "   ✓ Correctly rejected invalid password" -ForegroundColor Green
    } else {
        Write-Host "   ✗ Wrong error code: $($_.Exception.Response.StatusCode)" -ForegroundColor Red
    }
}

if ($token) {
    # Step 7: Access protected endpoint
    Write-Host "`n7. Testing access to protected endpoint..." -ForegroundColor Yellow
    try {
        $headers = @{
            Authorization = "Bearer $token"
        }
        
        $jobs = Invoke-RestMethod -Uri "$BaseUrl/admin/api/jobs" `
            -Method Get `
            -Headers $headers
        
        Write-Host "   ✓ Successfully accessed /admin/api/jobs" -ForegroundColor Green
        Write-Host "     Total jobs: $($jobs.total)" -ForegroundColor Gray
    } catch {
        Write-Host "   ✗ Failed to access protected endpoint" -ForegroundColor Red
        Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Red
    }

    # Step 8: Logout
    Write-Host "`n8. Testing logout..." -ForegroundColor Yellow
    try {
        $headers = @{
            Authorization = "Bearer $token"
        }
        
        Invoke-RestMethod -Uri "$BaseUrl/admin/api/logout" `
            -Method Post `
            -Headers $headers | Out-Null
        
        Write-Host "   ✓ Logout successful" -ForegroundColor Green
    } catch {
        Write-Host "   ✗ Logout failed" -ForegroundColor Red
        Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Red
    }

    # Step 9: Try to use revoked token
    Write-Host "`n9. Testing access with revoked token..." -ForegroundColor Yellow
    try {
        $headers = @{
            Authorization = "Bearer $token"
        }
        
        Invoke-RestMethod -Uri "$BaseUrl/admin/api/jobs" `
            -Method Get `
            -Headers $headers | Out-Null
        
        Write-Host "   ✗ Should have been rejected!" -ForegroundColor Red
    } catch {
        if ($_.Exception.Response.StatusCode -eq 401) {
            Write-Host "   ✓ Correctly rejected revoked token" -ForegroundColor Green
        } else {
            Write-Host "   ✗ Wrong error code: $($_.Exception.Response.StatusCode)" -ForegroundColor Red
        }
    }
}

# Cleanup
Write-Host "`n10. Cleanup..." -ForegroundColor Yellow
Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue
Write-Host "    Server stopped" -ForegroundColor Gray

Write-Host "`n=== Test Complete ===" -ForegroundColor Cyan
Write-Host "`nServer output (last 10 lines):" -ForegroundColor Cyan
Get-Content "server-output.log" -Tail 10 -ErrorAction SilentlyContinue

Write-Host "`nServer errors:" -ForegroundColor Cyan
Get-Content "server-error.log" -ErrorAction SilentlyContinue | Select-Object -Last 5
