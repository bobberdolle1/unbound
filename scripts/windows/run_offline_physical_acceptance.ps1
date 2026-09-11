#Requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string]$CandidateDirectory,
    [string]$OutputRoot = (Join-Path $env:USERPROFILE 'Documents\UnboundAcceptance'),
    [int]$CleanWaitSeconds = 600,
    [int]$ProfileSeconds = 45,
    [int]$AutoTuneSeconds = 180,
    [string]$LogSink = 'bobpc@192.168.0.236',
    [string]$SshKeyPath,
    [string]$CandidateCommit,
    [string]$ArchivePath,
    [ValidateSet('Acceptance', 'DetachedWorker', 'ForcedFailure', 'DnsBaseline', 'StatusWindow')] [string]$SmokeMode = 'Acceptance',
    [ValidateRange(1, 60)] [int]$SmokeSleepSeconds = 3,
    [switch]$SimulateNotificationFailure,
    [switch]$SimulateLogSinkFailure
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$timestamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$bundle = Join-Path $OutputRoot "v0.6.9-$timestamp-$SmokeMode-$([Guid]::NewGuid().ToString('N'))"
$logPath = Join-Path $bundle 'progress.log'
$resultPath = Join-Path $bundle 'result.json'
$statusPath = Join-Path $bundle 'status.json'
$statusEventLogPath = Join-Path $bundle 'status-window.log'
New-Item -ItemType Directory -Force -Path $bundle | Out-Null

$trackedProcesses = @()
$statusWindowProcess = $null

