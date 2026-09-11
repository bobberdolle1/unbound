#Requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string]$CandidateDirectory,
    [Parameter(Mandatory)] [string]$CandidateCommit,
    [Parameter(Mandatory)] [string]$ArchivePath,
    [int]$ProfileSeconds = 6,
    [int]$AutoTuneSeconds = 300
)

$ErrorActionPreference = 'Stop'
$CandidateDirectory = (Resolve-Path $CandidateDirectory).Path
$ArchivePath = (Resolve-Path $ArchivePath).Path
$bundle = Join-Path ([Environment]::GetFolderPath('MyDocuments')) "UnboundAcceptance\preflight-$([Guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Force -Path $bundle | Out-Null
$exe = Join-Path $CandidateDirectory 'Unbound.exe'

function Invoke-Captured([string]$Name, [string[]]$Arguments, [int]$TimeoutSeconds) {
    $stdout = Join-Path $bundle "$Name.stdout.log"
    $stderr = Join-Path $bundle "$Name.stderr.log"
    $process = Start-Process -FilePath $exe -ArgumentList $Arguments -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $timedOut = -not $process.WaitForExit($TimeoutSeconds * 1000)
    if ($timedOut) { Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue }
    [pscustomobject]@{ name=$Name; exitCode=if($timedOut){$null}else{$process.ExitCode}; timedOut=$timedOut; stdout=$stdout; stderr=$stderr }
}

$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) { throw 'ELEVATION_REQUIRED' }
if (-not (Test-Path $exe -PathType Leaf)) { throw "Missing candidate executable: $exe" }

$manifest = Join-Path $CandidateDirectory 'BUNDLE_SHA256SUMS.txt'
if (-not (Test-Path $manifest -PathType Leaf)) { throw 'Missing candidate manifest.' }
foreach ($line in Get-Content $manifest) {
    if ($line -match '^([0-9a-fA-F]{64})\s\s(.+)$') {
        $file = Join-Path $CandidateDirectory $matches[2]
        if (-not (Test-Path $file) -or (Get-FileHash $file -Algorithm SHA256).Hash -ne $matches[1]) { throw "Manifest mismatch: $($matches[2])" }
    }
}

$result = [ordered]@{ candidateCommit=$CandidateCommit; archivePath=$ArchivePath; executableVersion=(& $exe --version | Out-String).Trim(); startedAt=(Get-Date).ToString('o'); stages=@(); profiles=@() }
$kernel = Invoke-Captured 'kernel' @('--acceptance-test') 90
$result.kernel = $kernel
if ($kernel.timedOut -or $kernel.exitCode -ne 0) { throw 'KERNEL_FAILED' }
$result.stages += [pscustomobject]@{ name='KERNEL'; status='PASS' }

$catalog = Invoke-Captured 'profiles' @('--list-profiles','--json') 30
if ($catalog.timedOut -or $catalog.exitCode -ne 0) { throw 'PROFILE_CATALOG_FAILED' }
$profileSets = Get-Content $catalog.stdout -Raw | ConvertFrom-Json
$maxWinws = 0
foreach ($engine in $profileSets.PSObject.Properties) {
    foreach ($profile in @($engine.Value)) {
        $entry = Invoke-Captured ("profile-" + [Guid]::NewGuid().ToString('N')) @('--cli','--profile',$profile,"--run-duration=$($ProfileSeconds)s") ($ProfileSeconds + 35)
        $active = @(Get-Process winws2 -ErrorAction SilentlyContinue)
        $maxWinws = [Math]::Max($maxWinws, $active.Count)
        $result.profiles += [pscustomobject]@{ engine=$engine.Name; profile=$profile; exitCode=$entry.exitCode; timedOut=$entry.timedOut; winwsAfter=$active.Count; stdout=$entry.stdout; stderr=$entry.stderr }
        if ($entry.timedOut -or $entry.exitCode -ne 0 -or $active.Count -ne 0) { throw "PROFILE_LIFECYCLE_FAILED: $profile" }
    }
}
$result.maxConcurrentWinws2 = $maxWinws
$result.finalWinws2 = @(Get-Process winws2 -ErrorAction SilentlyContinue).Count
if ($maxWinws -ne 1 -or $result.finalWinws2 -ne 0) { throw 'OWNERSHIP_METRICS_FAILED' }
$result.stages += [pscustomobject]@{ name='OWNERSHIP'; status='PASS' }

$autotune = Invoke-Captured 'autotune' @('--cli','--autotune') $AutoTuneSeconds
$result.autotune = $autotune
$records = @([regex]::Matches((Get-Content $autotune.stdout -Raw), '(?m)^AUTOTUNE_RESULT_JSON=(.+)\r?$'))
if ($autotune.timedOut -or $autotune.exitCode -ne 0 -or $records.Count -ne 1) { throw 'AUTOTUNE_FAILED' }
$result.stages += [pscustomobject]@{ name='AUTOTUNE'; status='PASS' }

$result.doctor = Invoke-Captured 'doctor' @('--test') 90
$result.stages += [pscustomobject]@{ name='DOCTOR'; status=if($result.doctor.exitCode -eq 0 -and -not $result.doctor.timedOut){'PASS'}else{'FAIL'} }
if ($result.stages[-1].status -ne 'PASS') { throw 'DOCTOR_FAILED' }
$result.finishedAt = (Get-Date).ToString('o')
$result | ConvertTo-Json -Depth 8 | Set-Content (Join-Path $bundle 'result.json') -Encoding utf8
Write-Output "PRIVILEGED_PREFLIGHT_PASS bundle=$bundle"
