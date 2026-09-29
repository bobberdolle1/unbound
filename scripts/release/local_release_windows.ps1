#Requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string]$Version,
    [Parameter(Mandatory)] [string]$ExpectedCommit
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProjectRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
Set-Location $ProjectRoot

function Require([bool]$Condition, [string]$Message) {
    if (-not $Condition) { throw $Message }
}

$actualCommit = (& git rev-parse HEAD).Trim()
Require ($actualCommit -eq $ExpectedCommit) "checkout commit $actualCommit does not match $ExpectedCommit"
Require ([string]::IsNullOrWhiteSpace((& git status --porcelain --untracked-files=normal | Out-String))) 'release checkout is dirty'
$metadata = Get-Content wails.json -Raw | ConvertFrom-Json
$frontend = Get-Content frontend/package.json -Raw | ConvertFrom-Json
Require ($metadata.info.productVersion -eq $Version) 'wails.json version does not match requested release version'
Require ($frontend.version -eq $Version) 'frontend/package.json version does not match requested release version'

$env:EXPECTED_SHA = $ExpectedCommit
& ./scripts/ci/windows.ps1 -Target all
if ($LASTEXITCODE -ne 0) { throw "canonical Windows CI failed with exit code $LASTEXITCODE" }

$env:UNBOUND_VERSION = $Version
$env:UNBOUND_BUILD_COMMIT = $ExpectedCommit
$env:UNBOUND_BUILD_DIRTY = 'false'
$env:UNBOUND_BUILD_CHANNEL = 'release'
& ./scripts/build/build_windows.ps1
if ($LASTEXITCODE -ne 0) { throw "Windows release build failed with exit code $LASTEXITCODE" }

$binary = Join-Path $ProjectRoot 'build\bin\unbound.exe'
$identity = (& $binary --version --json | Out-String | ConvertFrom-Json)
Require ($identity.version -eq $Version) 'release binary version mismatch'
Require ($identity.commit -eq $ExpectedCommit) 'release binary commit mismatch'
Require ($identity.dirty -eq $false) 'release binary is marked dirty'
Require ($identity.channel -eq 'release') 'release binary channel mismatch'
Require ($identity.os -eq 'windows') 'release binary platform mismatch'

& ./scripts/build/package_windows_release.ps1 -Version $Version
if ($LASTEXITCODE -ne 0) { throw "Windows packaging failed with exit code $LASTEXITCODE" }
$archive = (Resolve-Path "release/unbound-v$Version-windows-amd64.zip").Path
$env:UNBOUND_WINDOWS_ARCHIVE = $archive
go test ./tests -run '^TestWindowsPackagingArchive' -count=1
if ($LASTEXITCODE -ne 0) { throw "Windows package tests failed with exit code $LASTEXITCODE" }

$evidence = [ordered]@{
    version = $Version
    source_commit = $ExpectedCommit
    platform = 'windows'
    build_channel = 'release'
    artifact = (Split-Path $archive -Leaf)
    built_at = (Get-Date).ToUniversalTime().ToString('o')
    build_host_role = 'windows-release-host'
    sha256 = (Get-FileHash $archive -Algorithm SHA256).Hash.ToLower()
    identity = $identity
} | ConvertTo-Json -Depth 5
$evidence | Set-Content (Join-Path $ProjectRoot 'release\windows-release-evidence.json') -Encoding utf8
Write-Output "WINDOWS_LOCAL_RELEASE=PASS artifact=$archive"
