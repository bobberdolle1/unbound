# DPI Bypass Testing Script
# Tests actual bypass functionality, not just URL accessibility

param(
    [string]$ProfileName = "general ALT2",
    [int]$TestDuration = 15
)

$ErrorActionPreference = "Stop"

Write-Host "=== DPI Bypass Test ===" -ForegroundColor Cyan
Write-Host "Profile: $ProfileName" -ForegroundColor Yellow
Write-Host "Test Duration: ${TestDuration}s" -ForegroundColor Yellow
Write-Host ""

# Get paths
$projectRoot = Split-Path -Parent $PSScriptRoot
$winwsPath = Join-Path $projectRoot "engine\core_bin\winws2.exe"
$presetPath = Join-Path $projectRoot "reference\zapret2-youtube-discord-main\presets\$ProfileName.txt"

if (-not (Test-Path $winwsPath)) {
    Write-Host "ERROR: winws2.exe not found at $winwsPath" -ForegroundColor Red
    exit 1
}

if (-not (Test-Path $presetPath)) {
    Write-Host "ERROR: Preset not found at $presetPath" -ForegroundColor Red
    exit 1
}

# Check admin rights
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "ERROR: Administrator rights required" -ForegroundColor Red
    exit 1
}

# Kill existing winws2 processes
Write-Host "Stopping existing winws2 processes..." -ForegroundColor Yellow
Get-Process -Name "winws2" -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 1

# Read preset file and build command
$presetContent = Get-Content $presetPath
$args = @()
$refDir = Join-Path $projectRoot "reference\zapret2-youtube-discord-main"

foreach ($line in $presetContent) {
    $line = $line.Trim()
    if ($line -and -not $line.StartsWith("#")) {
        # Replace relative paths with absolute paths
        $line = $line -replace '@lua/', "@$refDir/lua/"
        $line = $line -replace '@bin/', "@$refDir/bin/"
        $line = $line -replace '@windivert\.filter/', "@$refDir/windivert.filter/"
        $line = $line -replace 'lists/', "$refDir/lists/"
        $args += $line
    }
}

Write-Host "Starting winws2 with profile: $ProfileName" -ForegroundColor Green
Write-Host "Command: $winwsPath $($args -join ' ')" -ForegroundColor DarkGray
Write-Host ""

# Start winws2 process
$process = Start-Process -FilePath $winwsPath -ArgumentList $args -WorkingDirectory (Split-Path $winwsPath) -PassThru -WindowStyle Hidden

Write-Host "Process started (PID: $($process.Id))" -ForegroundColor Green
Write-Host "Waiting ${TestDuration}s for engine to initialize..." -ForegroundColor Yellow
Start-Sleep -Seconds 5

# Test YouTube
Write-Host ""
Write-Host "=== Testing YouTube ===" -ForegroundColor Cyan
try {
    $ytResponse = Invoke-WebRequest -Uri "https://www.youtube.com" -TimeoutSec 10 -UseBasicParsing
    if ($ytResponse.StatusCode -eq 200) {
        Write-Host "[PASS] YouTube accessible (HTTP 200)" -ForegroundColor Green
    } else {
        Write-Host "[FAIL] YouTube returned HTTP $($ytResponse.StatusCode)" -ForegroundColor Red
    }
} catch {
    Write-Host "[FAIL] YouTube test failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test Discord API
Write-Host ""
Write-Host "=== Testing Discord API ===" -ForegroundColor Cyan
try {
    $discordResponse = Invoke-WebRequest -Uri "https://discord.com/api/v10/gateway" -TimeoutSec 10 -UseBasicParsing
    if ($discordResponse.StatusCode -eq 200) {
        Write-Host "[PASS] Discord API accessible (HTTP 200)" -ForegroundColor Green
        $json = $discordResponse.Content | ConvertFrom-Json
        Write-Host "  Gateway URL: $($json.url)" -ForegroundColor DarkGray
    } else {
        Write-Host "[FAIL] Discord API returned HTTP $($discordResponse.StatusCode)" -ForegroundColor Red
    }
} catch {
    Write-Host "[FAIL] Discord API test failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test Discord CDN
Write-Host ""
Write-Host "=== Testing Discord CDN ===" -ForegroundColor Cyan
try {
    $cdnResponse = Invoke-WebRequest -Uri "https://cdn.discordapp.com/embed/avatars/0.png" -TimeoutSec 10 -UseBasicParsing
    if ($cdnResponse.StatusCode -eq 200) {
        Write-Host "[PASS] Discord CDN accessible (HTTP 200)" -ForegroundColor Green
    } else {
        Write-Host "[FAIL] Discord CDN returned HTTP $($cdnResponse.StatusCode)" -ForegroundColor Red
    }
} catch {
    Write-Host "[FAIL] Discord CDN test failed: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""
Write-Host "Test completed. Stopping winws2..." -ForegroundColor Yellow
Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

Write-Host "Done." -ForegroundColor Green