function Write-ProgressLine([string]$Message) {
    $line = "$(Get-Date -Format o) $Message"
    Add-Content -Path $logPath -Value $line -Encoding utf8
    Write-Host $line
}
function Save-Results {
    $temporaryResultPath = "$resultPath.tmp"
    $results | ConvertTo-Json -Depth 10 | Set-Content $temporaryResultPath -Encoding utf8
    Move-Item -Path $temporaryResultPath -Destination $resultPath -Force
}
function Show-LocalStatus([string]$Message, [string]$Title) {
    try {
        if ($SimulateNotificationFailure) { throw 'Simulated notification failure.' }
        Add-Type -AssemblyName System.Windows.Forms
        [void][Windows.Forms.MessageBox]::Show($Message, $Title, [Windows.Forms.MessageBoxButtons]::OK, [Windows.Forms.MessageBoxIcon]::Information)
    } catch {
        Write-ProgressLine "LOCAL_NOTIFICATION_FAILED: $($_.Exception.Message)"
    }
}
function Save-AcceptanceStatus([string]$Headline, [string]$Detail, [bool]$Terminal = $false) {
    $temporaryStatusPath = "$statusPath.tmp"
    [ordered]@{
        headline = $Headline
        detail = $Detail
        terminal = $Terminal
        updatedAt = (Get-Date).ToString('o')
    } | ConvertTo-Json | Set-Content $temporaryStatusPath -Encoding utf8
    Move-Item -Path $temporaryStatusPath -Destination $statusPath -Force
}
function Start-AcceptanceStatusWindow {
    try {
        if ($SimulateNotificationFailure) { throw 'Simulated notification failure.' }
        $statusScript = Join-Path $PSScriptRoot 'show_offline_acceptance_status.ps1'
        if (-not (Test-Path $statusScript -PathType Leaf)) { throw "Status window script missing: $statusScript" }
        $script:statusWindowProcess = Start-Process -FilePath powershell.exe -ArgumentList '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $statusScript, '-StatusPath', $statusPath, '-EventLogPath', $statusEventLogPath -PassThru
        $results.statusWindow = [ordered]@{ processId = $statusWindowProcess.Id; statusPath = $statusPath; eventLogPath = $statusEventLogPath }
        Write-ProgressLine "LOCAL_STATUS_WINDOW_STARTED: pid=$($statusWindowProcess.Id)"
    } catch {
        $results.statusWindowError = $_.Exception.Message
        Write-ProgressLine "LOCAL_NOTIFICATION_FAILED: $($_.Exception.Message)"
    }
}
function Register-HarnessProcess([Diagnostics.Process]$Process) {
    $script:trackedProcesses += $Process
    return $Process
}
function Get-ProcessTreeIds([int]$RootProcessId) {
    $processIds = New-Object 'System.Collections.Generic.List[int]'
    $pending = New-Object 'System.Collections.Generic.Queue[int]'
    $pending.Enqueue($RootProcessId)
    while ($pending.Count -gt 0) {
        $parentProcessId = $pending.Dequeue()
        foreach ($child in @(Get-CimInstance Win32_Process -Filter "ParentProcessId = $parentProcessId" -ErrorAction SilentlyContinue)) {
            $childProcessId = [int]$child.ProcessId
            $processIds.Add($childProcessId)
            $pending.Enqueue($childProcessId)
        }
    }
    return $processIds.ToArray()
}
function Stop-HarnessProcessTree([Diagnostics.Process]$Process) {
    if ($Process.HasExited) { return }
    $processIds = @(Get-ProcessTreeIds $Process.Id) + $Process.Id
    foreach ($processId in ($processIds | Sort-Object -Descending -Unique)) {
        Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
    }
}
function Invoke-Captured([string]$Name, [string]$FilePath, [string[]]$Arguments, [int]$TimeoutSeconds) {
    $stdout = Join-Path $bundle "$Name.stdout.log"
    $stderr = Join-Path $bundle "$Name.stderr.log"
    $process = Register-HarnessProcess (Start-Process -FilePath $FilePath -ArgumentList $Arguments -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr)
    $timedOut = -not $process.WaitForExit($TimeoutSeconds * 1000)
    if ($timedOut) { Stop-HarnessProcessTree $process }
    [pscustomobject]@{ name = $Name; exitCode = if ($timedOut) { $null } else { $process.ExitCode }; timedOut = $timedOut; stdout = $stdout; stderr = $stderr }
}
function Get-DataPlaneState {
    $proxy = Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings'
    $processes = @(Get-Process xray, 'sing-box' -ErrorAction SilentlyContinue | Select-Object ProcessName,Id)
    $tunnel = @(Get-NetAdapter -Name 'happ-tun' -ErrorAction SilentlyContinue | Select-Object Name,Status,ifIndex)
    $tunnelIndices = @($tunnel | Select-Object -ExpandProperty ifIndex)
    $tunnelDefaultRoutes = @(Get-NetRoute -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object { $_.DestinationPrefix -eq '0.0.0.0/0' -and $_.ifIndex -in $tunnelIndices } | Select-Object DestinationPrefix,InterfaceAlias,NextHop,RouteMetric,ifIndex)
    [pscustomobject]@{ xrayOrSingBox = $processes; proxyEnabled = [bool]$proxy.ProxyEnable; proxyServer = $proxy.ProxyServer; happTunnel = $tunnel; happTunnelDefaultRoutes = $tunnelDefaultRoutes; clean = ($processes.Count -eq 0 -and -not [bool]$proxy.ProxyEnable -and $tunnelDefaultRoutes.Count -eq 0) }
}
function Get-NetworkSnapshot {
    [pscustomobject]@{
        timestamp = (Get-Date).ToString('o')
        adapters = @(Get-NetAdapter | Select-Object Name,InterfaceDescription,Status,ifIndex,HardwareInterface,Virtual)
        ipConfiguration = @(Get-NetIPConfiguration | Select-Object InterfaceAlias,InterfaceIndex,IPv4Address,IPv4DefaultGateway,DnsServer)
        defaultRoutes = @(Get-NetRoute -AddressFamily IPv4 | Where-Object DestinationPrefix -eq '0.0.0.0/0' | Select-Object InterfaceAlias,InterfaceIndex,NextHop,RouteMetric,ifMetric)
        dns = @(Get-DnsClientServerAddress | Select-Object InterfaceAlias,InterfaceIndex,AddressFamily,ServerAddresses)
        processes = @(Get-Process Happ,xray,'sing-box',winws2,Unbound -ErrorAction SilentlyContinue | Select-Object ProcessName,Id)
    }
}
function Invoke-WebProbes {
    $targets = @(
        @{ name='Cloudflare'; url='https://www.cloudflare.com/' },
        @{ name='YouTube'; url='https://www.youtube.com/' },
        @{ name='Discord'; url='https://discord.com/' },
        @{ name='Steam'; url='https://store.steampowered.com/' }
    )
    foreach ($target in $targets) {
        $watch = [Diagnostics.Stopwatch]::StartNew()
        try {
            $request = [Net.HttpWebRequest]::Create($target.url); $request.Timeout = 10000; $request.ReadWriteTimeout = 10000; $request.AllowAutoRedirect = $true; $request.UserAgent = 'UnboundAcceptance/0.6.9'
            $response = $request.GetResponse(); $code = [int]$response.StatusCode; $response.Close()
            [pscustomobject]@{ name=$target.name; url=$target.url; ok=($code -ge 200 -and $code -lt 400); httpStatus=$code; elapsedMs=$watch.ElapsedMilliseconds; error=$null }
        } catch { [pscustomobject]@{ name=$target.name; url=$target.url; ok=$false; httpStatus=$null; elapsedMs=$watch.ElapsedMilliseconds; error=$_.Exception.Message } }
    }
}

