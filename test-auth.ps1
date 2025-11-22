# Test Authentication System
# This script tests the login/logout functionality

Write-Host "=== Testing Authentication System ===" -ForegroundColor Cyan

# Configuration
$BaseUrl = "http://localhost:8080"
$Username = "admin"
$Password = "TestPassword123!"

# Clean up old database
if (Test-Path "terraform-mirror-dev.db") {
    Write-Host "Removing old database..." -ForegroundColor Yellow
    Remove-Item "terraform-mirror-dev.db"
}

# Build the application
Write-Host "`nBuilding application..." -ForegroundColor Cyan
go build -o terraform-mirror.exe ./cmd/terraform-mirror
if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed!" -ForegroundColor Red
    exit 1
}

# Start the server in background
Write-Host "`nStarting server..." -ForegroundColor Cyan
$serverProcess = Start-Process -FilePath ".\terraform-mirror.exe" `
    -ArgumentList "serve", "--config", "config.dev.hcl" `
    -PassThru `
    -RedirectStandardOutput "server-output.log" `
    -RedirectStandardError "server-error.log"

# Wait for server to start
Write-Host "Waiting for server to start..." -ForegroundColor Yellow
Start-Sleep -Seconds 3

# Check if server is running
try {
    $health = Invoke-RestMethod -Uri "$BaseUrl/health" -Method Get
    Write-Host "Server is healthy: $($health.status)" -ForegroundColor Green
} catch {
    Write-Host "Server failed to start!" -ForegroundColor Red
    Stop-Process -Id $serverProcess.Id -Force
    Get-Content "server-error.log"
    exit 1
}

# Create initial admin user using Go code
Write-Host "`nCreating initial admin user..." -ForegroundColor Cyan
$createUserScript = @"
package main

import (
    "fmt"
    "log"
    "golang.org/x/crypto/bcrypt"
    "github.com/ned1313/terraform-mirror/internal/database"
)

func main() {
    db, err := database.New("terraform-mirror-dev.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Hash the password
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte("$Password"), 12)
    if err != nil {
        log.Fatal(err)
    }

    err = database.CreateInitialAdminUser(db, "$Username", string(hashedPassword))
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Admin user created successfully")
}
"@

# Actually, let's just use SQL directly since the server creates the database
Stop-Process -Id $serverProcess.Id -Force
Start-Sleep -Seconds 1

# Use Go to hash the password and create the user
Write-Host "Generating password hash..." -ForegroundColor Cyan
$hashScript = @"
package main
import (
    "fmt"
    "golang.org/x/crypto/bcrypt"
    "os"
)
func main() {
    hash, _ := bcrypt.GenerateFromPassword([]byte(os.Args[1]), 12)
    fmt.Print(string(hash))
}
"@

$hashScript | Out-File -FilePath "temp_hash.go" -Encoding UTF8
$passwordHash = go run temp_hash.go "$Password"
Remove-Item "temp_hash.go"

