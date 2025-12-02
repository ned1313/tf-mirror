# Test script for job processing
Write-Host "=== Terraform Mirror Job Processing Test ===" -ForegroundColor Cyan

# Set MinIO credentials
$env:AWS_ACCESS_KEY_ID = "minioadmin"
$env:AWS_SECRET_ACCESS_KEY = "minioadmin"

Write-Host "`nStarting server in background..." -ForegroundColor Yellow
Start-Process powershell -ArgumentList "-NoExit", "-Command", ".\terraform-mirror.exe --config config.dev.hcl" -WindowStyle Normal

# Wait for server to start
Write-Host "Waiting for server to start..." -ForegroundColor Yellow
Start-Sleep -Seconds 3

try {
    # Test health endpoint
    Write-Host "`nTesting health endpoint..." -ForegroundColor Yellow
    $health = Invoke-RestMethod -Uri "http://localhost:8080/health" -Method GET
    Write-Host "Health: $($health.status)" -ForegroundColor Green

    # Upload HCL file to create job
    Write-Host "`nUploading provider definitions..." -ForegroundColor Yellow
    $form = @{
        file = Get-Item -Path "test-providers.hcl"
    }
    $response = Invoke-RestMethod -Uri "http://localhost:8080/admin/api/providers/load" -Method POST -Form $form
    Write-Host "Job created!" -ForegroundColor Green
    Write-Host "  Job ID: $($response.job_id)" -ForegroundColor Cyan
    Write-Host "  Message: $($response.message)" -ForegroundColor Cyan
    Write-Host "  Total Providers: $($response.total_providers)" -ForegroundColor Cyan
    
    $jobId = $response.job_id

    # Get job details
    Write-Host "`nRetrieving job details..." -ForegroundColor Yellow
    $job = Invoke-RestMethod -Uri "http://localhost:8080/admin/api/jobs/$jobId" -Method GET
    Write-Host "Job Status: $($job.status)" -ForegroundColor $(if ($job.status -eq "completed") { "Green" } else { "Yellow" })
    Write-Host "  Progress: $($job.progress)%" -ForegroundColor Cyan
    Write-Host "  Total Items: $($job.total_items)" -ForegroundColor Cyan
    Write-Host "  Completed: $($job.completed_items)" -ForegroundColor Cyan
    Write-Host "  Failed: $($job.failed_items)" -ForegroundColor Cyan
    
    if ($job.items) {
        Write-Host "`nJob Items:" -ForegroundColor Yellow
        foreach ($item in $job.items) {
            $status_color = switch ($item.status) {
                "completed" { "Green" }
                "failed" { "Red" }
                default { "Yellow" }
            }
            Write-Host "  - $($item.namespace)/$($item.type) v$($item.version) [$($item.platform)]: $($item.status)" -ForegroundColor $status_color
            if ($item.error_message) {
                Write-Host "    Error: $($item.error_message)" -ForegroundColor Red
            }
        }
    }

    # List all jobs
    Write-Host "`nListing all jobs..." -ForegroundColor Yellow
    $jobs = Invoke-RestMethod -Uri "http://localhost:8080/admin/api/jobs" -Method GET
    Write-Host "Total jobs: $($jobs.total)" -ForegroundColor Cyan

    # Check MinIO for uploaded files (requires AWS CLI or mc client)
    Write-Host "`nChecking MinIO storage..." -ForegroundColor Yellow
    Write-Host "  To verify files in MinIO, open: http://localhost:9001" -ForegroundColor Cyan
    Write-Host "  Username: minioadmin" -ForegroundColor Cyan
    Write-Host "  Password: minioadmin" -ForegroundColor Cyan
    
    Write-Host "`n=== Test Complete ===" -ForegroundColor Green

} catch {
    Write-Host "`nError occurred: $_" -ForegroundColor Red
    Write-Host $_.Exception.Message -ForegroundColor Red
}

Write-Host "`nPress any key to continue (server is still running in separate window)..." -ForegroundColor Yellow
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