function Test-AutoTuneTerminalResult([object]$Captured) {
    if ($Captured.timedOut -or $Captured.exitCode -ne 0) { return [pscustomobject]@{ status='FAIL'; error='AUTOTUNE_PROCESS_EXIT' } }
    if (-not (Test-Path $Captured.stdout -PathType Leaf)) { return [pscustomobject]@{ status='FAIL'; error='AUTOTUNE_TERMINAL_JSON_MISSING' } }
    $records = @([regex]::Matches([IO.File]::ReadAllText($Captured.stdout, [Text.Encoding]::UTF8), '(?m)^AUTOTUNE_RESULT_JSON=(.+)\r?$'))
    if ($records.Count -ne 1) { return [pscustomobject]@{ status='FAIL'; error='AUTOTUNE_TERMINAL_JSON_COUNT' } }
    try { $report = $records[0].Groups[1].Value | ConvertFrom-Json -ErrorAction Stop } catch { return [pscustomobject]@{ status='FAIL'; error='AUTOTUNE_TERMINAL_JSON_MALFORMED' } }
    foreach ($field in 'completed','cancelled','lifecycle_failures','profiles_total','profiles_attempted','profiles_completed','profiles_failed_to_start','winner','winner_score','baseline') {
        if ($report.PSObject.Properties.Name -notcontains $field) { return [pscustomobject]@{ status='FAIL'; error="AUTOTUNE_TERMINAL_JSON_MISSING_$field" } }
    }
    if (-not $report.completed -or $report.cancelled -or $report.lifecycle_failures -ne 0) { return [pscustomobject]@{ status='FAIL'; error='AUTOTUNE_TERMINAL_JSON_FAILURE' } }
    if ($report.profiles_total -lt $report.profiles_attempted -or $report.profiles_attempted -ne $report.profiles_completed -or $report.profiles_failed_to_start -ne 0 -or [string]::IsNullOrWhiteSpace($report.winner)) { return [pscustomobject]@{ status='FAIL'; error='AUTOTUNE_TERMINAL_JSON_INCOHERENT' } }
    return [pscustomobject]@{ status='PASS'; error=$null; report=$report }
}

