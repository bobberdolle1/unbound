#Requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string]$CandidateDirectory,
    [string]$OutputRoot = (Join-Path $env:USERPROFILE 'Documents\UnboundAcceptance'),
    [int]$CleanWaitSeconds = 600,
    [int]$ProfileSeconds = 45,
    [int]$AutoTuneSeconds = 180,
    [string]$LogSink = 'bobpc@192.168.0.236',
    [string]$CandidateCommit,
    [string]$ArchivePath
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$timestamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$bundle = Join-Path $OutputRoot "v0.6.9-$timestamp"
$logPath = Join-Path $bundle 'harness.log'
$resultPath = Join-Path $bundle 'result.json'
New-Item -ItemType Directory -Force -Path $bundle | Out-Null

function Write-ProgressLine([string]$Message) {
    $line = "$(Get-Date -Format o) $Message"
    $line | Tee-Object -FilePath $logPath -Append
}
function Save-Results {
    $results | ConvertTo-Json -Depth 10 | Set-Content $resultPath -Encoding utf8
}
function Show-LocalStatus([string]$Message, [string]$Title) {
    Add-Type -AssemblyName System.Windows.Forms
    [void][Windows.Forms.MessageBox]::Show($Message, $Title, [Windows.Forms.MessageBoxButtons]::OK, [Windows.Forms.MessageBoxIcon]::Information)
}
function Invoke-Captured([string]$Name, [string]$FilePath, [string[]]$Arguments, [int]$TimeoutSeconds) {
    $stdout = Join-Path $bundle "$Name.stdout.log"
    $stderr = Join-Path $bundle "$Name.stderr.log"
    $process = Start-Process -FilePath $FilePath -ArgumentList $Arguments -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $timedOut = -not $process.WaitForExit($TimeoutSeconds * 1000)
    if ($timedOut) { Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue; $process.WaitForExit() }
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

function Invoke-RecommendedMatrix([string]$FilePath) {
    $stdout = Join-Path $bundle 'recommended.stdout.log'
    $stderr = Join-Path $bundle 'recommended.stderr.log'
    $process = Start-Process -FilePath $FilePath -ArgumentList '--cli','--profile','rec',"--run-duration=$($ProfileSeconds)s" -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    Start-Sleep -Seconds ([Math]::Min(10, $ProfileSeconds))
    $activeState = Get-NetworkSnapshot
    $probes = @(Invoke-WebProbes)
    $timedOut = -not $process.WaitForExit(($ProfileSeconds + 35) * 1000)
    if ($timedOut) { Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue; $process.WaitForExit() }
    [pscustomobject]@{ exitCode=if($timedOut){$null}else{$process.ExitCode}; timedOut=$timedOut; activeState=$activeState; probes=$probes; stdout=$stdout; stderr=$stderr }
}

$results = [ordered]@{ version='0.6.9'; startedAt=(Get-Date).ToString('o'); candidateDirectory=$CandidateDirectory; candidateCommit=$CandidateCommit; archivePath=$ArchivePath; stages=@(); status='RUNNING' }
try {
    $admin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
function Invoke-LauncherSmoke([string]$LauncherPath) {
    $stdout = Join-Path $bundle "$(Split-Path $LauncherPath -Leaf).stdout.log"
    $stderr = Join-Path $bundle "$(Split-Path $LauncherPath -Leaf).stderr.log"
    $command = "`"$LauncherPath`""
    $process = Start-Process -FilePath $env:ComSpec -ArgumentList '/d','/c',$command -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    Start-Sleep -Seconds 8
    $engineStarted = @(Get-Process Unbound,winws2 -ErrorAction SilentlyContinue | Select-Object ProcessName,Id)
    Get-Process winws2 -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Get-Process Unbound -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
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
    Show-LocalStatus 'TURN HAPP OFF NOW. The local worker is independent of OMP and keeps writing evidence.' 'UNBOUND v0.6.9 acceptance'
    $results.executableSha256 = (Get-FileHash $exe -Algorithm SHA256).Hash
    $results.executableVersion = (& $exe --version | Out-String).Trim()
    $results.archiveSha256 = if ($ArchivePath -and (Test-Path $ArchivePath -PathType Leaf)) { (Get-FileHash $ArchivePath -Algorithm SHA256).Hash } else { $null }
    $results.preCleanSnapshot = Get-NetworkSnapshot
    $results.stages += [pscustomobject]@{ name='PREPARED'; status='PASS'; at=(Get-Date).ToString('o') }
    Write-ProgressLine 'PREPARED. Turn Happ OFF now. The harness is waiting locally; no OMP connection is required.'
    Save-Results
    $deadline = (Get-Date).AddSeconds($CleanWaitSeconds)
    do { $clean = Get-DataPlaneState; if ($clean.clean) { break }; Start-Sleep -Seconds 2 } while ((Get-Date) -lt $deadline)
    $results.cleanDataPlane = $clean
    if (-not $clean.clean) { throw "Timed out waiting for Happ data plane to be off after $CleanWaitSeconds seconds." }
    $results.cleanNetworkSnapshot = Get-NetworkSnapshot
    $results.stages += [pscustomobject]@{ name='CLEAN_WINDOW'; status='PASS'; at=(Get-Date).ToString('o') }
    Save-Results
    $results.dnsBaseline = @(
        foreach ($target in 'www.youtube.com','discord.com','store.steampowered.com','www.cloudflare.com') {
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
    Save-Results
    $results.recommended = Invoke-RecommendedMatrix $exe
    Save-Results
    $results.autoTune = Invoke-Captured 'autotune' $exe @('--cli','--autotune',"--run-duration=$($ProfileSeconds)s") $AutoTuneSeconds
    Save-Results
    $results.launcherSmoke = @(
        foreach ($launcher in 'general_recommended.cmd','general_autotune.cmd','general_universal.cmd','general_alt1_multisplit.cmd','general_alt2_fake_tls.cmd','service_control.cmd') {
            $path = Join-Path $CandidateDirectory $launcher
            if (-not (Test-Path $path -PathType Leaf)) { [pscustomobject]@{ name=$launcher; started=$false; failure='MISSING' } } else { Invoke-LauncherSmoke $path }
        }
    )
    Save-Results
    $results.status = 'COMPLETE'
} catch {
    $results.status = 'FAILED'; $results.failure = $_.Exception.ToString(); Write-ProgressLine "FAILED: $($_.Exception.Message)"
} finally {
    Get-Process winws2 -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Get-Process Unbound -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    $results.cleanupSnapshot = Get-NetworkSnapshot
    $results.finishedAt = (Get-Date).ToString('o')
    $results | ConvertTo-Json -Depth 10 | Set-Content $resultPath -Encoding utf8
    if ((Test-Path $SshKeyPath) -and (Get-Command ssh.exe -ErrorAction SilentlyContinue) -and (Get-Command scp.exe -ErrorAction SilentlyContinue)) {
        $remote = "unbound-acceptance/v0.6.9/$timestamp"
        & ssh.exe -i $SshKeyPath -o BatchMode=yes -o ConnectTimeout=10 $LogSink "mkdir -p '$remote'" 2>&1 | Tee-Object -FilePath (Join-Path $bundle 'log-sink.log') -Append
        & scp.exe -i $SshKeyPath -o BatchMode=yes -o ConnectTimeout=10 -r $bundle "$LogSink`:$remote/" 2>&1 | Tee-Object -FilePath (Join-Path $bundle 'log-sink.log') -Append
    }
    Write-ProgressLine "ACCEPTANCE $($results.status). Local bundle: $bundle"
    Write-Host "ACCEPTANCE $($results.status)" -ForegroundColor Green
    Write-Host 'SAFE TO RE-ENABLE HAPP' -ForegroundColor Green
    Show-LocalStatus "ACCEPTANCE $($results.status)`nSAFE TO RE-ENABLE HAPP" 'UNBOUND v0.6.9 acceptance'
}
if ($results.status -ne 'COMPLETE') { exit 1 }
