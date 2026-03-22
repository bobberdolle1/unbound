#!/usr/bin/env pwsh
# E2E Test Runner with Admin Privileges
# Must be run as Administrator

$ErrorActionPreference = "Stop"

Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host "🎯 E2E BYPASS MATRIX TEST (ADMIN MODE)" -ForegroundColor Cyan
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host ""

# Check admin privileges
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if (-not $isAdmin) {
    Write-Host "❌ This script must be run as Administrator" -ForegroundColor Red
    Write-Host "   Right-click PowerShell and select 'Run as Administrator'" -ForegroundColor Yellow
    exit 1
}

Write-Host "✅ Running with Administrator privileges" -ForegroundColor Green
Write-Host ""

# Run E2E test
try {
    go test -v -timeout 5m ./tests/e2e_bypass_matrix_test.go
    if ($LASTEXITCODE -ne 0) {
        Write-Host ""
        Write-Host "❌ E2E Bypass Matrix test failed" -ForegroundColor Red
        exit 1
    }
    Write-Host ""
    Write-Host "✅ E2E Bypass Matrix test passed" -ForegroundColor Green
} catch {
    Write-Host ""
    Write-Host "❌ E2E test failed: $_" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host "✅ E2E TEST COMPLETED" -ForegroundColor Green
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