# Insert user directly into database using sqlite3 (if available) or Go
Write-Host "Inserting admin user into database..." -ForegroundColor Cyan
$insertScript = @"
package main
import (
    "database/sql"
    "log"
    _ "modernc.org/sqlite"
)
func main() {
    db, err := sql.Open("sqlite", "terraform-mirror-dev.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    _, err = db.Exec(`
        INSERT INTO admin_users (username, password_hash, full_name, email, active)
        VALUES (?, ?, ?, ?, ?)
    `, "$Username", "$passwordHash", "System Administrator", "admin@localhost", true)
    
    if err != nil {
        log.Fatal(err)
    }
    log.Println("User inserted successfully")
}
"@

$insertScript | Out-File -FilePath "temp_insert.go" -Encoding UTF8
go run temp_insert.go
Remove-Item "temp_insert.go"

# Restart server
Write-Host "`nRestarting server..." -ForegroundColor Cyan
$serverProcess = Start-Process -FilePath ".\terraform-mirror.exe" `
    -ArgumentList "serve", "--config", "config.dev.hcl" `
    -PassThru `
    -RedirectStandardOutput "server-output.log" `
    -RedirectStandardError "server-error.log"

Start-Sleep -Seconds 3

# Test 1: Login with correct credentials
Write-Host "`n=== Test 1: Login with correct credentials ===" -ForegroundColor Cyan
try {
    $loginBody = @{
        username = $Username
        password = $Password
    } | ConvertTo-Json

    $loginResponse = Invoke-RestMethod -Uri "$BaseUrl/admin/api/login" `
        -Method Post `
        -Body $loginBody `
        -ContentType "application/json"
    
    Write-Host "✓ Login successful!" -ForegroundColor Green
    Write-Host "  Token: $($loginResponse.token.Substring(0, 20))..." -ForegroundColor Gray
    Write-Host "  User: $($loginResponse.user.username)" -ForegroundColor Gray
    Write-Host "  Expires: $($loginResponse.expires_at)" -ForegroundColor Gray
    
    $token = $loginResponse.token
} catch {
    Write-Host "✗ Login failed: $($_.Exception.Message)" -ForegroundColor Red
    $_.Exception.Response | ConvertTo-Json
}

# Test 2: Login with wrong password
Write-Host "`n=== Test 2: Login with wrong password ===" -ForegroundColor Cyan
try {
    $loginBody = @{
        username = $Username
        password = "WrongPassword123!"
    } | ConvertTo-Json

    $loginResponse = Invoke-RestMethod -Uri "$BaseUrl/admin/api/login" `
        -Method Post `
        -Body $loginBody `
        -ContentType "application/json"
    
    Write-Host "✗ Should have failed but didn't!" -ForegroundColor Red
} catch {
    if ($_.Exception.Response.StatusCode -eq 401) {
        Write-Host "✓ Correctly rejected invalid credentials" -ForegroundColor Green
    } else {
        Write-Host "✗ Wrong error code: $($_.Exception.Response.StatusCode)" -ForegroundColor Red
    }
}

# Test 3: Access protected endpoint with token
Write-Host "`n=== Test 3: Access protected endpoint with token ===" -ForegroundColor Cyan
try {
    $headers = @{
        Authorization = "Bearer $token"
    }
    
    $jobs = Invoke-RestMethod -Uri "$BaseUrl/admin/api/jobs" `
        -Method Get `
        -Headers $headers
    
    Write-Host "✓ Successfully accessed protected endpoint" -ForegroundColor Green
    Write-Host "  Jobs count: $($jobs.jobs.Count)" -ForegroundColor Gray
} catch {
    Write-Host "✗ Failed to access protected endpoint: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 4: Logout
Write-Host "`n=== Test 4: Logout ===" -ForegroundColor Cyan
try {
    $headers = @{
        Authorization = "Bearer $token"
    }
    
    Invoke-RestMethod -Uri "$BaseUrl/admin/api/logout" `
        -Method Post `
        -Headers $headers
    
    Write-Host "✓ Logout successful" -ForegroundColor Green
} catch {
    Write-Host "✗ Logout failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 5: Try to use token after logout
Write-Host "`n=== Test 5: Try to use token after logout ===" -ForegroundColor Cyan
try {
    $headers = @{
        Authorization = "Bearer $token"
    }
    
    $jobs = Invoke-RestMethod -Uri "$BaseUrl/admin/api/jobs" `
        -Method Get `
        -Headers $headers
    
    Write-Host "✗ Should have failed but didn't!" -ForegroundColor Red
} catch {
    if ($_.Exception.Response.StatusCode -eq 401) {
        Write-Host "✓ Correctly rejected revoked token" -ForegroundColor Green
    } else {
        Write-Host "✗ Wrong error code: $($_.Exception.Response.StatusCode)" -ForegroundColor Red
    }
}

# Cleanup
Write-Host "`n=== Cleanup ===" -ForegroundColor Cyan
Stop-Process -Id $serverProcess.Id -Force
Write-Host "Server stopped" -ForegroundColor Yellow

Write-Host "`nServer logs:" -ForegroundColor Cyan
Get-Content "server-output.log" | Select-Object -Last 20

if (Test-Path "server-error.log") {
    $errors = Get-Content "server-error.log"
    if ($errors) {
        Write-Host "`nServer errors:" -ForegroundColor Red
        $errors | Select-Object -Last 10
    }
}

Write-Host "`n=== Test Complete ===" -ForegroundColor Cyan
