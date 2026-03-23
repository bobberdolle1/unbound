# Find Best Working Profiles
# Tests all reference profiles and identifies top performers

param(
    [int]$TopN = 5,
    [int]$TimeoutSec = 6
)

$ErrorActionPreference = "Continue"

$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[ERROR] Administrator rights required" -ForegroundColor Red
    exit 1
}

$projectRoot = Split-Path -Parent $PSScriptRoot
$presetsDir = Join-Path $projectRoot "reference\zapret2-youtube-discord-main\presets"
$winwsPath = Join-Path $projectRoot "engine\core_bin\winws2.exe"
$refDir = Resolve-Path (Join-Path $projectRoot "reference\zapret2-youtube-discord-main")

$targets = @(
    @{ Name = "Discord API"; Url = "https://discord.com/api/v10/gateway" },
    @{ Name = "Discord CDN"; Url = "https://cdn.discordapp.com/embed/avatars/0.png" },
    @{ Name = "YouTube"; Url = "https://www.youtube.com" },
    @{ Name = "GoogleVideo"; Url = "https://redirector.googlevideo.com" },
    @{ Name = "Facebook"; Url = "https://www.facebook.com" },
    @{ Name = "Instagram"; Url = "https://www.instagram.com" },
    @{ Name = "Twitter"; Url = "https://twitter.com" },
    @{ Name = "Telegram Web"; Url = "https://web.telegram.org" },
    @{ Name = "WhatsApp Web"; Url = "https://web.whatsapp.com" },
    @{ Name = "Cloudflare"; Url = "https://www.cloudflare.com" }
)

function Stop-AllWinws2 {
    Get-Process -Name "winws2" -ErrorAction SilentlyContinue | Stop-Process -Force
    Start-Sleep -Milliseconds 500
}

function Test-Profile {
    param([string]$PresetPath, [string]$PresetName)
    
    Stop-AllWinws2
    
    # Build args
    $content = Get-Content $PresetPath
    $args = @()
    foreach ($line in $content) {
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
        $proc = Start-Process -FilePath $winwsPath -ArgumentList $args -WorkingDirectory $refDir -PassThru -WindowStyle Hidden -ErrorAction Stop
        Start-Sleep -Seconds 3
        
        $running = Get-Process -Id $proc.Id -ErrorAction SilentlyContinue
        if (-not $running) {
            return @{ Name = $PresetName; Score = 0; Results = @(); Failed = $true }
        }
        
        # Test targets
        $results = @()
        $score = 0
        
        foreach ($target in $targets) {
            $success = $false
            try {
                $response = Invoke-WebRequest -Uri $target.Url -TimeoutSec $TimeoutSec -UseBasicParsing -ErrorAction Stop
                $success = ($response.StatusCode -eq 200)
                if ($success) { $score += 5 }
            } catch {}
            
            $results += @{ Target = $target.Name; Success = $success }
        }
        
        Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
        
        return @{ Name = $PresetName; Score = $score; Results = $results; Failed = $false }
        
    } catch {
        return @{ Name = $PresetName; Score = 0; Results = @(); Failed = $true }
    }
}

# Get all presets
$presets = Get-ChildItem -Path $presetsDir -Filter "*.txt" | Where-Object { 
    $_.Name -notlike "*game filter*" 
} | Sort-Object Name

Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "         FINDING BEST PROFILES" -ForegroundColor Cyan
Write-Host "         Testing: $($presets.Count) profiles" -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host ""

$allResults = @()
$current = 0

foreach ($preset in $presets) {
    $current++
    Write-Host "[$current/$($presets.Count)] Testing: $($preset.BaseName)" -ForegroundColor Yellow -NoNewline
    
    $result = Test-Profile -PresetPath $preset.FullName -PresetName $preset.BaseName
    $allResults += $result
    
    if ($result.Failed) {
        Write-Host " [FAILED]" -ForegroundColor Red
    } else {
        $successCount = ($result.Results | Where-Object { $_.Success }).Count
        $color = if ($successCount -ge 7) { "Green" } elseif ($successCount -ge 5) { "Yellow" } else { "Red" }
        Write-Host " [$successCount/10]" -ForegroundColor $color
    }
}

Stop-AllWinws2

# Sort by score
$sorted = $allResults | Where-Object { -not $_.Failed } | Sort-Object -Property Score -Descending

Write-Host ""
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "                   TOP $TopN PROFILES" -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host ""

$topProfiles = $sorted | Select-Object -First $TopN

foreach ($profile in $topProfiles) {
    $successCount = ($profile.Results | Where-Object { $_.Success }).Count
    Write-Host "$($profile.Name)" -ForegroundColor Green
    Write-Host "  Score: $($profile.Score)/50  Success: $successCount/10" -ForegroundColor Cyan
    foreach ($res in $profile.Results) {
        $status = if ($res.Success) { "[OK]" } else { "[FAIL]" }
        $color = if ($res.Success) { "Green" } else { "Red" }
        Write-Host "    $status $($res.Target)" -ForegroundColor $color
    }
    Write-Host ""
}

# Save results
$resultsPath = Join-Path $projectRoot "scripts\best_profiles.txt"
$output = @()
$output += "TOP $TopN WORKING PROFILES"
$output += "=" * 60
$output += ""
foreach ($profile in $topProfiles) {
    $successCount = ($profile.Results | Where-Object { $_.Success }).Count
    $output += $profile.Name
    $output += "  Score: $($profile.Score)/50  Success: $successCount/10"
}
$output | Out-File $resultsPath -Encoding UTF8

Write-Host "Results saved to: $resultsPath" -ForegroundColor Cyan
