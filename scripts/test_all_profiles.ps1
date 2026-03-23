# Test all reference profiles automatically
# Finds working profiles for YouTube and Discord bypass

param(
    [int]$TestDuration = 10,
    [switch]$QuickTest
)

$ErrorActionPreference = "Stop"

# Check admin rights
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "ERROR: Administrator rights required" -ForegroundColor Red
    exit 1
}

$projectRoot = Split-Path -Parent $PSScriptRoot
$presetsDir = Join-Path $projectRoot "reference\zapret2-youtube-discord-main\presets"
$winwsPath = Join-Path $projectRoot "engine\core_bin\winws2.exe"
$refDir = Join-Path $projectRoot "reference\zapret2-youtube-discord-main"

# Get all preset files
$presets = Get-ChildItem -Path $presetsDir -Filter "*.txt" | Where-Object { 
    $_.Name -notlike "*game filter*" 
} | Select-Object -First $(if ($QuickTest) { 10 } else { 100 })

Write-Host "=== Testing $($presets.Count) Profiles ===" -ForegroundColor Cyan
Write-Host ""

$results = @()

foreach ($preset in $presets) {
    $profileName = $preset.BaseName
    Write-Host "Testing: $profileName" -ForegroundColor Yellow
    
    # Kill existing processes
    Get-Process -Name "winws2" -ErrorAction SilentlyContinue | Stop-Process -Force
    Start-Sleep -Milliseconds 500
    
    # Read preset and build args
    $presetContent = Get-Content $preset.FullName
    $args = @()
    
    foreach ($line in $presetContent) {
        $line = $line.Trim()
        if ($line -and -not $line.StartsWith("#")) {
            $line = $line -replace '@lua/', "@$refDir/lua/"
            $line = $line -replace '@bin/', "@$refDir/bin/"
            $line = $line -replace '@windivert\.filter/', "@$refDir/windivert.filter/"
            $line = $line -replace 'lists/', "$refDir/lists/"
            $args += $line
        }
    }
    
    # Start winws2
    try {
        $process = Start-Process -FilePath $winwsPath -ArgumentList $args -WorkingDirectory (Split-Path $winwsPath) -PassThru -WindowStyle Hidden -ErrorAction Stop
        Start-Sleep -Seconds 3
        
        # Test YouTube
        $ytPass = $false
        try {
            $ytResponse = Invoke-WebRequest -Uri "https://www.youtube.com" -TimeoutSec 5 -UseBasicParsing -ErrorAction Stop
            $ytPass = ($ytResponse.StatusCode -eq 200)
        } catch {}
        
        # Test Discord
        $discordPass = $false
        try {
            $discordResponse = Invoke-WebRequest -Uri "https://discord.com/api/v10/gateway" -TimeoutSec 5 -UseBasicParsing -ErrorAction Stop
            $discordPass = ($discordResponse.StatusCode -eq 200)
        } catch {}
        
        # Stop process
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        
        $result = [PSCustomObject]@{
            Profile = $profileName
            YouTube = $ytPass
            Discord = $discordPass
            Success = ($ytPass -and $discordPass)
        }
        
        $results += $result
        
        $status = if ($result.Success) { "[PASS]" } else { "[FAIL]" }
        $color = if ($result.Success) { "Green" } else { "Red" }
        Write-Host "  $status YT:$ytPass DC:$discordPass" -ForegroundColor $color
        
    } catch {
        Write-Host "  [ERROR] Failed to start: $($_.Exception.Message)" -ForegroundColor Red
    }
    
    Write-Host ""
}

# Summary
Write-Host "=== RESULTS ===" -ForegroundColor Cyan
Write-Host ""

$working = $results | Where-Object { $_.Success }
Write-Host "Working Profiles: $($working.Count)/$($results.Count)" -ForegroundColor Green
Write-Host ""

if ($working.Count -gt 0) {
    Write-Host "Top Working Profiles:" -ForegroundColor Green
    foreach ($profile in $working | Select-Object -First 10) {
        Write-Host "  - $($profile.Profile)" -ForegroundColor Green
    }
}

# Export results
$resultsPath = Join-Path $projectRoot "scripts\test_results.json"
$results | ConvertTo-Json | Out-File $resultsPath
Write-Host ""
Write-Host "Results saved to: $resultsPath" -ForegroundColor Cyan
