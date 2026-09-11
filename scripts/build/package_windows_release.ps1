#Requires -Version 5.1
[CmdletBinding()]
param(
    [string]$Version
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent

$MetadataVersion = (Get-Content (Join-Path $ProjectRoot "wails.json") -Raw | ConvertFrom-Json).info.productVersion
if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = $MetadataVersion
} elseif ($Version -ne $MetadataVersion) {
    throw "Requested version $Version does not match canonical wails.json version $MetadataVersion"
}
$Binary = Join-Path $ProjectRoot "build\bin\unbound.exe"
$ReleaseRoot = Join-Path $ProjectRoot "release"
$Bundle = Join-Path $ReleaseRoot "unbound-v$Version-windows-amd64"
$Archive = "$Bundle.zip"

if (-not (Test-Path $Binary -PathType Leaf)) {
    throw "Windows binary not found: $Binary"
}

Remove-Item $Bundle -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item $Archive -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $Bundle -Force | Out-Null

Copy-Item $Binary (Join-Path $Bundle "Unbound.exe")
Copy-Item (Join-Path $ProjectRoot "README.md") $Bundle
Copy-Item (Join-Path $ProjectRoot "CHANGELOG.md") $Bundle
Copy-Item (Join-Path $ProjectRoot "LICENSE") $Bundle
Copy-Item (Join-Path $ProjectRoot "engine\third_party\ZAPRET2_LICENSE.txt") $Bundle
Copy-Item (Join-Path $ProjectRoot "engine\third_party\ZAPRET_LICENSE.txt") $Bundle
Copy-Item (Join-Path $ProjectRoot "engine\ENGINE_PROVENANCE.json") $Bundle
Copy-Item (Join-Path $ProjectRoot "scripts\windows\verify_uac_acceptance.ps1") $Bundle
Copy-Item (Join-Path $ProjectRoot "scripts\windows\run_offline_physical_acceptance.ps1") $Bundle
Copy-Item (Join-Path $ProjectRoot "scripts\windows\start_offline_physical_acceptance.ps1") $Bundle
Copy-Item (Join-Path $ProjectRoot "scripts\windows\show_offline_acceptance_status.ps1") $Bundle
Copy-Item (Join-Path $ProjectRoot "scripts\control_windows\*") $Bundle

Get-ChildItem $Bundle -File | Sort-Object Name | ForEach-Object {
    "$((Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLower())  $($_.Name)"
} | Set-Content (Join-Path $Bundle "BUNDLE_SHA256SUMS.txt") -Encoding ascii

Compress-Archive -Path "$Bundle\*" -DestinationPath $Archive -Force

$SmokeDir = Join-Path ([System.IO.Path]::GetTempPath()) "unbound-package-smoke-$([Guid]::NewGuid())"
try {
    Expand-Archive -Path $Archive -DestinationPath $SmokeDir -Force
    $RequiredFiles = @(
        "Unbound.exe",
        "README.md",
        "CHANGELOG.md",
        "LICENSE",
        "ZAPRET2_LICENSE.txt",
        "ZAPRET_LICENSE.txt",
        "ENGINE_PROVENANCE.json",
        "verify_uac_acceptance.ps1",
        "run_offline_physical_acceptance.ps1",
        "start_offline_physical_acceptance.ps1",
        "show_offline_acceptance_status.ps1",
        "general_recommended.cmd",
        "general_autotune.cmd",
        "general_universal.cmd",
        "general_alt1_multisplit.cmd",
        "general_alt2_fake_tls.cmd",
        "service_control.cmd",
        "BUNDLE_SHA256SUMS.txt"
    )
    $Missing = $RequiredFiles | Where-Object { -not (Test-Path (Join-Path $SmokeDir $_) -PathType Leaf) }
    if ($Missing) {
        throw "Windows archive is missing required files: $($Missing -join ', ')"
    }
} finally {
    Remove-Item $SmokeDir -Recurse -Force -ErrorAction SilentlyContinue
}
Write-Output $Archive
