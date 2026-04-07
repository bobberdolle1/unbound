# ============================================================================
# UNBOUND — Windows Standalone Build Script (PowerShell)
# ============================================================================
# Usage:
#   .\scripts\build\build_windows.ps1 [-Debug]
# ============================================================================

[CmdletBinding()]
param([switch]$Debug)

$ScriptDir   = $PSScriptRoot
$ProjectRoot = Split-Path (Split-Path $ScriptDir -Parent) -Parent
$BuildDir    = Join-Path $ProjectRoot "build\bin"

function Write-Info  { Write-Host "[INFO] $args" -ForegroundColor Cyan }
function Write-Ok    { Write-Host "[OK] $args" -ForegroundColor Green }

$DebugFlag = ""
if ($Debug) {
    $DebugFlag = "-gcflags='all=-N -l'"
    Write-Info "Building in DEBUG mode"
}

Write-Info "Building Windows binary..."

New-Item -ItemType Directory -Force -Path $BuildDir | Out-Null

Push-Location $ProjectRoot
try {
    $goArgs = @("build", "-trimpath")
    if ($Debug) { $goArgs += "-gcflags=all=-N -l" }
    $goArgs += "-o", "$BuildDir\unbound.exe"
    $goArgs += "./..."

    & go $goArgs

    if ($LASTEXITCODE -ne 0) { throw "Go build failed" }

    Write-Ok "Windows binary built: $BuildDir\unbound.exe"
    Get-ChildItem "$BuildDir\unbound.exe" | Select-Object Name, Length
} finally {
    Pop-Location
}
