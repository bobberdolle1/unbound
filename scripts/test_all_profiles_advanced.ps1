# Advanced Profile Testing Script
# Based on zapret2-youtube-discord-main testing methodology

param(
    [int]$TimeoutSec = 5,
    [int]$MaxParallel = 8,
    [switch]$QuickTest
)

$ErrorActionPreference = "Continue"

# Check admin rights
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[ERROR] Administrator rights required" -ForegroundColor Red
    exit 1
}

$projectRoot = Split-Path -Parent $PSScriptRoot
$presetsDir = Join-Path $projectRoot "reference\zapret2-youtube-discord-main\presets"
$winwsPath = Join-Path $projectRoot "engine\core_bin\winws2.exe"
$refDir = Join-Path $projectRoot "reference\zapret2-youtube-discord-main"
$resultsDir = Join-Path $projectRoot "scripts\test-results"

if (-not (Test-Path $resultsDir)) { New-Item -ItemType Directory -Path $resultsDir | Out-Null }

# Test targets
$targets = @(
    @{ Name = "Discord"; Url = "https://discord.com/api/v10/gateway"; Host = "discord.com" },
    @{ Name = "DiscordCDN"; Url = "https://cdn.discordapp.com/embed/avatars/0.png"; Host = "cdn.discordapp.com" },
    @{ Name = "YouTube"; Url = "https://www.youtube.com"; Host = "www.youtube.com" },
    @{ Name = "GoogleVideo"; Url = "https://redirector.googlevideo.com"; Host = "redirector.googlevideo.com" }
)

function Stop-Winws2 {
    Get-Process -Name "winws2" -ErrorAction SilentlyContinue | Stop-Process -Force
    Start-Sleep -Milliseconds 500
}

function Start-Winws2 {
    param([string]$PresetPath)
    
    # Read preset and build args with absolute paths
    $presetContent = Get-Content $PresetPath
    $args = @()
    
    foreach ($line in $presetContent) {
        $line = $line.Trim()
        if ($line -and -not $line.StartsWith("#")) {
            # Convert relative paths to absolute
            $line = $line -replace '@lua/', "@$(Resolve-Path $refDir)/lua/"
            $line = $line -replace '@bin/', "@$(Resolve-Path $refDir)/bin/"
            $line = $line -replace '@windivert\.filter/', "@$(Resolve-Path $refDir)/windivert.filter/"
            $line = $line -replace 'lists/', "$(Resolve-Path $refDir)/lists/"
            $args += $line
        }
    }
    
    try {
        $proc = Start-Process -FilePath $winwsPath -ArgumentList $args -WorkingDirectory (Resolve-Path $refDir) -PassThru -WindowStyle Hidden -ErrorAction Stop
        Start-Sleep -Seconds 4
        
        # Verify it's running
        $running = Get-Process -Id $proc.Id -ErrorAction SilentlyContinue
        if (-not $running) {
            Write-Host "  [!] Process started but immediately exited" -ForegroundColor Yellow
            return $null
        }
        
        return $proc
    } catch {
        Write-Host "  [!] Failed to start: $($_.Exception.Message)" -ForegroundColor Red
        return $null
    }
}

function Test-Targets {
    param([array]$TargetList, [int]$TimeoutSec = 5)
    
    $results = @()
    
    foreach ($target in $TargetList) {
        $testResult = @{
            Name = $target.Name
            HTTP = $false
            TLS12 = $false
            TLS13 = $false
            Ping = "n/a"
        }
        
        # HTTP test
        try {
            $response = Invoke-WebRequest -Uri $target.Url -TimeoutSec $TimeoutSec -UseBasicParsing -ErrorAction Stop
            $testResult.HTTP = ($response.StatusCode -eq 200)
        } catch {}
        
        # TLS 1.2 test (using curl if available)
        if (Get-Command "curl.exe" -ErrorAction SilentlyContinue) {
            try {
                $output = & curl.exe -I -s -m $TimeoutSec --tlsv1.2 --tls-max 1.2 -w "%{http_code}" -o NUL $target.Url 2>&1
                $testResult.TLS12 = ($LASTEXITCODE -eq 0)
            } catch {}
            
            try {
                $output = & curl.exe -I -s -m $TimeoutSec --tlsv1.3 --tls-max 1.3 -w "%{http_code}" -o NUL $target.Url 2>&1
                $testResult.TLS13 = ($LASTEXITCODE -eq 0)
            } catch {}
        }
        
        # Ping test
        try {
            $pings = Test-Connection -ComputerName $target.Host -Count 2 -ErrorAction Stop
            $avg = ($pings | Measure-Object -Property ResponseTime -Average).Average
            $testResult.Ping = "{0:N0}ms" -f $avg
        } catch {
            $testResult.Ping = "Timeout"
        }
        
        $results += $testResult
    }
    
    return $results
}

