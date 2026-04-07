# ============================================================================
# UNBOUND — Master Build Script (PowerShell / Windows)
# ============================================================================
# Usage:
#   .\build_all.ps1 <target> [options]
#
# Targets:
#   windows          - Build Windows GUI binary via Wails
#   linux-docker     - Build Linux binary via Docker cross-compilation
#   openwrt-docker   - Build OpenWrt IPK via Docker
#   android          - Build Android APK via Gradle
#   all              - Build all available targets
#
# Options:
#   -Debug           - Enable debug build mode
#   -Clean           - Clean build artifacts before building
#   -Version <ver>   - Override version string
#   -Help            - Show help message
#
# Examples:
#   .\build_all.ps1 windows
#   .\build_all.ps1 linux-docker -Debug
#   .\build_all.ps1 all -Clean -Version 1.0.5
# ============================================================================

[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [ValidateSet('windows', 'linux-docker', 'openwrt-docker', 'android', 'decky', 'magisk', 'all')]
    [string]$Target = '',

    [switch]$Debug,
    [switch]$Clean,
    [string]$Version,
    [switch]$Help
)

# ── Colors ───────────────────────────────────────────────────────────────────
function Write-Info  { Write-Host "[INFO] $args" -ForegroundColor Cyan }
function Write-Ok    { Write-Host "[OK] $args" -ForegroundColor Green }
function Write-Warn  { Write-Host "[WARN] $args" -ForegroundColor Yellow }
function Write-Err   { Write-Host "[ERROR] $args" -ForegroundColor Red }
function Write-Step  { Write-Host "`n━━━ $args ━━━" -ForegroundColor White -BackgroundColor DarkCyan }

# ── Globals ──────────────────────────────────────────────────────────────────
$ScriptDir    = $PSScriptRoot
$ProjectRoot  = $ScriptDir
$BuildDir     = Join-Path $ProjectRoot "build"
$DistDir      = Join-Path $ProjectRoot "dist"
$DebugMode    = $Debug.IsPresent
$CleanBuild   = $Clean.IsPresent

# ── Help ─────────────────────────────────────────────────────────────────────
if ($Help -or -not $Target) {
    Get-Content "$ScriptDir\build_all.ps1" | Select-String '^#' | ForEach-Object { $_ -replace '^#\s?' }
    exit 0
}

# ── Version resolution ───────────────────────────────────────────────────────
function Resolve-Version {
    if ($Version) { return $Version }
    if (Test-Path "$ProjectRoot\wails.json") {
        $wails = Get-Content "$ProjectRoot\wails.json" -Raw | ConvertFrom-Json
        return $wails.info.productVersion ?? "0.0.0"
    }
    return "0.0.0"
}

# ── Tool check ───────────────────────────────────────────────────────────────
function Require-Command($Name, $InstallHint) {
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        Write-Err "$Name not found in PATH."
        Write-Info "Install: $InstallHint"
        throw "Missing prerequisite: $Name"
    }
}

# ── Clean ────────────────────────────────────────────────────────────────────
function Do-Clean {
    Write-Step "Cleaning build artifacts"
    Remove-Item -Recurse -Force "$BuildDir\bin"        -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force "$BuildDir\bin-linux"  -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force "$BuildDir\bin-darwin" -ErrorAction SilentlyContinue
    Remove-Item          "$DistDir\*.zip"              -ErrorAction SilentlyContinue
    Remove-Item          "$DistDir\*.apk"              -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force "$ProjectRoot\frontend\dist"            -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force "$ProjectRoot\frontend\node_modules\.cache" -ErrorAction SilentlyContinue
    Write-Ok "Clean complete"
}

# ── Frontend ─────────────────────────────────────────────────────────────────
function Build-Frontend {
    Write-Step "Building frontend assets"
    if (Test-Path "$ProjectRoot\frontend") {
        Push-Location "$ProjectRoot\frontend"
        npm install --include=dev
        npm run build
        Pop-Location
        Write-Ok "Frontend built"
    } else {
        Write-Warn "frontend/ not found, skipping"
    }
}

# ── Windows ──────────────────────────────────────────────────────────────────
function Build-Windows {
    $ver = Resolve-Version
    Write-Step "Building Windows binary (wails)"
    Require-Command "wails" "go install github.com/wailsapp/wails/v2/cmd/wails@latest"
    Require-Command "go"    "https://go.dev/dl/"

    Build-Frontend

    $debugFlag = ""
    if ($DebugMode) { $debugFlag = "-debug" }

    Push-Location $ProjectRoot
    wails build -clean -o unbound.exe $debugFlag
    Pop-Location

    $outDir  = Join-Path $DistDir "unbound-v$ver-win64"
    New-Item -ItemType Directory -Force -Path $outDir | Out-Null

    $builtExe = Join-Path "$BuildDir\bin" "unbound.exe"
    if (-not (Test-Path $builtExe)) {
        $builtExe = Join-Path "$BuildDir\bin" "unbound\unbound.exe"
    }

    if (Test-Path $builtExe) {
        Copy-Item $builtExe $outDir -Force
        Write-Ok "Windows binary: $outDir\unbound.exe"
    } else {
        Write-Err "Built binary not found"
    }
}

