#Requires -Version 5.1
[CmdletBinding()]
param(
 [Parameter(Mandatory)][string]$CandidateDirectory,
 [Parameter(Mandatory)][string]$CandidateCommit,
 [Parameter(Mandatory)][string]$ArchivePath
)
$worker = Join-Path $CandidateDirectory 'run_offline_physical_acceptance.ps1'
if (-not (Test-Path $worker -PathType Leaf)) { throw "Worker missing: $worker" }
$args = @('-NoProfile','-ExecutionPolicy','Bypass','-WindowStyle','Hidden','-File',$worker,'-CandidateDirectory',$CandidateDirectory,'-CandidateCommit',$CandidateCommit,'-ArchivePath',$ArchivePath)
Start-Process powershell.exe -Verb RunAs -ArgumentList $args
