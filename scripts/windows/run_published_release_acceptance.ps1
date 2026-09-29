#Requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string]$ArchivePath,
    [Parameter(Mandatory)] [string]$OutputRoot,
    [Parameter(Mandatory)] [string]$ExpectedVersion,
    [Parameter(Mandatory)] [string]$ExpectedCommit,
    [ValidateRange(5, 60)] [int]$DiagnosticTimeoutSeconds = 15,
    [ValidateRange(30, 1800)] [int]$AutoTuneTimeoutSeconds = 600
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$archive = (Resolve-Path $ArchivePath -ErrorAction Stop).Path
$bundle = Join-Path $OutputRoot "published-v$ExpectedVersion-$(Get-Date -Format 'yyyyMMdd-HHmmss')-$([guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Force -Path $bundle | Out-Null
$commandDirectory = Join-Path $bundle 'commands'
New-Item -ItemType Directory -Force -Path $commandDirectory | Out-Null
$resultPath = Join-Path $bundle 'result.json'
$extract = Join-Path $bundle 'artifact'
$results = [ordered]@{ version=$ExpectedVersion; commit=$ExpectedCommit; archive=$archive; started_at=(Get-Date).ToString('o'); diagnostic_snapshot=[ordered]@{status='NOT_RUN'; commands=@()}; stages=[ordered]@{}; commands=@(); verdict='RUNNING' }

function Save-Result { $results | ConvertTo-Json -Depth 12 | Set-Content "$resultPath.tmp" -Encoding utf8; Move-Item "$resultPath.tmp" $resultPath -Force }
function Set-Stage([string]$Name, [ValidateSet('PASS','FAIL','TIMEOUT','NOT_APPLICABLE')] [string]$Status, [string]$Detail) { $results.stages[$Name] = [ordered]@{status=$Status; detail=$Detail; at=(Get-Date).ToString('o')}; Save-Result }
function Stop-Tree([int]$ProcessId) { & $env:ComSpec /d /c "taskkill /F /T /PID $ProcessId" *> $null; return -not (Get-Process -Id $ProcessId -ErrorAction SilentlyContinue) }
function Invoke-BoundedCommand([string]$Name, [string]$FilePath, [string[]]$Arguments, [int]$TimeoutSeconds) {
    $safe = ($Name -replace '[^A-Za-z0-9_.-]', '_'); $stdout = Join-Path $commandDirectory "$safe.stdout.log"; $stderr = Join-Path $commandDirectory "$safe.stderr.log"
    $started = Get-Date; $process = Start-Process -FilePath $FilePath -ArgumentList $Arguments -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $timedOut = -not $process.WaitForExit($TimeoutSeconds * 1000); $cleanup = $true; $exitCode = $null
    if ($timedOut) { $cleanup = Stop-Tree $process.Id } else { $process.WaitForExit(); $exitCode = [int]$process.ExitCode }
    $record = [ordered]@{name=$Name; file=$FilePath; arguments=$Arguments; started_at=$started.ToString('o'); finished_at=(Get-Date).ToString('o'); process_id=$process.Id; timeout_seconds=$TimeoutSeconds; timed_out=$timedOut; exit_code=$exitCode; stdout=$stdout; stderr=$stderr; cleanup_succeeded=$cleanup}
    $results.commands += [pscustomobject]$record; Save-Result; return [pscustomobject]$record
}
function Test-CleanDataPlane {
    $names = @('Unbound','winws2','Happ','xray','sing-box')
    $running = @(Get-Process -Name $names -ErrorAction SilentlyContinue | Select-Object ProcessName,Id)
    $proxy = Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -ErrorAction Stop
    $proxyProperty = $proxy.PSObject.Properties['ProxyEnable']
    $proxyEnabled = $null -ne $proxyProperty -and [bool]$proxyProperty.Value
    return [pscustomobject]@{clean=($running.Count -eq 0 -and -not $proxyEnabled); running=$running; proxy_enabled=$proxyEnabled}
}
function Invoke-Diagnostic([string]$Name, [string]$Command) {
    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes("`$ErrorActionPreference='Stop'; & { $Command } | ConvertTo-Json -Depth 6 -Compress"))
    $result = Invoke-BoundedCommand "diagnostic-$Name" 'powershell.exe' @('-NoProfile','-NonInteractive','-EncodedCommand',$encoded) $DiagnosticTimeoutSeconds
    return [pscustomobject]@{name=$Name; status=if($result.timed_out){'TIMEOUT'}elseif($result.exit_code -eq 0){'PASS'}else{'PARTIAL'}; command=$result}
}
function Test-CommandStage([string]$Stage, [string]$Name, [string]$Exe, [string[]]$Arguments, [int]$Timeout) {
    $command = Invoke-BoundedCommand $Name $Exe $Arguments $Timeout
    if ($command.timed_out) { Set-Stage $Stage 'TIMEOUT' "command timed out; stdout=$($command.stdout); stderr=$($command.stderr)"; return $false }
    if ($command.exit_code -ne 0) { Set-Stage $Stage 'FAIL' "exit=$($command.exit_code); stdout=$($command.stdout); stderr=$($command.stderr)"; return $false }
    Set-Stage $Stage 'PASS' "exit=0; stdout=$($command.stdout); stderr=$($command.stderr)"; return $true
}