# ── Linux via Docker ─────────────────────────────────────────────────────────
function Build-LinuxDocker {
    $ver = Resolve-Version
    Write-Step "Building Linux binary (Docker)"
    Require-Command "docker" "https://www.docker.com/products/docker-desktop/"

    $imageTag = "unbound-linux-builder"
    $debugFlag = ""
    if ($DebugMode) { $debugFlag = "--build-arg DEBUG=1" }

    docker build $debugFlag `
        --build-arg VERSION=$ver `
        -t $imageTag `
        -f "$ScriptDir\scripts\docker\Dockerfile.linux" `
        $ProjectRoot

    # Extract binary from container
    $outDir = Join-Path "$BuildDir" "bin-linux"
    New-Item -ItemType Directory -Force -Path $outDir | Out-Null

    $containerId = docker create $imageTag
    docker cp "$containerId:/app/build/bin/unbound-linux" "$outDir\unbound-linux"
    docker rm $containerId | Out-Null

    Write-Ok "Linux binary: $outDir\unbound-linux"
}

# ── OpenWrt via Docker ───────────────────────────────────────────────────────
function Build-OpenWrtDocker {
    $ver = Resolve-Version
    Write-Step "Building OpenWrt IPK (Docker)"
    Require-Command "docker" "https://www.docker.com/products/docker-desktop/"

    $imageTag = "unbound-openwrt-builder"

    docker build `
        --build-arg VERSION=$ver `
        -t $imageTag `
        -f "$ScriptDir\scripts\docker\Dockerfile.openwrt" `
        "$ProjectRoot\openwrt\unbound-wrt"

    $outDir = Join-Path $DistDir "openwrt"
    New-Item -ItemType Directory -Force -Path $outDir | Out-Null

    $containerId = docker create $imageTag
    docker cp "$containerId:/builder/bin/packages/" "$outDir\" 2>$null
    docker rm $containerId | Out-Null

    Write-Ok "OpenWrt packages: $outDir\"
}

# ── Android ──────────────────────────────────────────────────────────────────
function Build-Android {
    $ver = Resolve-Version
    Write-Step "Building Android APK"

    if (-not (Test-Path "$ProjectRoot\android")) {
        Write-Warn "android/ not found, skipping"
        return
    }

    Require-Command "gradle" "https://gradle.org/install/ (or use gradlew wrapper)"

    Push-Location "$ProjectRoot\android"
    $variant = "assembleRelease"
    if ($DebugMode) { $variant = "assembleDebug" }
    gradle $variant
    Pop-Location

    New-Item -ItemType Directory -Force -Path $DistDir | Out-Null
    Get-ChildItem -Recurse "$ProjectRoot\android\**\*.apk" | ForEach-Object {
        Copy-Item $_.FullName $DistDir -Force
        Write-Info "Copied: $($_.Name)"
    }

    Write-Ok "Android APK(s) in: $DistDir\"
}

# ── Decky Plugin ─────────────────────────────────────────────────────────────
function Build-Decky {
    Write-Step "Building Decky Loader plugin"
    if (-not (Test-Path "$ProjectRoot\decky-plugin")) {
        Write-Warn "decky-plugin/ not found, skipping"
        return
    }

    Require-Command "npm" "https://nodejs.org/"

    Push-Location "$ProjectRoot\decky-plugin"
    npm install
    npm run build 2>$null
    Pop-Location

    New-Item -ItemType Directory -Force -Path $DistDir | Out-Null
    Write-Ok "Decky plugin built"
}

# ── Magisk Module ────────────────────────────────────────────────────────────
function Build-Magisk {
    $ver = Resolve-Version
    Write-Step "Building Magisk module"

    if (-not (Test-Path "$ProjectRoot\magisk-module")) {
        Write-Warn "magisk-module/ not found, skipping"
        return
    }

    New-Item -ItemType Directory -Force -Path $DistDir | Out-Null
    $zipPath = Join-Path $DistDir "unbound-magisk-v$ver.zip"

    Push-Location "$ProjectRoot\magisk-module"
    if (Get-Command "Compress-Archive" -ErrorAction SilentlyContinue) {
        # PowerShell native
        Get-ChildItem -Path . -Recurse | Compress-Archive -DestinationPath $zipPath -Force
    } else {
        Write-Warn "Compress-Archive not available, create zip manually"
    }
    Pop-Location

    Write-Ok "Magisk module: $zipPath"
}

# ── All targets ──────────────────────────────────────────────────────────────
function Build-All {
    Write-Step "Building ALL targets"
    Build-Windows
    Build-LinuxDocker
    Build-OpenWrtDocker
    Build-Android
    Build-Decky
    Build-Magisk
    Write-Ok "All builds complete"
}

# ── Main ─────────────────────────────────────────────────────────────────────
if ($CleanBuild) { Do-Clean }

switch ($Target) {
    'windows'          { Build-Windows }
    'linux-docker'     { Build-LinuxDocker }
    'openwrt-docker'   { Build-OpenWrtDocker }
    'android'          { Build-Android }
    'decky'            { Build-Decky }
    'magisk'           { Build-Magisk }
    'all'              { Build-All }
    default            { Write-Err "Unknown target: $Target"; exit 1 }
}

Write-Step "Build complete: $Target"
Write-Info "Output:    $DistDir\"
Write-Info "Binaries:  $BuildDir\bin\"
