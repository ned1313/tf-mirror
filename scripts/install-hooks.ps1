#Requires -Version 5.1
<#
.SYNOPSIS
    Install the pre-commit hook for terraform-mirror

.DESCRIPTION
    This script installs the git pre-commit hook that checks for common issues
    that will be caught by the CI pipeline.

.EXAMPLE
    .\scripts\install-hooks.ps1
#>

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir
$HooksDir = Join-Path $RepoRoot ".git\hooks"

Write-Host "Installing git hooks for terraform-mirror..." -ForegroundColor Cyan

# Ensure .git/hooks directory exists
if (-not (Test-Path $HooksDir)) {
    Write-Host "Error: .git\hooks directory not found. Are you in a git repository?" -ForegroundColor Red
    exit 1
}

# Copy pre-commit.ps1
$SourceHook = Join-Path $ScriptDir "pre-commit.ps1"
$DestHook = Join-Path $HooksDir "pre-commit.ps1"
Copy-Item $SourceHook $DestHook -Force

# Create wrapper script for Git to call PowerShell
$WrapperScript = @'
#!/bin/sh
# Git hook wrapper - calls the PowerShell pre-commit script
powershell.exe -ExecutionPolicy Bypass -NoProfile -File ".git/hooks/pre-commit.ps1"
exit $?
'@

$WrapperPath = Join-Path $HooksDir "pre-commit"
$WrapperScript | Out-File -FilePath $WrapperPath -Encoding ascii -NoNewline

Write-Host ""
Write-Host "✓ Pre-commit hook installed successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "The hook will run automatically before each commit."
Write-Host "To skip the hook (not recommended), use: git commit --no-verify" -ForegroundColor Gray
Write-Host ""
Write-Host "Prerequisites:" -ForegroundColor Yellow
Write-Host "  - Go (required)"
Write-Host "  - golangci-lint (recommended):"
Write-Host "    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" -ForegroundColor Gray
