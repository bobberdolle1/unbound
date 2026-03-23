#!/usr/bin/env pwsh
#Requires -RunAsAdministrator

$ErrorActionPreference = "Stop"

$REFERENCE_DIR = "F:\Projects\Unbound\reference\zapret2-youtube-discord-main"
$WINWS_PATH = Join-Path $REFERENCE_DIR "exe\winws2.exe"
$PRESET_DIR = Join-Path $REFERENCE_DIR "presets"

Write-Host "=== ZAPRET2 REFERENCE PROFILE TESTER ===" -ForegroundColor Cyan
Write-Host ""

if (-not (Test-Path $WINWS_PATH)) {
    Write-Host "ERROR: winws2.exe not found at $WINWS_PATH" -ForegroundColor Red
    exit 1
}

$profiles = @(
    "Default.txt",
    "general ALT.txt",
    "general FAKE TLS AUTO.txt",
    "general ALT2.txt"
)

Write-Host "Available profiles:" -ForegroundColor Yellow
for ($i = 0; $i -lt $profiles.Length; $i++) {
    Write-Host "  [$i] $($profiles[$i])"
}
Write-Host ""

$selection = Read-Host "Select profile number (0-$($profiles.Length-1))"
$selectedProfile = $profiles[[int]$selection]
$presetPath = Join-Path $PRESET_DIR $selectedProfile

if (-not (Test-Path $presetPath)) {
    Write-Host "ERROR: Preset not found: $presetPath" -ForegroundColor Red
    exit 1
}

Write-Host "Loading preset: $selectedProfile" -ForegroundColor Green
Write-Host ""

$args = Get-Content $presetPath | Where-Object {$_ -notmatch "^#" -and $_.Trim() -ne ""}

Write-Host "Starting winws2.exe with $($args.Count) arguments..." -ForegroundColor Yellow
Write-Host ""

$process = Start-Process -FilePath $WINWS_PATH -ArgumentList $args -WorkingDirectory $REFERENCE_DIR -PassThru -NoNewWindow

Write-Host "Process started (PID: $($process.Id))" -ForegroundColor Green
Write-Host ""
Write-Host "Testing URLs..." -ForegroundColor Yellow
Write-Host ""

Start-Sleep -Seconds 3

$testUrls = @(
    "https://www.youtube.com",
    "https://discord.com",
    "https://www.google.com"
)

foreach ($url in $testUrls) {
    try {
        $start = Get-Date
        $response = Invoke-WebRequest -Uri $url -TimeoutSec 10 -UseBasicParsing
        $elapsed = ((Get-Date) - $start).TotalMilliseconds
        
        if ($response.StatusCode -eq 200) {
            Write-Host "[OK] $url - ${elapsed}ms" -ForegroundColor Green
        } else {
            Write-Host "[FAIL] $url - Status: $($response.StatusCode)" -ForegroundColor Red
        }
    } catch {
        Write-Host "[FAIL] $url - Error: $($_.Exception.Message)" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "Press Enter to stop winws2.exe..."
Read-Host

Stop-Process -Id $process.Id -Force
Write-Host "Process stopped" -ForegroundColor Yellow
