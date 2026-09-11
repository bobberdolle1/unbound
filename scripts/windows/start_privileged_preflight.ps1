#Requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string]$CandidateDirectory,
    [Parameter(Mandatory)] [string]$CandidateCommit,
    [Parameter(Mandatory)] [string]$ArchivePath,
    [ValidateRange(5, 300)] [int]$HandshakeTimeoutSeconds = 45
)

$ErrorActionPreference = 'Stop'
$CandidateDirectory = (Resolve-Path $CandidateDirectory -ErrorAction Stop).Path
$ArchivePath = (Resolve-Path $ArchivePath -ErrorAction Stop).Path
$worker = Join-Path $CandidateDirectory 'run_privileged_preflight.ps1'
if (-not (Test-Path $worker -PathType Leaf)) { throw "Worker missing: $worker" }

$acceptanceRoot = Join-Path ([Environment]::GetFolderPath('MyDocuments')) 'UnboundAcceptance'
$bundle = Join-Path $acceptanceRoot "preflight-$(Get-Date -Format 'yyyyMMdd-HHmmss')-$([Guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Force -Path $bundle | Out-Null
$expectedHash = (Get-FileHash (Join-Path $CandidateDirectory 'Unbound.exe') -Algorithm SHA256).Hash.ToLower()
$arguments = @(
    '-NoProfile', '-ExecutionPolicy', 'Bypass',
    '-File', $worker,
    '-CandidateDirectory', $CandidateDirectory,
    '-CandidateCommit', $CandidateCommit,
    '-ArchivePath', $ArchivePath,
    '-BundleDirectory', $bundle
)
try {
    $process = Start-Process -FilePath powershell.exe -Verb RunAs -WorkingDirectory $CandidateDirectory -ArgumentList $arguments -PassThru -ErrorAction Stop
} catch {
    throw "UAC_SHELLEXECUTE_FAILED: $($_.Exception.Message)"
}
if ($null -eq $process) { throw 'UAC_SHELLEXECUTE_FAILED: elevated preflight controller did not start.' }
Write-Output "UAC_REQUEST_ISSUED processId=$($process.Id) bundle=$bundle"

$readyPath = Join-Path $bundle 'elevated-ready.json'
$deadline = (Get-Date).AddSeconds($HandshakeTimeoutSeconds)
do {
    if (Test-Path $readyPath -PathType Leaf) {
        try {
            $ready = Get-Content $readyPath -Raw | ConvertFrom-Json -ErrorAction Stop
            $alive = $null -ne (Get-Process -Id $ready.pid -ErrorAction SilentlyContinue)
            if ($ready.admin -eq $true -and $alive -and $ready.candidate_commit -eq $CandidateCommit -and $ready.candidate_exe_sha256 -eq $expectedHash) {
                Write-Output "ELEVATED_HANDSHAKE_VERIFIED processId=$($ready.pid) bundle=$bundle"
                [pscustomobject]@{ worker=$worker; processId=$ready.pid; bundle=$bundle; startedAt=$ready.started_at }
                exit 0
            }
            throw 'ELEVATED_HANDSHAKE_IDENTITY_MISMATCH'
        } catch {
            if ($_.Exception.Message -eq 'ELEVATED_HANDSHAKE_IDENTITY_MISMATCH') { throw }
        }
    }
    Start-Sleep -Milliseconds 250
} while ((Get-Date) -lt $deadline)
$childState = if ($process.HasExited) { "exited:$($process.ExitCode)" } else { 'running_without_handshake' }
throw "ELEVATED_HANDSHAKE_TIMEOUT: processId=$($process.Id) state=$childState readyPath=$readyPath"
