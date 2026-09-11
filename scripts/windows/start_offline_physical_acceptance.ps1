#Requires -Version 5.1
[CmdletBinding()]
param(
 [Parameter(Mandatory)][string]$CandidateDirectory,
 [Parameter(Mandatory)][string]$CandidateCommit,
 [Parameter(Mandatory)][string]$ArchivePath
)
$CandidateDirectory = (Resolve-Path $CandidateDirectory -ErrorAction Stop).Path
$ArchivePath = (Resolve-Path $ArchivePath -ErrorAction Stop).Path
$worker = Join-Path $CandidateDirectory 'run_offline_physical_acceptance.ps1'
if (-not (Test-Path $worker -PathType Leaf)) { throw "Worker missing: $worker" }
$args = @('-NoProfile','-ExecutionPolicy','Bypass','-WindowStyle','Hidden','-File',$worker,'-CandidateDirectory',$CandidateDirectory,'-CandidateCommit',$CandidateCommit,'-ArchivePath',$ArchivePath)
$process = Start-Process powershell.exe -Verb RunAs -ArgumentList $args -WorkingDirectory $CandidateDirectory -PassThru
if ($null -eq $process) { throw 'Elevated acceptance worker did not start.' }
[pscustomobject]@{
    worker = $worker
    processId = $process.Id
    startedAt = (Get-Date).ToString('o')
}
