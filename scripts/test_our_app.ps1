#!/usr/bin/env pwsh
#Requires -RunAsAdministrator

$ErrorActionPreference = "Stop"

Write-Host "=== TESTING UNBOUND APP WITH NEW PROFILES ===" -ForegroundColor Cyan
Write-Host ""

Write-Host "Step 1: Building app..." -ForegroundColor Yellow
& wails build -clean

if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "Step 2: Starting app..." -ForegroundColor Yellow
$app = Start-Process -FilePath "build\bin\unbound.exe" -PassThru

Write-Host "App started (PID: $($app.Id))" -ForegroundColor Green
Write-Host ""
Write-Host "Waiting 10 seconds for manual testing..." -ForegroundColor Yellow
Write-Host "Test the following:" -ForegroundColor Cyan
Write-Host "  1. Check profile dropdown shows new profiles" -ForegroundColor White
Write-Host "  2. Select 'General FAKE TLS AUTO (Verified 100%)'" -ForegroundColor White
Write-Host "  3. Click CONNECT" -ForegroundColor White
Write-Host "  4. Open YouTube/Discord in browser" -ForegroundColor White
Write-Host ""

Start-Sleep -Seconds 10

Write-Host "Testing URLs..." -ForegroundColor Yellow

$testUrls = @(
    "https://www.youtube.com",
    "https://discord.com",
    "https://www.google.com"
)

foreach ($url in $testUrls) {
    try {
        $start = Get-Date
        $response = Invoke-WebRequest -Uri $url -TimeoutSec 10 -UseBasicParsing
        $elapsed = [math]::Round(((Get-Date) - $start).TotalMilliseconds, 0)
        
        if ($response.StatusCode -eq 200) {
            Write-Host "[OK] $url - ${elapsed}ms" -ForegroundColor Green
        } else {
            Write-Host "[FAIL] $url - HTTP $($response.StatusCode)" -ForegroundColor Red
        }
    } catch {
        Write-Host "[FAIL] $url - $($_.Exception.Message)" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "Press Enter to stop app..."
Read-Host

Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue
Write-Host "App stopped" -ForegroundColor Yellow