function Invoke-HarmlessSmoke {
    $resolvedCandidateDirectory = (Resolve-Path $CandidateDirectory -ErrorAction Stop).Path
    $results.worker = [ordered]@{
        processId = $PID
        workingDirectory = (Get-Location).Path
        expectedWorkingDirectory = $resolvedCandidateDirectory
        resultEncoding = 'UTF-8'
        progressEncoding = 'UTF-8'
    }
    if ($results.worker.workingDirectory -ne $resolvedCandidateDirectory) {
        throw "Worker current directory mismatch: expected $resolvedCandidateDirectory, got $($results.worker.workingDirectory)"
    }

    $results.stages += [pscustomobject]@{ name = 'WORKER_READY'; status = 'PASS'; at = (Get-Date).ToString('o') }
    Write-ProgressLine "WORKER_READY mode=$SmokeMode"
    Save-Results

    switch ($SmokeMode) {
        'DetachedWorker' {
            Start-Sleep -Seconds $SmokeSleepSeconds
            $results.stages += [pscustomobject]@{ name = 'DETACHED_WORKER_RUNTIME'; status = 'PASS'; at = (Get-Date).ToString('o') }
            Write-ProgressLine 'DETACHED_WORKER_RUNTIME=PASS'
        }
        'ForcedFailure' {
            $child = Register-HarnessProcess (Start-Process -FilePath powershell.exe -ArgumentList '-NoProfile', '-Command', 'Start-Sleep -Seconds 30' -PassThru)
            $results.ownedChildProcessId = $child.Id
            $results.stages += [pscustomobject]@{ name = 'FORCED_SMOKE'; status = 'RUNNING'; at = (Get-Date).ToString('o') }
            Save-Results
            throw 'FORCED_SMOKE_FAILURE: intentional worker exception'
        }
        'DnsBaseline' {
            $dnsTargets = @('www.youtube.com','discord.com','store.steampowered.com','www.cloudflare.com')
            $results.dnsBaseline = @(
                foreach ($target in $dnsTargets) {
                    $watch = [Diagnostics.Stopwatch]::StartNew()
                    try {
                        $records = @(Resolve-DnsName -Name $target -ErrorAction Stop | Where-Object { $_.Type -in 'A','AAAA' } | Select-Object Name,Type,IPAddress,Server)
                        [pscustomobject]@{ name=$target; records=$records; latencyMs=$watch.ElapsedMilliseconds; error=$null; category='PASS' }
                    } catch {
                        [pscustomobject]@{ name=$target; records=@(); latencyMs=$watch.ElapsedMilliseconds; error=$_.Exception.Message; category='DNS_ERROR' }
                    }
                }
            )
            $results.stages += [pscustomobject]@{ name = 'DNS_HARNESS_RUNTIME'; status = 'PASS'; at = (Get-Date).ToString('o') }
            Write-ProgressLine 'DNS_HARNESS_RUNTIME=PASS'
        }
        'StatusWindow' {
            Save-AcceptanceStatus 'CLEAN WINDOW DETECTED' 'STATUS SMOKE: ACCEPTANCE RUNNING — DO NOT ENABLE HAPP'
            Start-Sleep -Seconds $SmokeSleepSeconds
            Save-AcceptanceStatus 'ACCEPTANCE COMPLETE' "CLEANUP COMPLETE`nSAFE TO RE-ENABLE HAPP" $true
            $results.stages += [pscustomobject]@{ name = 'STATUS_WINDOW_RUNTIME'; status = 'PASS'; at = (Get-Date).ToString('o') }
            Write-ProgressLine 'STATUS_WINDOW_RUNTIME=PASS'
        }
    }
}

function Invoke-RecommendedMatrix([string]$FilePath) {
    $stdout = Join-Path $bundle 'recommended.stdout.log'
    $stderr = Join-Path $bundle 'recommended.stderr.log'
    $process = Register-HarnessProcess (Start-Process -FilePath $FilePath -ArgumentList '--cli','--profile','rec',"--run-duration=$($ProfileSeconds)s" -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr)
    Start-Sleep -Seconds ([Math]::Min(10, $ProfileSeconds))
    $activeState = Get-NetworkSnapshot
    $probes = @(Invoke-WebProbes)
    $timedOut = -not $process.WaitForExit(($ProfileSeconds + 35) * 1000)
    if ($timedOut) { Stop-HarnessProcessTree $process }
    [pscustomobject]@{ exitCode=if($timedOut){$null}else{$process.ExitCode}; timedOut=$timedOut; activeState=$activeState; probes=$probes; stdout=$stdout; stderr=$stderr }
}

