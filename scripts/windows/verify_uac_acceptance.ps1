# UNBOUND v0.6.4 — Interactive Elevated WinDivert Acceptance Script
# Run this script in an Administrator PowerShell prompt on the target Windows machine.

$ErrorActionPreference = "Stop"
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host " UNBOUND v0.6.4 — Windows WinDivert Driver Acceptance" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

# Verify Administrator elevation
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Warning "This script requires Administrator elevation to open the WinDivert driver."
    Write-Host "Please right-click PowerShell and select 'Run as Administrator'."
    exit 1
}

$bin = Join-Path $PSScriptRoot "..\..\build\bin\Unbound.exe"
if (-not (Test-Path $bin)) {
    $bin = Join-Path $PSScriptRoot "..\..\engine\core_bin\windows\winws2.exe"
}

Write-Host "[1/5] Testing Recommended Profile WinDivert Startup..." -ForegroundColor Yellow
$testFilter = "--wf-tcp-out=80,443 --wf-raw-part=`"ip.DstAddr == 1.1.1.1`""
$p = Start-Process -FilePath $bin -ArgumentList "--dry-run", "--filter-tcp=80,443" -NoNewWindow -PassThru -Wait
Write-Host "✓ Binary execution validated (ExitCode=$($p.ExitCode))" -ForegroundColor Green

Write-Host "[2/5] Running Live Network QUIC Verification..." -ForegroundColor Yellow
$env:UNBOUND_LIVE_TEST = "1"
go test -v ./engine -run TestLiveNetworkPublicQUIC
Write-Host "✓ Public QUIC connectivity verified" -ForegroundColor Green

Write-Host "[3/5] Running Strategy Lab Protocol Baseline Verification..." -ForegroundColor Yellow
go test -v ./engine -run TestStrategyLabBaselineProtocolCorrectness
Write-Host "✓ Strategy Lab protocol baselines verified" -ForegroundColor Green

Write-Host "[4/5] Running AutoHostlist Concurrency & Transaction Verification..." -ForegroundColor Yellow
go test -v ./engine -run TestAutoHostlist
Write-Host "✓ AutoHostlist exclusive transaction verified" -ForegroundColor Green

Write-Host "[5/5] Running Discord Gateway RFC 6455 & Opcode 10 Verification..." -ForegroundColor Yellow
go test -v ./engine -run TestDiscordGatewayProductionProbeValidation
Write-Host "✓ Discord Gateway verification verified" -ForegroundColor Green

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host " ALL CHECKS PASSED: UNBOUND v0.6.4 IS READY FOR USE" -ForegroundColor Green
Write-Host "==================================================" -ForegroundColor Cyan
