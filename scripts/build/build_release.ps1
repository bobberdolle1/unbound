#!/usr/bin/env pwsh
#Requires -Version 5.1

$ErrorActionPreference = "Stop"

$VERSION = "1.0.4"
$PROJECT_ROOT = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$BUILD_DIR    = Join-Path $PROJECT_ROOT "build\bin"
$DIST_DIR     = Join-Path $PROJECT_ROOT "dist"
$RELEASE_NAME = "unbound-v$VERSION-win64"
$RELEASE_DIR  = Join-Path $DIST_DIR $RELEASE_NAME

Write-Host ""
Write-Host "=================================================" -ForegroundColor Cyan
Write-Host "UNBOUND v$VERSION - Release Build"                 -ForegroundColor Cyan
Write-Host "=================================================" -ForegroundColor Cyan
Write-Host ""

function Test-Command { param([string]$Command)
    return [bool](Get-Command $Command -ErrorAction SilentlyContinue)
}

Write-Host "[1/3] Checking tools..." -ForegroundColor Yellow
foreach ($tool in @("wails","go")) {
    if (-not (Test-Command $tool)) {
        Write-Host "ERROR: $tool not found." -ForegroundColor Red
        exit 1
    }
}
Write-Host "OK: Wails and Go present." -ForegroundColor Green
Write-Host ""

Write-Host "[2/3] Building via Wails..." -ForegroundColor Yellow
Push-Location $PROJECT_ROOT
try {
    $goExe = (Get-Command go).Source
    & wails build -clean -compiler="$goExe" -o unbound.exe
    if ($LASTEXITCODE -ne 0) { throw "wails build failed." }
    Write-Host "OK: Wails build completed." -ForegroundColor Green
} finally {
    Pop-Location
}

$builtExe = Join-Path $BUILD_DIR "unbound.exe"
if (-not (Test-Path $builtExe)) {
    Write-Host "ERROR: Binary not found." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "[3/3] Creating ZIP archive..." -ForegroundColor Yellow

if (Test-Path $RELEASE_DIR) { Remove-Item $RELEASE_DIR -Recurse -Force }
New-Item -ItemType Directory -Path $RELEASE_DIR -Force | Out-Null
if (-not (Test-Path $DIST_DIR)) { New-Item -ItemType Directory -Path $DIST_DIR -Force | Out-Null }

Copy-Item $builtExe (Join-Path $RELEASE_DIR "unbound.exe")
Copy-Item (Join-Path $PROJECT_ROOT "README_RELEASE.txt") (Join-Path $RELEASE_DIR "README.txt")

$zipPath = Join-Path $DIST_DIR "$RELEASE_NAME.zip"
if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
Compress-Archive -Path "$RELEASE_DIR\*" -DestinationPath $zipPath -CompressionLevel Optimal

$zipMB = [math]::Round((Get-Item $zipPath).Length / 1MB, 1)

Write-Host ""
Write-Host "=================================================" -ForegroundColor Green
Write-Host "RELEASE BUILT SUCCESSFULLY"                        -ForegroundColor Green
Write-Host "=================================================" -ForegroundColor Green
Write-Host ""
Write-Host "Archive: $zipPath" -ForegroundColor Cyan
Write-Host "Size: $zipMB MB" -ForegroundColor Cyan
Write-Host "Version: $VERSION" -ForegroundColor Cyan
Write-Host ""
Write-Host "Ready." -ForegroundColor Green
Write-Host ""
