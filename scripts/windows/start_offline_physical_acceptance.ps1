#Requires -Version 5.1
[CmdletBinding()]
param(
 [Parameter(Mandatory)][string]$CandidateDirectory,
 [Parameter(Mandatory)][string]$CandidateCommit,
 [Parameter(Mandatory)][string]$ArchivePath,
 [ValidateSet('Acceptance', 'DetachedWorker', 'ForcedFailure', 'DnsBaseline')] [string]$SmokeMode = 'Acceptance',
 [string]$OutputRoot,
 [ValidateRange(1, 60)] [int]$SmokeSleepSeconds = 3,
 [switch]$SimulateNotificationFailure,
 [switch]$SimulateLogSinkFailure
)
$CandidateDirectory = (Resolve-Path $CandidateDirectory -ErrorAction Stop).Path
$ArchivePath = (Resolve-Path $ArchivePath -ErrorAction Stop).Path
$worker = Join-Path $CandidateDirectory 'run_offline_physical_acceptance.ps1'
if (-not (Test-Path $worker -PathType Leaf)) {
    $worker = Join-Path $PSScriptRoot 'run_offline_physical_acceptance.ps1'
}
if (-not (Test-Path $worker -PathType Leaf)) { throw "Worker missing: $worker" }
$args = @('-NoProfile','-ExecutionPolicy','Bypass','-File',$worker,'-CandidateDirectory',$CandidateDirectory,'-CandidateCommit',$CandidateCommit,'-ArchivePath',$ArchivePath,'-SmokeMode',$SmokeMode,'-SmokeSleepSeconds',$SmokeSleepSeconds)
if ($SmokeMode -ne 'Acceptance') { $args += @('-WindowStyle', 'Hidden') }
if ($OutputRoot) { $args += @('-OutputRoot', $OutputRoot) }
if ($SimulateNotificationFailure) { $args += '-SimulateNotificationFailure' }
if ($SimulateLogSinkFailure) { $args += '-SimulateLogSinkFailure' }
$startParameters = @{ FilePath = 'powershell.exe'; ArgumentList = $args; WorkingDirectory = $CandidateDirectory; PassThru = $true }
if ($SmokeMode -eq 'Acceptance') { $startParameters.Verb = 'RunAs' }
$process = Start-Process @startParameters
if ($null -eq $process) { throw 'Elevated acceptance worker did not start.' }
[pscustomobject]@{
    worker = $worker
    processId = $process.Id
    startedAt = (Get-Date).ToString('o')
}
