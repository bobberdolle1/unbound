#Requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string]$CandidateDirectory,
    [Parameter(Mandatory)] [string]$CandidateCommit,
    [Parameter(Mandatory)] [string]$ArchivePath,
    [Parameter(Mandatory)] [string]$BundleDirectory,
    [int]$ProfileSeconds = 6,
    [int]$AutoTuneSeconds = 420
)

$ErrorActionPreference = 'Stop'
$CandidateDirectory = (Resolve-Path $CandidateDirectory).Path
$ArchivePath = (Resolve-Path $ArchivePath).Path
$bundle = [IO.Path]::GetFullPath($BundleDirectory)
New-Item -ItemType Directory -Force -Path $bundle | Out-Null

function Invoke-Captured([string]$Name, [string[]]$Arguments, [int]$TimeoutSeconds) {
    $stdout = Join-Path $bundle "$Name.stdout.log"
    $stderr = Join-Path $bundle "$Name.stderr.log"
    $startInfo = New-Object System.Diagnostics.ProcessStartInfo
    $startInfo.FileName = $exe
    $startInfo.Arguments = (($Arguments | ForEach-Object { "`"$_`"" }) -join ' ')
    $startInfo.UseShellExecute = $false
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $startInfo.CreateNoWindow = $true
    $process = New-Object System.Diagnostics.Process
    $process.StartInfo = $startInfo
    if (-not $process.Start()) { throw "PROCESS_START_FAILED: $Name" }
    $stdoutTask = $process.StandardOutput.ReadToEndAsync()
    $stderrTask = $process.StandardError.ReadToEndAsync()
    $timedOut = -not $process.WaitForExit($TimeoutSeconds * 1000)
    if ($timedOut) {
        Stop-TrackedProcessTree $process
        return [pscustomobject]@{ name=$Name; exitCode=$null; timedOut=$true; stdout=$stdout; stderr=$stderr }
    }
    $process.WaitForExit()
    [IO.File]::WriteAllText($stdout, $stdoutTask.Result)
    [IO.File]::WriteAllText($stderr, $stderrTask.Result)
    $capturedExitCode = $process.ExitCode
    if ($null -eq $capturedExitCode) { throw "PROCESS_EXIT_CODE_MISSING: $Name" }
    $exitCodeValue = [string]($capturedExitCode)
    $capture = [pscustomobject]@{ name=$Name; timedOut=$false; stdout=$stdout; stderr=$stderr }
    $capture | Add-Member -NotePropertyName exitCode -NotePropertyValue $exitCodeValue
    return $capture
}

trap {
    $errorText = ($_ | Out-String).Trim()
    if (Test-Path variable:result) {
        $result.status = 'FAIL'
        $result.error = $errorText
        $result.failedAt = (Get-Date).ToString('o')
        $result | ConvertTo-Json -Depth 8 | Set-Content -Path (Join-Path $bundle 'result.json') -Encoding utf8
    } else {
        [ordered]@{
            candidateCommit = $CandidateCommit
            candidateDirectory = $CandidateDirectory
            status = 'FAIL'
            error = $errorText
            failedAt = (Get-Date).ToString('o')
        } | ConvertTo-Json | Set-Content -Path (Join-Path $bundle 'result.json') -Encoding utf8
    }
    $errorText | Set-Content -Path (Join-Path $bundle 'controller.error.log') -Encoding utf8
    exit 1
}
$exe = Join-Path $CandidateDirectory 'Unbound.exe'


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

function Stop-TrackedProcessTree([Diagnostics.Process]$Process) {
    foreach ($id in @(Get-ProcessTreeIds $Process.Id | Sort-Object -Descending)) {
        Stop-Process -Id $id -Force -ErrorAction SilentlyContinue
    }
    Stop-Process -Id $Process.Id -Force -ErrorAction SilentlyContinue
    $Process.WaitForExit(10000) | Out-Null
}

function Test-AutoTuneTerminalResult([pscustomobject]$Capture, [switch]$AllowNoWinner) {
    if ($Capture.timedOut) { return 'PROCESS_TIMEOUT' }
    if (-not (Test-Path $Capture.stdout -PathType Leaf)) { return 'AUTOTUNE_RESULT_MISSING' }
    $records = @([regex]::Matches((Get-Content $Capture.stdout -Raw), '(?m)^AUTOTUNE_RESULT_JSON=(.+)\r?$'))
    if ($records.Count -ne 1) { return 'AUTOTUNE_RESULT_COUNT_INVALID' }
    try { $result = $records[0].Groups[1].Value | ConvertFrom-Json -ErrorAction Stop } catch { return 'AUTOTUNE_RESULT_MALFORMED' }
    if ($AllowNoWinner -and $Capture.exitCode -eq 1 -and -not $result.cancelled -and $result.lifecycle_failures -eq 0 -and $result.error_category -eq 'AUTOTUNE_INTERNAL_FAILURE' -and $result.error -eq 'no profile improved connectivity without regressions') { return $null }
    if ($Capture.exitCode -ne 0) { return "PROCESS_EXIT_$($Capture.exitCode)" }
    if (-not $result.completed -or $result.cancelled -or $result.lifecycle_failures -ne 0) { return 'AUTOTUNE_RESULT_FAILED' }
    if ($result.profiles_attempted -ne $result.profiles_total -or $result.profiles_completed -ne $result.profiles_total -or $result.profiles_failed_to_start -ne 0) { return 'AUTOTUNE_RESULT_INCOMPLETE' }
    return $null
}

function Invoke-LauncherSmoke([string]$LauncherPath) {
    $name = Split-Path $LauncherPath -Leaf
    $stdout = Join-Path $bundle "$name.stdout.log"
    $stderr = Join-Path $bundle "$name.stderr.log"
    $process = Start-Process -FilePath $env:ComSpec -ArgumentList '/d','/c',"`"$LauncherPath`"" -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    try {
        Start-Sleep -Seconds 8
        $started = @(Get-ProcessTreeIds $process.Id | ForEach-Object { Get-Process -Id $_ -ErrorAction SilentlyContinue } | Where-Object { $_.ProcessName -in 'Unbound','winws2' }).Count -gt 0
        return [pscustomobject]@{ name=$name; started=$started; stdout=$stdout; stderr=$stderr }
    } finally {
        Stop-TrackedProcessTree $process
    }
}

$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) { throw 'ELEVATION_REQUIRED' }
if (-not (Test-Path $exe -PathType Leaf)) { throw "Missing candidate executable: $exe" }
$candidateExeSha256 = (Get-FileHash $exe -Algorithm SHA256).Hash.ToLower()
$readyPath = Join-Path $bundle 'elevated-ready.json'
$readyTempPath = Join-Path $bundle ".elevated-ready-$PID.tmp"
[ordered]@{
    admin = $true
    pid = $PID
    started_at = (Get-Date).ToString('o')
    candidate_commit = $CandidateCommit
    candidate_exe_sha256 = $candidateExeSha256
} | ConvertTo-Json | Set-Content -Path $readyTempPath -Encoding utf8
Move-Item -Path $readyTempPath -Destination $readyPath -Force

$manifest = Join-Path $CandidateDirectory 'BUNDLE_SHA256SUMS.txt'
if (-not (Test-Path $manifest -PathType Leaf)) { throw 'Missing candidate manifest.' }
foreach ($line in Get-Content $manifest) {
    if ($line -match '^([0-9a-fA-F]{64})\s\s(.+)$') {
        $file = Join-Path $CandidateDirectory $matches[2]
        if (-not (Test-Path $file) -or (Get-FileHash $file -Algorithm SHA256).Hash -ne $matches[1]) { throw "Manifest mismatch: $($matches[2])" }
    }
}

$result = [ordered]@{ candidateCommit=$CandidateCommit; archivePath=$ArchivePath; executableVersion=(& $exe --version | Out-String).Trim(); startedAt=(Get-Date).ToString('o'); stages=@(); profiles=@() }
$kernel = @(Invoke-Captured 'kernel' @('--acceptance-test') 90 | Select-Object -Last 1)[0]
$result.kernel = $kernel
$kernel | ConvertTo-Json -Depth 4 | Set-Content -Path (Join-Path $bundle 'kernel.capture.json') -Encoding utf8
if ($kernel.timedOut -or $kernel.exitCode -ne 0) { throw "KERNEL_FAILED: timedOut=$($kernel.timedOut) exitCode=$($kernel.exitCode)" }
$result.stages += [pscustomobject]@{ name='KERNEL'; status='PASS' }

$catalog = @(Invoke-Captured 'profiles' @('--list-profiles','--json') 30 | Select-Object -Last 1)[0]
if ($catalog.timedOut -or $catalog.exitCode -ne 0) { throw 'PROFILE_CATALOG_FAILED' }
$profileSets = Get-Content $catalog.stdout -Raw | ConvertFrom-Json
$maxWinws = 0
$startConflicts = 0
$runningEmptyProfile = 0
foreach ($engine in $profileSets.PSObject.Properties) {
    foreach ($profile in @($engine.Value)) {
        if ($profile -ne 'Recommended (hostfakesplit)') { continue }
        $name = "profile-$([Guid]::NewGuid().ToString('N'))"
        $stdout = Join-Path $bundle "$name.stdout.log"
        $stderr = Join-Path $bundle "$name.stderr.log"
        Write-Host "[2/6] WINWS2 LIFECYCLE: $profile"
        $process = Start-Process -FilePath $exe -ArgumentList '--cli',"`"--profile=$profile`"","--run-duration=$($ProfileSeconds)s" -WindowStyle Hidden -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr
        Start-Sleep -Seconds 2
        $owned = @(Get-ProcessTreeIds $process.Id | ForEach-Object { Get-Process -Id $_ -ErrorAction SilentlyContinue } | Where-Object { $_.ProcessName -eq 'winws2' } | Select-Object -ExpandProperty Id)
        $active = @(Get-Process winws2 -ErrorAction SilentlyContinue)
        $maxWinws = [Math]::Max($maxWinws, $active.Count)
        $aliveAtStart = $owned.Count -eq 1 -and $null -ne (Get-Process -Id $owned[0] -ErrorAction SilentlyContinue)
        if ($owned.Count -ne 1 -or -not $aliveAtStart -or $active.Count -ne 1) { $startConflicts++ }
        $timedOut = -not $process.WaitForExit(($ProfileSeconds + 120) * 1000)
        if (-not $timedOut) {
            $process.WaitForExit()
            $exitCode = [int]$process.ExitCode
        } else {
            $exitCode = $null
        }
        $aliveAfterStop = if ($owned.Count -eq 1) { $null -ne (Get-Process -Id $owned[0] -ErrorAction SilentlyContinue) } else { $true }
        if ($timedOut -or $exitCode -ne 0 -or $aliveAfterStop) { $startConflicts++ }
        $emptyProfile = Select-String -Path $stdout -Pattern 'Profile:\s*$' -Quiet
        if ($emptyProfile) { $runningEmptyProfile++ }
        $result.profiles += [pscustomobject]@{ engine=$engine.Name; profile=$profile; processId=$process.Id; winws2Pid=if($owned.Count -eq 1){$owned[0]}else{$null}; aliveAtStart=$aliveAtStart; aliveAfterStop=$aliveAfterStop; timedOut=$timedOut; exitCode=$exitCode; emptyProfile=$emptyProfile; stdout=$stdout; stderr=$stderr }
        $result | ConvertTo-Json -Depth 8 | Set-Content -Path (Join-Path $bundle 'progress.json') -Encoding utf8
    }
}
$result.maxConcurrentWinws2 = $maxWinws
$result.startConflicts = $startConflicts
$result.runningEmptyProfile = $runningEmptyProfile
$result.finalWinws2 = @(Get-Process winws2 -ErrorAction SilentlyContinue).Count
Write-Host '[3/6] AUTOTUNE'
if ($maxWinws -ne 1 -or $startConflicts -ne 0 -or $runningEmptyProfile -ne 0 -or $result.finalWinws2 -ne 0) { throw 'OWNERSHIP_METRICS_FAILED' }
$result.stages += [pscustomobject]@{ name='OWNERSHIP'; status='PASS' }
$autotune = @(Invoke-Captured 'autotune' @('--cli','--autotune',"--run-duration=$($ProfileSeconds)s") $AutoTuneSeconds | Select-Object -Last 1)[0]
$result.autotune = $autotune
$autoTuneError = Test-AutoTuneTerminalResult $autotune -AllowNoWinner
if ($autoTuneError) { throw "AUTOTUNE_FAILED: $autoTuneError" }
Write-Host '[4/6] DOCTOR'
$result.stages += [pscustomobject]@{ name='AUTOTUNE'; status='PASS' }
$result.doctor = @(Invoke-Captured 'doctor' @('--test') 90 | Select-Object -Last 1)[0]
$result.stages += [pscustomobject]@{ name='DOCTOR'; status=if($result.doctor.exitCode -eq 0 -and -not $result.doctor.timedOut){'PASS'}else{'FAIL'} }
if ($result.stages[-1].status -ne 'PASS') { throw 'DOCTOR_FAILED' }
Write-Host '[5/6] LAUNCHERS'

$result.launchers = @(
    foreach ($launcher in 'general_recommended.cmd','general_autotune.cmd','general_universal.cmd','general_alt1_multisplit.cmd','general_alt2_fake_tls.cmd','service_control.cmd') {
        $path = Join-Path $CandidateDirectory $launcher
        if (-not (Test-Path $path -PathType Leaf)) { [pscustomobject]@{ name=$launcher; started=$false; error='MISSING' } } else { Invoke-LauncherSmoke $path }
    }
)
$result.finalWinws2AfterLaunchers = @(Get-Process winws2 -ErrorAction SilentlyContinue).Count
if (@($result.launchers | Where-Object { -not $_.started }).Count -ne 0 -or $result.finalWinws2AfterLaunchers -ne 0) { throw 'LAUNCHER_LIFECYCLE_FAILED' }
$result.stages += [pscustomobject]@{ name='LAUNCHERS'; status='PASS' }
$result.finishedAt = (Get-Date).ToString('o')
$result | ConvertTo-Json -Depth 8 | Set-Content (Join-Path $bundle 'result.json') -Encoding utf8
Write-Output "PRIVILEGED_PREFLIGHT_PASS bundle=$bundle"
