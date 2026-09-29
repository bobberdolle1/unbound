#Requires -Version 5.1
# Elevated WinDivert acceptance for a packaged, identity-verified release candidate.
[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$candidatePath = Join-Path $PSScriptRoot 'CANDIDATE.json'
$bin = Join-Path $PSScriptRoot 'Unbound.exe'
if (-not (Test-Path $candidatePath -PathType Leaf) -or -not (Test-Path $bin -PathType Leaf)) {
    throw 'Run this script from an extracted release bundle with CANDIDATE.json and Unbound.exe.'
}
$candidate = Get-Content $candidatePath -Raw | ConvertFrom-Json
$identity = (& $bin --version --json | Out-String | ConvertFrom-Json)
if ($identity.version -ne $candidate.version -or $identity.commit -ne $candidate.candidate_commit -or $identity.dirty -ne $false -or $identity.channel -ne 'release' -or $identity.os -ne 'windows') {
    throw 'Candidate executable release identity mismatch.'
}

$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Warning 'This script requires Administrator elevation to open the WinDivert kernel driver.'
    exit 1
}

Write-Host "Executing WinDivert kernel driver acceptance for Unbound $($candidate.version)." -ForegroundColor Yellow
$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $bin
$psi.Arguments = '--acceptance-test'
$psi.WorkingDirectory = $PSScriptRoot
$psi.UseShellExecute = $false
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true
$psi.CreateNoWindow = $true

$proc = [System.Diagnostics.Process]::Start($psi)
while (-not $proc.StandardOutput.EndOfStream) {
    Write-Host $proc.StandardOutput.ReadLine()
}
$stderr = $proc.StandardError.ReadToEnd()
$proc.WaitForExit()
if (-not [string]::IsNullOrWhiteSpace($stderr)) {
    Write-Warning "Standard error: $stderr"
}
if ($proc.ExitCode -ne 0) {
    throw "WinDivert kernel driver acceptance failed with exit code $($proc.ExitCode)"
}
Write-Host 'KERNEL_RUNTIME_VERIFIED: WinDivert Acceptance PASSED' -ForegroundColor Green
