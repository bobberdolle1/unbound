# UNBOUND — native Windows Wails build.
# Usage: .\scripts\build\build_windows.ps1 [-DebugBuild]
# Environment: UNBOUND_VERSION=<override>

[CmdletBinding()]
param([switch]$DebugBuild)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$BuildDir = Join-Path $ProjectRoot "build\bin"

foreach ($Command in @("go", "node", "npm", "wails")) {
    if (-not (Get-Command $Command -ErrorAction SilentlyContinue)) {
        throw "$Command is required but was not found in PATH"
    }
}

Push-Location $ProjectRoot
try {
    $ConfigVersion = (Get-Content "wails.json" -Raw | ConvertFrom-Json).info.productVersion
    $Version = if ($env:UNBOUND_VERSION) { $env:UNBOUND_VERSION } else { $ConfigVersion }
    $WailsArgs = @(
        "build",
        "-clean",
        "-o", "unbound.exe",
        "-ldflags", "-X unbound/engine.Version=$Version"
    )
    if ($DebugBuild) {
        $WailsArgs += "-debug"
    }

    Write-Host "[INFO] Building native Windows Wails app v$Version..." -ForegroundColor Cyan
    & wails $WailsArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Wails build failed with exit code $LASTEXITCODE"
    }

    $Output = Join-Path $BuildDir "unbound.exe"
    if (-not (Test-Path $Output -PathType Leaf)) {
        throw "Wails reported success but did not create $Output"
    }

    $File = Get-Item $Output
    if ($File.Length -le 0) {
        throw "Built executable is empty: $Output"
    }
    Write-Host "[OK] Windows Wails app built: $Output ($($File.Length) bytes)" -ForegroundColor Green
} finally {
    Pop-Location
}