$results = [ordered]@{ version='0.6.9'; mode=$SmokeMode; startedAt=(Get-Date).ToString('o'); candidateDirectory=$CandidateDirectory; candidateCommit=$CandidateCommit; archivePath=$ArchivePath; stages=@(); execution_state='RUNNING'; acceptance_verdict='INVALID' }
try {
    if ($SmokeMode -in 'Acceptance', 'StatusWindow') {
        Start-AcceptanceStatusWindow
        Save-AcceptanceStatus 'ACCEPTANCE WORKER STARTED' 'Waiting for the local clean data plane.'
    }
    if ($SmokeMode -ne 'Acceptance') {
        Invoke-HarmlessSmoke
        $results.execution_state = 'COMPLETE'
        return
    }
    $admin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
function Invoke-LauncherSmoke([string]$LauncherPath) {
    $stdout = Join-Path $bundle "$(Split-Path $LauncherPath -Leaf).stdout.log"
    $stderr = Join-Path $bundle "$(Split-Path $LauncherPath -Leaf).stderr.log"
    $command = "`"$LauncherPath`""
    $process = Register-HarnessProcess (Start-Process -FilePath $env:ComSpec -ArgumentList '/d','/c',$command -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr)
    Start-Sleep -Seconds 8
    $engineStarted = @(Get-ProcessTreeIds $process.Id | ForEach-Object { Get-Process -Id $_ -ErrorAction SilentlyContinue } | Where-Object { $_.ProcessName -in 'Unbound','winws2' } | Select-Object ProcessName,Id)
    Stop-HarnessProcessTree $process
    $process.WaitForExit(10000) | Out-Null
    [pscustomobject]@{ name=(Split-Path $LauncherPath -Leaf); started=($engineStarted.Count -gt 0); launcherExitCode=if ($process.HasExited) {$process.ExitCode} else {$null}; stdout=$stdout; stderr=$stderr }
}

    if (-not $admin) { throw 'Run this harness elevated. It never changes Happ, routes, DNS, or proxy settings.' }
    $exe = Join-Path $CandidateDirectory 'Unbound.exe'
    if (-not (Test-Path $exe -PathType Leaf)) { throw "Candidate executable missing: $exe" }
    $hashes = Join-Path $CandidateDirectory 'BUNDLE_SHA256SUMS.txt'
    if (-not (Test-Path $hashes -PathType Leaf)) { throw "Bundle manifest missing: $hashes" }
    $hashFailures = @()
    foreach ($line in Get-Content $hashes) { if ($line -match '^([0-9a-fA-F]{64})\s\s(.+)$') { $file = Join-Path $CandidateDirectory $matches[2]; if (-not (Test-Path $file) -or (Get-FileHash $file -Algorithm SHA256).Hash -ne $matches[1]) { $hashFailures += $matches[2] } } }
    if ($hashFailures) { throw "Bundle hash verification failed: $($hashFailures -join ', ')" }
    $results.bundleHashVerification = 'PASS'
    $results.executableSha256 = (Get-FileHash $exe -Algorithm SHA256).Hash
    $results.executableVersion = (& $exe --version | Out-String).Trim()
    $results.archiveSha256 = if ($ArchivePath -and (Test-Path $ArchivePath -PathType Leaf)) { (Get-FileHash $ArchivePath -Algorithm SHA256).Hash } else { $null }
    $results.preCleanSnapshot = Get-NetworkSnapshot
    $results.stages += [pscustomobject]@{ name='PREPARED'; status='PASS'; at=(Get-Date).ToString('o') }
    Write-ProgressLine 'PREPARED. The local worker is waiting for a clean data plane; no OMP connection is required.'
    Save-Results
    $deadline = (Get-Date).AddSeconds($CleanWaitSeconds)
    do { $clean = Get-DataPlaneState; if ($clean.clean) { break }; Start-Sleep -Seconds 2 } while ((Get-Date) -lt $deadline)
    $results.cleanDataPlane = $clean
    if (-not $clean.clean) { throw "Timed out waiting for a clean data plane after $CleanWaitSeconds seconds." }
    $results.cleanNetworkSnapshot = Get-NetworkSnapshot
    $results.stages += [pscustomobject]@{ name='CLEAN_WINDOW'; status='PASS'; at=(Get-Date).ToString('o') }
    Save-AcceptanceStatus 'CLEAN WINDOW DETECTED' 'ACCEPTANCE RUNNING — DO NOT ENABLE HAPP'
    Save-Results
    $dnsTargets = @('www.youtube.com','discord.com','store.steampowered.com','www.cloudflare.com')
    $results.dnsBaseline = @(
        foreach ($target in $dnsTargets) {
            $watch = [Diagnostics.Stopwatch]::StartNew()
            try {
                $records = @(Resolve-DnsName -Name $target -ErrorAction Stop | Where-Object { $_.Type -in 'A','AAAA' } | Select-Object Name,Type,IPAddress,Server)
                [pscustomobject]@{ name=$target; records=$records; latencyMs=$watch.ElapsedMilliseconds; error=$null; category='PASS' }
            } catch {
                [pscustomobject]@{ name=$target; records=@(); latencyMs=$watch.ElapsedMilliseconds; error=$_.Exception.Message; category='DNS_ERROR' }
            }
        }
    )
    $results.cleanWebBaseline = @(Invoke-WebProbes)
    Save-Results
    $results.kernelAcceptance = Invoke-Captured 'kernel-acceptance' $exe @('--acceptance-test') 90
    Save-Results
    $results.elevatedDoctor = Invoke-Captured 'doctor' $exe @('--test') 90
    $results.autoTune = Invoke-Captured 'autotune' $exe @('--cli','--autotune',"--run-duration=$($ProfileSeconds)s") $AutoTuneSeconds
    $autoTuneStage = Test-AutoTuneTerminalResult $results.autoTune
    $results.stages += [pscustomobject]@{ name='AUTOTUNE'; status=$autoTuneStage.status; error=$autoTuneStage.error; at=(Get-Date).ToString('o') }
    if ($autoTuneStage.status -ne 'PASS') { throw "AutoTune acceptance failed closed: $($autoTuneStage.error)" }
    Save-Results
    $results.launcherSmoke = @(
        foreach ($launcher in 'general_recommended.cmd','general_autotune.cmd','general_universal.cmd','general_alt1_multisplit.cmd','general_alt2_fake_tls.cmd','service_control.cmd') {
            $path = Join-Path $CandidateDirectory $launcher
            if (-not (Test-Path $path -PathType Leaf)) { [pscustomobject]@{ name=$launcher; started=$false; failure='MISSING' } } else { Invoke-LauncherSmoke $path }
        }
    )
    Save-Results
    $results.execution_state = 'COMPLETE'
    $requiredStages = @('CLEAN_WINDOW','KERNEL','RECOMMENDED_FIELD','DOCTOR','CONCURRENT_START','AUTOTUNE','LAUNCHERS','CLEANUP')
    $stagePasses = foreach ($stageName in $requiredStages) {
        @($results.stages | Where-Object { $_.name -eq $stageName -and $_.status -eq 'PASS' }).Count -eq 1
    }
    $results.acceptance_verdict = if ($stagePasses -notcontains $false) { 'PASS' } else { 'FAIL' }
} catch {
    $results.execution_state = 'FAILED'
    $results.acceptance_verdict = 'FAIL'
    $results.failure = [ordered]@{ stage = if ($SmokeMode -eq 'ForcedFailure') { 'FORCED_SMOKE' } else { 'HARNESS' }; error = $_.Exception.Message }
    Write-ProgressLine "FAILED: $($_.Exception.Message)"
} finally {
    foreach ($process in $trackedProcesses) {
        Stop-HarnessProcessTree $process
    }
    $results.ownedChildrenAliveAfterCleanup = @(
        $trackedProcesses | Where-Object { -not $_.HasExited }
    ).Count
    if ($statusWindowProcess -and $statusWindowProcess.HasExited) {
        $results.statusWindowExitedBeforeTerminal = $true
        Write-ProgressLine 'LOCAL_NOTIFICATION_FAILED: Status window exited before terminal acceptance status.'
    }
    $failedStage = @($results.stages | Where-Object { $_.status -ne 'PASS' } | Select-Object -First 1).name
    $finalHeadline = if ($results.acceptance_verdict -eq 'PASS') { 'ACCEPTANCE PASSED' } else { 'ACCEPTANCE FAILED' }
    $finalDetail = if ($results.acceptance_verdict -eq 'PASS') { "CLEANUP COMPLETE`nSAFE TO RE-ENABLE HAPP" } else { "Failed stage: $failedStage`nCLEANUP COMPLETE`nSAFE TO RE-ENABLE HAPP" }
    Save-AcceptanceStatus $finalHeadline $finalDetail $true
    $results.cleanupSnapshot = Get-NetworkSnapshot
    $results.finishedAt = (Get-Date).ToString('o')
    Save-Results
    Write-ProgressLine "ACCEPTANCE $($results.acceptance_verdict). Local bundle: $bundle"
    Write-Host "ACCEPTANCE $($results.acceptance_verdict)" -ForegroundColor Green
    Write-Host 'Acceptance evidence is available in the local bundle.' -ForegroundColor Green
    if ($SimulateNotificationFailure) {
        Show-LocalStatus "ACCEPTANCE $($results.acceptance_verdict)`nEvidence is available in the local bundle." 'UNBOUND v0.6.9 acceptance'
    }
}
if ($results.acceptance_verdict -ne 'PASS') { exit 1 }
