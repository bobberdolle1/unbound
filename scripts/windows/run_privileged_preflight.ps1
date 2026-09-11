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
    if ($timedOut) {
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        return [pscustomobject]@{ name=$Name; exitCode=$null; timedOut=$true; stdout=$stdout; stderr=$stderr }
    }
    $process.Refresh()
    [pscustomobject]@{ name=$Name; exitCode=$process.ExitCode; timedOut=$false; stdout=$stdout; stderr=$stderr }
}

function Get-ProcessTreeIds([int]$RootProcessId) {
    $ids = New-Object 'System.Collections.Generic.List[int]'
    $pending = New-Object 'System.Collections.Generic.Queue[int]'
    $pending.Enqueue($RootProcessId)
    while ($pending.Count -gt 0) {
        $parent = $pending.Dequeue()
        foreach ($child in @(Get-CimInstance Win32_Process -Filter "ParentProcessId = $parent" -ErrorAction SilentlyContinue)) {
            $ids.Add([int]$child.ProcessId)
            $pending.Enqueue([int]$child.ProcessId)
        }
    }
    return $ids.ToArray()
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
        $name = "profile-$([Guid]::NewGuid().ToString('N'))"
        $stdout = Join-Path $bundle "$name.stdout.log"
        $stderr = Join-Path $bundle "$name.stderr.log"
        $process = Start-Process -FilePath $exe -ArgumentList '--cli','--profile',$profile,"--run-duration=$($ProfileSeconds)s" -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr
        Start-Sleep -Seconds 2
        $owned = @(Get-ProcessTreeIds $process.Id | ForEach-Object { Get-Process -Id $_ -ErrorAction SilentlyContinue } | Where-Object { $_.ProcessName -eq 'winws2' } | Select-Object -ExpandProperty Id)
        $active = @(Get-Process winws2 -ErrorAction SilentlyContinue)
        $maxWinws = [Math]::Max($maxWinws, $active.Count)
        $aliveAtStart = $owned.Count -eq 1 -and $null -ne (Get-Process -Id $owned[0] -ErrorAction SilentlyContinue)
        $timedOut = -not $process.WaitForExit(($ProfileSeconds + 35) * 1000)
        if ($timedOut) {
            Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        } else {
            $process.Refresh()
        }
        $aliveAfterStop = if ($owned.Count -eq 1) { $null -ne (Get-Process -Id $owned[0] -ErrorAction SilentlyContinue) } else { $true }
        if ($timedOut -or $process.ExitCode -ne 0 -or -not $aliveAtStart -or $aliveAfterStop -or $active.Count -ne 1) { throw "PROFILE_LIFECYCLE_FAILED: $profile" }
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