try {
    Expand-Archive -Path $archive -DestinationPath $extract -Force
    $exe = Join-Path $extract 'Unbound.exe'; $candidate = Get-Content (Join-Path $extract 'CANDIDATE.json') -Raw | ConvertFrom-Json
    $manifestOK = $true; foreach ($line in Get-Content (Join-Path $extract 'BUNDLE_SHA256SUMS.txt')) { if ($line -match '^([0-9a-fA-F]{64})\s\s(.+)$') { $path = Join-Path $extract $matches[2]; $manifestOK = $manifestOK -and (Test-Path $path) -and ((Get-FileHash $path -Algorithm SHA256).Hash.ToLower() -eq $matches[1].ToLower()) } }
    $identity = (& $exe --version --json | Out-String | ConvertFrom-Json); $exeHash=(Get-FileHash $exe -Algorithm SHA256).Hash.ToLower()
    $identityOK = $manifestOK -and $candidate.version -eq $ExpectedVersion -and $candidate.candidate_commit -eq $ExpectedCommit -and $candidate.exe_sha256 -eq $exeHash -and $identity.version -eq $ExpectedVersion -and $identity.commit -eq $ExpectedCommit -and -not $identity.dirty -and $identity.channel -eq 'release' -and $identity.os -eq 'windows'
    Set-Stage ARTIFACT_IDENTITY $(if($identityOK){'PASS'}else{'FAIL'}) ($identity | ConvertTo-Json -Compress)
    $clean = Test-CleanDataPlane; Set-Stage CLEAN_DATA_PLANE $(if($clean.clean){'PASS'}else{'FAIL'}) ($clean | ConvertTo-Json -Compress)
    $diagnostics = @(
        Invoke-Diagnostic 'adapters' 'Get-NetAdapter | Select-Object Name,Status,ifIndex'
        Invoke-Diagnostic 'ip_configuration' 'Get-NetIPConfiguration | Select-Object InterfaceAlias,InterfaceIndex'
        Invoke-Diagnostic 'routes' "Get-NetRoute -AddressFamily IPv4 | Where-Object DestinationPrefix -eq '0.0.0.0/0' | Select-Object InterfaceAlias,NextHop"
        Invoke-Diagnostic 'dns' 'Get-DnsClientServerAddress | Select-Object InterfaceAlias,InterfaceIndex,AddressFamily,ServerAddresses'
    )
    $results.diagnostic_snapshot.commands = $diagnostics; $results.diagnostic_snapshot.status = if(@($diagnostics | Where-Object status -eq 'TIMEOUT').Count){'TIMEOUT'}elseif(@($diagnostics | Where-Object status -ne 'PASS').Count){'PARTIAL'}else{'PASS'}; Save-Result
    [void](Test-CommandStage KERNEL_ACCEPTANCE 'kernel' $exe @('--acceptance-test') 120)
    $profiles = @('rec','universal'); $profileOK = $true; foreach($profile in $profiles) { $record=Invoke-BoundedCommand "profile-$profile" $exe @('--cli','--profile',$profile,'--run-duration=20s') 75; $alive=@(Get-Process winws2 -ErrorAction SilentlyContinue).Count; if($record.timed_out -or $record.exit_code -ne 0 -or $alive){$profileOK=$false} }
    Set-Stage PROFILE_LIFECYCLE $(if($profileOK){'PASS'}else{'FAIL'}) 'Recommended and Universal bounded lifecycle records retained.'
    $recommended=Invoke-BoundedCommand 'recommended-smoke' $exe @('--cli','--profile','rec','--run-duration=30s') 90; Set-Stage RECOMMENDED_SMOKE $(if(!$recommended.timed_out -and $recommended.exit_code -eq 0){'PASS'}elseif($recommended.timed_out){'TIMEOUT'}else{'FAIL'}) "stdout=$($recommended.stdout); stderr=$($recommended.stderr)"
    [void](Test-CommandStage DOCTOR 'doctor' $exe @('--test') 120)
    $autotune=Invoke-BoundedCommand 'autotune' $exe @('--cli','--autotune','--run-duration=20s') $AutoTuneTimeoutSeconds; $autoText=if(Test-Path $autotune.stdout){Get-Content $autotune.stdout -Raw}else{''}; $autoOK=!$autotune.timed_out -and $autotune.exit_code -eq 0 -and @([regex]::Matches($autoText,'(?m)^AUTOTUNE_RESULT_JSON=' )).Count -eq 1; Set-Stage AUTOTUNE $(if($autoOK){'PASS'}elseif($autotune.timed_out){'TIMEOUT'}else{'FAIL'}) "stdout=$($autotune.stdout); stderr=$($autotune.stderr)"
    $launcherOK=$true; foreach($launcher in Get-ChildItem $extract -Filter '*.cmd') { $record=Invoke-BoundedCommand "launcher-$($launcher.BaseName)" $env:ComSpec @('/d','/c',("`"$($launcher.FullName)`"")) 20; if($record.timed_out){$launcherOK=$false}; Get-Process Unbound,winws2 -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue }; Set-Stage LAUNCHERS $(if($launcherOK){'PASS'}else{'FAIL'}) 'Individual launcher records retained.'
} catch { $results.harness_error=$_.Exception.Message; foreach($stage in 'ARTIFACT_IDENTITY','CLEAN_DATA_PLANE','KERNEL_ACCEPTANCE','PROFILE_LIFECYCLE','RECOMMENDED_SMOKE','DOCTOR','AUTOTUNE','LAUNCHERS'){if(-not $results.stages.Contains($stage)){Set-Stage $stage 'FAIL' "harness error: $($_.Exception.Message)"}} } finally {
    Get-Process Unbound,winws2 -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    $left=@(Get-Process Unbound,winws2 -ErrorAction SilentlyContinue); Set-Stage CLEANUP $(if($left.Count -eq 0){'PASS'}else{'FAIL'}) ($left | ConvertTo-Json -Compress)
    $critical=@('ARTIFACT_IDENTITY','CLEAN_DATA_PLANE','KERNEL_ACCEPTANCE','PROFILE_LIFECYCLE','RECOMMENDED_SMOKE','DOCTOR','AUTOTUNE','LAUNCHERS','CLEANUP'); $results.verdict=if(@($critical|Where-Object{$results.stages[$_].status -ne 'PASS'}).Count -eq 0){'PASS'}else{'FAIL'}; $results.finished_at=(Get-Date).ToString('o'); Save-Result
}
if($results.verdict -ne 'PASS'){exit 1}