# Get preset files
$presetFiles = Get-ChildItem -Path $presetsDir -Filter "*.txt" | Where-Object { 
    $_.Name -notlike "*game filter*" 
} | Sort-Object Name | Select-Object -First $(if ($QuickTest) { 10 } else { 100 })

Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "         ZAPRET2 PROFILE TESTING" -ForegroundColor Cyan
Write-Host "         Profiles: $($presetFiles.Count)  |  Targets: $($targets.Count)" -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host ""

$globalResults = @()
$presetNum = 0

foreach ($preset in $presetFiles) {
    $presetNum++
    Write-Host "------------------------------------------------------------" -ForegroundColor DarkCyan
    Write-Host "  [$presetNum/$($presetFiles.Count)] $($preset.BaseName)" -ForegroundColor Yellow
    Write-Host "------------------------------------------------------------" -ForegroundColor DarkCyan
    
    Stop-Winws2
    Start-Sleep -Milliseconds 500
    
    Write-Host "  > Starting winws2..." -ForegroundColor DarkGray
    $proc = Start-Winws2 -PresetPath $preset.FullName
    
    if (-not $proc) {
        Write-Host "  [X] Failed to start winws2" -ForegroundColor Red
        $globalResults += @{ Preset = $preset.BaseName; Results = @(); Failed = $true }
        continue
    }
    
    # Check if running
    $running = Get-Process -Name "winws2" -ErrorAction SilentlyContinue
    if (-not $running) {
        Write-Host "  [X] winws2 not running" -ForegroundColor Red
        $globalResults += @{ Preset = $preset.BaseName; Results = @(); Failed = $true }
        continue
    }
    
    Write-Host "  > Testing targets..." -ForegroundColor DarkGray
    $testResults = Test-Targets -TargetList $targets -TimeoutSec $TimeoutSec
    
    # Display results
    foreach ($result in $testResults) {
        $status = ""
        $color = "Red"
        
        if ($result.HTTP) {
            $status = "HTTP:OK"
            $color = "Green"
        } else {
            $status = "HTTP:FAIL"
        }
        
        if ($result.TLS12) { $status += " TLS1.2:OK" } else { $status += " TLS1.2:FAIL" }
        if ($result.TLS13) { $status += " TLS1.3:OK" } else { $status += " TLS1.3:FAIL" }
        
        Write-Host "  $($result.Name.PadRight(15)) $status | Ping: $($result.Ping)" -ForegroundColor $color
    }
    
    $globalResults += @{ Preset = $preset.BaseName; Results = $testResults; Failed = $false }
    
    Stop-Winws2
    if ($proc -and -not $proc.HasExited) {
        Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    }
    
    Write-Host ""
}

# Analytics
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "                      RESULTS" -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host ""

$analytics = @{}
foreach ($res in $globalResults) {
    $name = $res.Preset
    $analytics[$name] = @{ OK = 0; FAIL = 0; LaunchFail = $res.Failed }
    
    foreach ($tr in $res.Results) {
        if ($tr.HTTP) { $analytics[$name].OK++ } else { $analytics[$name].FAIL++ }
    }
}

$workingProfiles = @()
foreach ($name in $analytics.Keys) {
    $a = $analytics[$name]
    if ($a.LaunchFail) {
        Write-Host "  $name : " -NoNewline
        Write-Host "FAILED TO START" -ForegroundColor Red
    } else {
        $score = $a.OK
        $color = if ($score -ge 3) { "Green" } elseif ($score -ge 2) { "Yellow" } else { "Red" }
        Write-Host "  $name : " -NoNewline
        Write-Host "OK=$($a.OK)/$($targets.Count)" -ForegroundColor $color
        
        if ($score -ge 3) {
            $workingProfiles += $name
        }
    }
}

Write-Host ""
Write-Host "Working Profiles: $($workingProfiles.Count)" -ForegroundColor Green
if ($workingProfiles.Count -gt 0) {
    Write-Host ""
    Write-Host "Top Working Profiles:" -ForegroundColor Green
    foreach ($profile in $workingProfiles | Select-Object -First 10) {
        Write-Host "  - $profile" -ForegroundColor Green
    }
}

# Save results
$dateStr = Get-Date -Format "yyyy-MM-dd_HH-mm-ss"
$resultFile = Join-Path $resultsDir "test_$dateStr.json"
$globalResults | ConvertTo-Json -Depth 10 | Out-File $resultFile
Write-Host ""
Write-Host "Results saved to: $resultFile" -ForegroundColor Cyan

Stop-Winws2
