#Requires -Version 5.1
<#
.SYNOPSIS
    Pre-commit hook for terraform-mirror (Windows PowerShell version)

.DESCRIPTION
    This hook checks for common issues that will be caught by the CI pipeline.
    
    To install this hook, run:
        Copy-Item scripts\pre-commit.ps1 .git\hooks\pre-commit.ps1
    
    Note: Git for Windows may need a wrapper script. Create .git/hooks/pre-commit with:
        #!/bin/sh
        powershell.exe -ExecutionPolicy Bypass -File ".git/hooks/pre-commit.ps1"
        exit $?

.NOTES
    Exit codes:
    0 - All checks passed
    1 - One or more checks failed
#>

$ErrorActionPreference = "Continue"
$script:Failed = $false

Write-Host "Running pre-commit checks..." -ForegroundColor Cyan
Write-Host ""

# Get list of staged Go files
$StagedFiles = git diff --cached --name-only --diff-filter=ACM 2>$null | Where-Object { $_ -like "*.go" }

# =============================================================================
# Check 1: Go formatting (gofmt)
# =============================================================================
Write-Host "Checking Go formatting..." -ForegroundColor White

if ($StagedFiles) {
    $Unformatted = @()
    foreach ($file in $StagedFiles) {
        if (Test-Path $file) {
            $result = gofmt -l $file 2>&1
            if ($result) {
                $Unformatted += $result
            }
        }
    }
    
    if ($Unformatted.Count -gt 0) {
        Write-Host "X The following files are not properly formatted:" -ForegroundColor Red
        $Unformatted | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
        Write-Host ""
        Write-Host "Run 'gofmt -w <file>' to fix formatting, or 'gofmt -w .' to fix all files." -ForegroundColor Yellow
        $script:Failed = $true
    }
    else {
        Write-Host "[OK] Go formatting check passed" -ForegroundColor Green
    }
}
else {
    Write-Host "[SKIP] No Go files staged, skipping format check" -ForegroundColor Yellow
}

# =============================================================================
# Check 2: Go vet
# =============================================================================
Write-Host ""
Write-Host "Running go vet..." -ForegroundColor White

$vetOutput = go vet ./... 2>&1
$vetExitCode = $LASTEXITCODE

if ($vetExitCode -ne 0) {
    Write-Host "X go vet found issues:" -ForegroundColor Red
    Write-Host $vetOutput -ForegroundColor Red
    $script:Failed = $true
}
else {
    Write-Host "[OK] go vet passed" -ForegroundColor Green
}

# =============================================================================
# Check 3: Go build (ensures code compiles)
# =============================================================================
Write-Host ""
Write-Host "Checking Go build..." -ForegroundColor White

$buildOutput = go build ./... 2>&1
$buildExitCode = $LASTEXITCODE

if ($buildExitCode -ne 0) {
    Write-Host "X Build failed:" -ForegroundColor Red
    Write-Host $buildOutput -ForegroundColor Red
    $script:Failed = $true
}
else {
    Write-Host "[OK] Build check passed" -ForegroundColor Green
}

# =============================================================================
# Check 4: golangci-lint (if installed)
# =============================================================================
Write-Host ""
Write-Host "Running golangci-lint..." -ForegroundColor White

$lintCmd = Get-Command golangci-lint -ErrorAction SilentlyContinue
if ($lintCmd) {
    $lintOutput = golangci-lint run --timeout=5m 2>&1
    $lintExitCode = $LASTEXITCODE
    
    if ($lintExitCode -ne 0) {
        Write-Host "X golangci-lint found issues:" -ForegroundColor Red
        Write-Host $lintOutput -ForegroundColor Red
        $script:Failed = $true
    }
    else {
        Write-Host "[OK] golangci-lint passed" -ForegroundColor Green
    }
}
else {
    Write-Host "[SKIP] golangci-lint not installed, skipping" -ForegroundColor Yellow
    Write-Host "  Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" -ForegroundColor Gray
}

# =============================================================================
# Check 5: Go tests (quick run without race detector for speed)
# =============================================================================
Write-Host ""
Write-Host "Running Go tests..." -ForegroundColor White

$testOutput = go test -short ./... 2>&1
$testExitCode = $LASTEXITCODE

if ($testExitCode -ne 0) {
    Write-Host "X Tests failed:" -ForegroundColor Red
    Write-Host $testOutput -ForegroundColor Red
    $script:Failed = $true
}
else {
    Write-Host "[OK] Tests passed" -ForegroundColor Green
}

# =============================================================================
# Check 6: Check for common mistakes
# =============================================================================
Write-Host ""
Write-Host "Checking for common mistakes..." -ForegroundColor White

if ($StagedFiles) {
    # Check for fmt.Println debugging statements in non-test files
    $NonTestFiles = $StagedFiles | Where-Object { $_ -notlike "*_test.go" }
    foreach ($file in $NonTestFiles) {
        if (Test-Path $file) {
            $content = Get-Content $file -Raw -ErrorAction SilentlyContinue
            if ($content -and $content -match 'fmt\.Println') {
                Write-Host "[WARN] Found fmt.Println in $file (possible debug statement)" -ForegroundColor Yellow
            }
        }
    }
    
    # Check for TODO/FIXME comments (informational only)
    $TodoFound = $false
    foreach ($file in $StagedFiles) {
        if (Test-Path $file) {
            $todoMatches = Select-String -Path $file -Pattern 'TODO|FIXME' -ErrorAction SilentlyContinue
            if ($todoMatches) {
                if (-not $TodoFound) {
                    Write-Host "[INFO] Found TODO/FIXME comments (informational):" -ForegroundColor Yellow
                    $TodoFound = $true
                }
                $todoMatches | ForEach-Object { 
                    Write-Host "  $($_.Filename):$($_.LineNumber): $($_.Line.Trim())" -ForegroundColor Gray 
                }
            }
        }
    }
}

Write-Host "[OK] Common mistakes check passed" -ForegroundColor Green

# =============================================================================
# Final result
# =============================================================================
Write-Host ""
if ($script:Failed) {
    Write-Host "================================================================" -ForegroundColor Red
    Write-Host "Pre-commit checks FAILED. Please fix the issues above." -ForegroundColor Red
    Write-Host "================================================================" -ForegroundColor Red
    Write-Host ""
    Write-Host "To bypass this hook (not recommended), use: git commit --no-verify" -ForegroundColor Gray
    exit 1
}
else {
    Write-Host "================================================================" -ForegroundColor Green
    Write-Host "All pre-commit checks passed!" -ForegroundColor Green
    Write-Host "================================================================" -ForegroundColor Green
    exit 0
}
