# UNBOUND — native Windows Wails build.
# Usage: .\scripts\build\build_windows.ps1 [-DebugBuild]
# Environment: UNBOUND_VERSION=<override>, UNBOUND_BUILD_COMMIT=<sha>,
# UNBOUND_BUILD_DIRTY=true|false|unknown, UNBOUND_BUILD_CHANNEL=development|release

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
    $BuildCommit = if ($env:UNBOUND_BUILD_COMMIT) {
        $env:UNBOUND_BUILD_COMMIT
    } else {
        $commit = (& git rev-parse --verify HEAD 2>$null | Select-Object -First 1)
        if ($commit) { $commit.Trim() } else { "unknown" }
    }
    $BuildDirty = if ($env:UNBOUND_BUILD_DIRTY) {
        $env:UNBOUND_BUILD_DIRTY
    } elseif ($BuildCommit -eq "unknown") {
        "unknown"
    } elseif ([string]::IsNullOrWhiteSpace((& git status --porcelain --untracked-files=normal | Out-String))) {
        "false"
    } else {
        "true"
    }
    $BuildChannel = if ($env:UNBOUND_BUILD_CHANNEL) { $env:UNBOUND_BUILD_CHANNEL } else { "development" }
    $LinkerFlags = "-H windowsgui -X unbound/engine.Version=$Version -X unbound/engine.BuildCommit=$BuildCommit -X unbound/engine.BuildDirty=$BuildDirty -X unbound/engine.BuildChannel=$BuildChannel"
    $WailsArgs = @(
        "build",
        "-clean",
        "-o", "unbound.exe",
        "-ldflags", $LinkerFlags
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
