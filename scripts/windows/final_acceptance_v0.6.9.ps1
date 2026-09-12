#Requires -Version 5.1
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot
$admin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $admin) { throw 'Run final_acceptance_v0.6.9.cmd as Administrator.' }
$metadataPath = Join-Path $root 'CANDIDATE.json'
$manifestPath = Join-Path $root 'BUNDLE_SHA256SUMS.txt'
$exe = Join-Path $root 'Unbound.exe'
if (-not (Test-Path $metadataPath) -or -not (Test-Path $manifestPath) -or -not (Test-Path $exe)) { throw 'Candidate identity files are missing.' }
$candidate = Get-Content $metadataPath -Raw | ConvertFrom-Json
if ($candidate.version -ne '0.6.9') { throw 'Candidate version mismatch.' }
if ((& $exe --version | Out-String).Trim() -ne 'unbound 0.6.9 (windows/amd64)') { throw 'Candidate executable version mismatch.' }
if ((Get-FileHash $exe -Algorithm SHA256).Hash.ToLower() -ne $candidate.exe_sha256) { throw 'Candidate executable hash mismatch.' }
foreach ($line in Get-Content $manifestPath) {
    if ($line -match '^([0-9a-fA-F]{64})\s\s(.+)$') {
        $file = Join-Path $root $matches[2]
        if (-not (Test-Path $file) -or (Get-FileHash $file -Algorithm SHA256).Hash.ToLower() -ne $matches[1].ToLower()) { throw "Manifest mismatch: $($matches[2])" }
    }
}
$evidenceRoot = Join-Path ([Environment]::GetFolderPath('MyDocuments')) "UnboundAcceptance\\final-v0.6.9-$([Guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Path $evidenceRoot -Force | Out-Null
$preflightBundle = Join-Path $evidenceRoot 'privileged-preflight'
& (Join-Path $root 'run_privileged_preflight.ps1') -CandidateDirectory $root -CandidateCommit $candidate.candidate_commit -ArchivePath $root -BundleDirectory $preflightBundle
if ($LASTEXITCODE -ne 0) { throw "PRIVILEGED_PREFLIGHT_FAILED evidence=$preflightBundle" }
Write-Host 'PRIVILEGED PREFLIGHT PASSED' -ForegroundColor Green
Write-Host 'NOW TURN HAPP OFF' -ForegroundColor Yellow
Write-Host 'DO NOT CLOSE THIS WINDOW' -ForegroundColor Yellow
& (Join-Path $root 'run_offline_physical_acceptance.ps1') -CandidateDirectory $root -CandidateCommit $candidate.candidate_commit -ArchivePath $root -OutputRoot $evidenceRoot
$exitCode = $LASTEXITCODE
$acceptance = Get-ChildItem $evidenceRoot -Directory | Where-Object { $_.Name -like '*-Acceptance-*' } | Sort-Object LastWriteTime -Descending | Select-Object -First 1
if ($exitCode -eq 0) {
    Write-Host 'ACCEPTANCE PASSED' -ForegroundColor Green
    Write-Host 'CLEANUP COMPLETE' -ForegroundColor Green
    Write-Host 'SAFE TO RE-ENABLE HAPP' -ForegroundColor Green
    exit 0
}
$result = if ($acceptance) { Get-Content (Join-Path $acceptance.FullName 'result.json') -Raw | ConvertFrom-Json } else { $null }
$failed = if ($result) { @($result.stages | Where-Object { $_.status -ne 'PASS' } | Select-Object -First 1).name } else { 'HARNESS' }
Write-Host 'ACCEPTANCE FAILED' -ForegroundColor Red
Write-Host "Failed stage: $failed" -ForegroundColor Red
Write-Host 'CLEANUP COMPLETE' -ForegroundColor Green
Write-Host 'SAFE TO RE-ENABLE HAPP' -ForegroundColor Green
exit 1
