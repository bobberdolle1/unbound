#Requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string]$CandidateDirectory,
    [Parameter(Mandatory)] [string]$CandidateCommit,
    [Parameter(Mandatory)] [string]$ArchivePath
)
$CandidateDirectory = (Resolve-Path $CandidateDirectory -ErrorAction Stop).Path
$ArchivePath = (Resolve-Path $ArchivePath -ErrorAction Stop).Path
$worker = Join-Path $CandidateDirectory 'run_privileged_preflight.ps1'
if (-not (Test-Path $worker -PathType Leaf)) { throw "Worker missing: $worker" }
$process = Start-Process -FilePath powershell.exe -Verb RunAs -WorkingDirectory $CandidateDirectory -ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-File',$worker,'-CandidateDirectory',$CandidateDirectory,'-CandidateCommit',$CandidateCommit,'-ArchivePath',$ArchivePath -PassThru
if ($null -eq $process) { throw 'Elevated preflight controller did not start.' }
[pscustomobject]@{ worker=$worker; processId=$process.Id; startedAt=(Get-Date).ToString('o') }
