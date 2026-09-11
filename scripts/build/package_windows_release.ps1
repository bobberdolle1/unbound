#Requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Version
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
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

Copy-Item $Binary $Bundle
Copy-Item (Join-Path $ProjectRoot "README.md") $Bundle
Copy-Item (Join-Path $ProjectRoot "LICENSE") $Bundle
Copy-Item (Join-Path $ProjectRoot "engine\third_party\ZAPRET2_LICENSE.txt") $Bundle
Copy-Item (Join-Path $ProjectRoot "engine\third_party\ZAPRET_LICENSE.txt") $Bundle
Copy-Item (Join-Path $ProjectRoot "engine\ENGINE_PROVENANCE.json") $Bundle
Copy-Item (Join-Path $ProjectRoot "scripts\control_windows\*") $Bundle

Get-ChildItem $Bundle -File | Sort-Object Name | ForEach-Object {
    "$((Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLower())  $($_.Name)"
} | Set-Content (Join-Path $Bundle "BUNDLE_SHA256SUMS.txt") -Encoding ascii

Compress-Archive -Path "$Bundle\*" -DestinationPath $Archive -Force
Write-Output $Archive
