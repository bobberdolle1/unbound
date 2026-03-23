#!/usr/bin/env pwsh
#Requires -RunAsAdministrator

$ErrorActionPreference = "Continue"

$REFERENCE_DIR = "F:\Projects\Unbound\reference\zapret2-youtube-discord-main"
$WINWS_PATH = Join-Path $REFERENCE_DIR "exe\winws2.exe"
$PRESET_PATH = Join-Path $REFERENCE_DIR "presets\general ALT2.txt"

Write-Host "=== DPI BYPASS COMPARISON TEST ===" -ForegroundColor Cyan
Write-Host ""

$testUrls = @(
    @{URL="https://www.youtube.com"; Name="YouTube"},
    @{URL="https://discord.com/api/v9/gateway"; Name="Discord API"},
    @{URL="https://www.google.com"; Name="Google"}
)

Write-Host "Phase 1: Testing WITHOUT bypass..." -ForegroundColor Yellow
Write-Host ""

$withoutBypass = @()
foreach ($test in $testUrls) {
    try {
        $start = Get-Date
        $response = Invoke-WebRequest -Uri $test.URL -TimeoutSec 5 -UseBasicParsing -ErrorAction Stop
        $elapsed = [math]::Round(((Get-Date) - $start).TotalMilliseconds, 0)
        
        Write-Host "  [$($test.Name)] OK - ${elapsed}ms" -ForegroundColor Green
        $withoutBypass += @{Name=$test.Name; Status="OK"; Latency=$elapsed}
    } catch {
        $error = $_.Exception.Message
        Write-Host "  [$($test.Name)] BLOCKED - $error" -ForegroundColor Red
        $withoutBypass += @{Name=$test.Name; Status="BLOCKED"; Error=$error}
    }
}

Write-Host ""
Write-Host "Phase 2: Starting bypass engine..." -ForegroundColor Yellow

$args = Get-Content $PRESET_PATH | Where-Object {$_ -notmatch "^#" -and $_.Trim() -ne ""}
$process = Start-Process -FilePath $WINWS_PATH -ArgumentList $args -WorkingDirectory $REFERENCE_DIR -PassThru

Write-Host "Engine PID: $($process.Id)" -ForegroundColor Green
Start-Sleep -Seconds 5

Write-Host ""
Write-Host "Phase 3: Testing WITH bypass..." -ForegroundColor Yellow
Write-Host ""

$withBypass = @()
foreach ($test in $testUrls) {
    try {
        $start = Get-Date
        $response = Invoke-WebRequest -Uri $test.URL -TimeoutSec 8 -UseBasicParsing -ErrorAction Stop
        $elapsed = [math]::Round(((Get-Date) - $start).TotalMilliseconds, 0)
        
        Write-Host "  [$($test.Name)] OK - ${elapsed}ms" -ForegroundColor Green
        $withBypass += @{Name=$test.Name; Status="OK"; Latency=$elapsed}
    } catch {
        $error = $_.Exception.Message
        Write-Host "  [$($test.Name)] FAILED - $error" -ForegroundColor Red
        $withBypass += @{Name=$test.Name; Status="FAILED"; Error=$error}
    }
}

Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "COMPARISON RESULTS" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

for ($i = 0; $i -lt $testUrls.Count; $i++) {
    $name = $testUrls[$i].Name
    $before = $withoutBypass[$i].Status
    $after = $withBypass[$i].Status
    
    Write-Host ""
    Write-Host "$name" -ForegroundColor White
    Write-Host "  Without: $before" -ForegroundColor $(if($before -eq "OK"){"Green"}else{"Red"})
    Write-Host "  With:    $after" -ForegroundColor $(if($after -eq "OK"){"Green"}else{"Red"})
    
    if ($before -eq "BLOCKED" -and $after -eq "OK") {
        Write-Host "  BYPASS WORKS!" -ForegroundColor Green
    } elseif ($before -eq "OK" -and $after -eq "OK") {
        Write-Host "  No DPI detected (both work)" -ForegroundColor Yellow
    } else {
        Write-Host "  BYPASS FAILED" -ForegroundColor Red
    }
}
