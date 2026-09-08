# UNBOUND v0.6.7 — Elevated WinDivert Driver Acceptance Script
# Run this script in an Administrator PowerShell prompt on Windows.

$ErrorActionPreference = "Stop"

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host " UNBOUND v0.6.7 — Windows WinDivert Driver Acceptance" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

# 1. Verify Administrator elevation
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Warning "This script requires Administrator elevation to open the WinDivert kernel driver."
    Write-Host "Please right-click PowerShell and select 'Run as Administrator'."
    exit 1
}

# 2. Locate Unbound.exe
$bin = Join-Path $PSScriptRoot "Unbound.exe"
if (-not (Test-Path $bin)) {
    $bin = Join-Path $PSScriptRoot "..\..\build\bin\Unbound.exe"
}
if (-not (Test-Path $bin)) {
    $bin = "Unbound.exe"
}

Write-Host "Executing WinDivert kernel driver acceptance: $bin --acceptance-test" -ForegroundColor Yellow

$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $bin
$psi.Arguments = "--acceptance-test"
$psi.WorkingDirectory = $PSScriptRoot
$psi.UseShellExecute = $false
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true
$psi.CreateNoWindow = $true

$proc = [System.Diagnostics.Process]::Start($psi)
while (-not $proc.StandardOutput.EndOfStream) {
    $line = $proc.StandardOutput.ReadLine()
    Write-Host $line
}
$stderr = $proc.StandardError.ReadToEnd()
$proc.WaitForExit()

if (-not [string]::IsNullOrWhiteSpace($stderr)) {
    Write-Warning "Standard error: $stderr"
}

if ($proc.ExitCode -ne 0) {
    Write-Error "WinDivert kernel driver acceptance failed with exit code $($proc.ExitCode)"
    exit $proc.ExitCode
}
Write-Host "==================================================" -ForegroundColor Green
Write-Host " KERNEL_RUNTIME_VERIFIED: WinDivert Acceptance PASSED" -ForegroundColor Green
Write-Host "==================================================" -ForegroundColor Green
exit 0
