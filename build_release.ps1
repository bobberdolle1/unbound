#!/usr/bin/env pwsh
#Requires -Version 5.1

$ErrorActionPreference = "Stop"

$VERSION = "1.0.0"
$PROJECT_ROOT = $PSScriptRoot
$DIST_DIR = Join-Path $PROJECT_ROOT "dist"
$BUILD_DIR = Join-Path $PROJECT_ROOT "build"
$INSTALLER_ISS = Join-Path $BUILD_DIR "windows\installer\installer.iss"
$OUTPUT_INSTALLER = Join-Path $DIST_DIR "Unbound-Setup-v$VERSION.exe"

Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host "🚀 UNBOUND v$VERSION - RELEASE BUILD PIPELINE" -ForegroundColor Cyan
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host ""

function Test-Command {
    param([string]$Command)
    try {
        if (Get-Command $Command -ErrorAction Stop) { return $true }
    } catch {
        return $false
    }
}

function Find-InnoSetup {
    $possiblePaths = @(
        "C:\Program Files (x86)\Inno Setup 6\ISCC.exe",
        "C:\Program Files\Inno Setup 6\ISCC.exe",
        "C:\Program Files (x86)\Inno Setup 5\ISCC.exe",
        "C:\Program Files\Inno Setup 5\ISCC.exe"
    )
    
    foreach ($path in $possiblePaths) {
        if (Test-Path $path) {
            return $path
        }
    }
    
    return $null
}

Write-Host "📦 Step 1/4: Checking prerequisites..." -ForegroundColor Yellow

if (-not (Test-Command "wails")) {
    Write-Host "❌ Wails CLI not found. Install: go install github.com/wailsapp/wails/v2/cmd/wails@latest" -ForegroundColor Red
    exit 1
}

if (-not (Test-Command "go")) {
    Write-Host "❌ Go compiler not found. Install from https://go.dev/dl/" -ForegroundColor Red
    exit 1
}

Write-Host "✅ Wails CLI: $(wails version)" -ForegroundColor Green
Write-Host "✅ Go compiler: $(go version)" -ForegroundColor Green
Write-Host ""

Write-Host "🔨 Step 2/4: Building Unbound UI with Wails..." -ForegroundColor Yellow

$goCompiler = "C:\Program Files\Go\bin\go.exe"
if (-not (Test-Path $goCompiler)) {
    $goCompiler = (Get-Command go).Source
}

try {
    & wails build -clean -compiler="$goCompiler"
    if ($LASTEXITCODE -ne 0) {
        throw "Wails build failed with exit code $LASTEXITCODE"
    }
    Write-Host "✅ Wails build completed successfully" -ForegroundColor Green
} catch {
    Write-Host "❌ Wails build failed: $_" -ForegroundColor Red
    exit 1
}

$builtExe = Join-Path $BUILD_DIR "bin\unbound.exe"
if (-not (Test-Path $builtExe)) {
    Write-Host "❌ Built executable not found at: $builtExe" -ForegroundColor Red
    exit 1
}

Write-Host "✅ Binary located: $builtExe" -ForegroundColor Green
Write-Host ""

Write-Host "🔍 Step 3/4: Locating Inno Setup compiler..." -ForegroundColor Yellow

$iscc = Find-InnoSetup

if (-not $iscc) {
    Write-Host "❌ Inno Setup not found. Please install manually from: https://jrsoftware.org/isdl.php" -ForegroundColor Red
    Write-Host "After installation, run this script again." -ForegroundColor Yellow
    exit 1
} else {
    Write-Host "✅ Inno Setup found: $iscc" -ForegroundColor Green
}

Write-Host ""

Write-Host "📦 Step 4/4: Compiling installer..." -ForegroundColor Yellow

if (-not (Test-Path $INSTALLER_ISS)) {
    Write-Host "❌ Installer script not found: $INSTALLER_ISS" -ForegroundColor Red
    exit 1
}

if (-not (Test-Path $DIST_DIR)) {
    New-Item -ItemType Directory -Path $DIST_DIR -Force | Out-Null
}

try {
    & $iscc $INSTALLER_ISS
    if ($LASTEXITCODE -ne 0) {
        throw "Inno Setup compilation failed with exit code $LASTEXITCODE"
    }
    
    if (-not (Test-Path $OUTPUT_INSTALLER)) {
        throw "Installer not found at expected location: $OUTPUT_INSTALLER"
    }
    
    $installerSize = (Get-Item $OUTPUT_INSTALLER).Length / 1MB
    
    Write-Host ""
    Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
    Write-Host "✅ RELEASE BUILD SUCCESSFUL" -ForegroundColor Green
    Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
    Write-Host ""
    Write-Host "📦 Installer: $OUTPUT_INSTALLER" -ForegroundColor Cyan
    Write-Host "📊 Size: $([math]::Round($installerSize, 2)) MB" -ForegroundColor Cyan
    Write-Host "🎯 Version: $VERSION" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "🚀 Ready for distribution!" -ForegroundColor Green
    Write-Host ""
    
} catch {
    Write-Host "❌ Installer compilation failed: $_" -ForegroundColor Red
    exit 1
}
