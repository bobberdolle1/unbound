#!/usr/bin/env pwsh
#Requires -RunAsAdministrator

$ErrorActionPreference = "Stop"

$REFERENCE_DIR = "F:\Projects\Unbound\reference\zapret2-youtube-discord-main"
$WINWS_PATH = Join-Path $REFERENCE_DIR "exe\winws2.exe"
$PRESET_DIR = Join-Path $REFERENCE_DIR "presets"
$RESULTS_FILE = "test_results.json"

Write-Host "=== AUTOMATED ZAPRET2 PROFILE TESTER ===" -ForegroundColor Cyan
Write-Host ""

if (-not (Test-Path $WINWS_PATH)) {
    Write-Host "ERROR: winws2.exe not found" -ForegroundColor Red
    exit 1
}

$testProfiles = @(
    "Default.txt",
    "general ALT.txt",
    "general FAKE TLS AUTO.txt",
    "general ALT2.txt",
    "general ALT3.txt",
    "Default v2 (game filter).txt",
    "Default v3 (game filter).txt"
)

$testUrls = @(
    "https://www.youtube.com",
    "https://discord.com",
    "https://www.google.com",
    "https://www.cloudflare.com"
)

$results = @()

foreach ($profileName in $testProfiles) {
    $presetPath = Join-Path $PRESET_DIR $profileName
    
    if (-not (Test-Path $presetPath)) {
        Write-Host "SKIP: $profileName (not found)" -ForegroundColor Yellow
        continue
    }
    
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "Testing: $profileName" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
    
    $args = Get-Content $presetPath | Where-Object {$_ -notmatch "^#" -and $_.Trim() -ne ""}
    
    try {
        $process = Start-Process -FilePath $WINWS_PATH -ArgumentList $args -WorkingDirectory $REFERENCE_DIR -PassThru -NoNewWindow -RedirectStandardError "nul"
        
        Write-Host "Started PID: $($process.Id)" -ForegroundColor Green
        Start-Sleep -Seconds 4
        
        $profileResults = @{
            Profile = $profileName
            Success = 0
            Failed = 0
            Tests = @()
        }
        
        foreach ($url in $testUrls) {
            try {
                $start = Get-Date
                $response = Invoke-WebRequest -Uri $url -TimeoutSec 8 -UseBasicParsing -ErrorAction Stop
                $elapsed = [math]::Round(((Get-Date) - $start).TotalMilliseconds, 0)
                
                if ($response.StatusCode -eq 200) {
                    Write-Host "  [OK] $url - ${elapsed}ms" -ForegroundColor Green
                    $profileResults.Success++
                    $profileResults.Tests += @{URL=$url; Status="OK"; Latency=$elapsed}
                } else {
                    Write-Host "  [FAIL] $url - HTTP $($response.StatusCode)" -ForegroundColor Red
                    $profileResults.Failed++
                    $profileResults.Tests += @{URL=$url; Status="FAIL"; Error="HTTP $($response.StatusCode)"}
                }
            } catch {
                Write-Host "  [FAIL] $url - $($_.Exception.Message)" -ForegroundColor Red
                $profileResults.Failed++
                $profileResults.Tests += @{URL=$url; Status="FAIL"; Error=$_.Exception.Message}
            }
            
            Start-Sleep -Milliseconds 500
        }
        
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        Start-Sleep -Milliseconds 500
        
        $results += $profileResults
        
        $successRate = [math]::Round(($profileResults.Success / $testUrls.Count) * 100, 0)
        Write-Host ""
        Write-Host "Result: $($profileResults.Success)/$($testUrls.Count) passed (${successRate}%)" -ForegroundColor $(if($successRate -ge 75){"Green"}elseif($successRate -ge 50){"Yellow"}else{"Red"})
        
    } catch {
        Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
        if ($process) {
            Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        }
    }
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "SUMMARY" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

$results | Sort-Object -Property Success -Descending | ForEach-Object {
    $rate = [math]::Round(($_.Success / $testUrls.Count) * 100, 0)
    $color = if($rate -ge 75){"Green"}elseif($rate -ge 50){"Yellow"}else{"Red"}
    Write-Host "$($_.Profile): $($_.Success)/$($testUrls.Count) (${rate}%)" -ForegroundColor $color
}

$results | ConvertTo-Json -Depth 10 | Out-File $RESULTS_FILE -Encoding UTF8
Write-Host ""
Write-Host "Results saved to: $RESULTS_FILE" -ForegroundColor Cyan

$bestProfile = $results | Sort-Object -Property Success -Descending | Select-Object -First 1
if ($bestProfile.Success -gt 0) {
    Write-Host ""
    Write-Host "BEST PROFILE: $($bestProfile.Profile)" -ForegroundColor Green
}
