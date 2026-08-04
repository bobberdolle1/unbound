# UNBOUND Controlled Network Validation Script (No Xray VPN)
# Usage: powershell -ExecutionPolicy Bypass -File .\scripts\windows\validate-network-without-vpn.ps1

param(
    [string]$ExePath = "F:\Projects\Unbound\build\bin\Unbound.exe",
    [string]$ReportPath = "F:\Projects\Unbound\build\windows-network-validation-report.json",
    [string]$ProfileName = "Recommended (hostfakesplit)"
)

$ErrorActionPreference = "Continue"

$Targets = @(
    @{ Name = "YouTube"; URL = "https://www.youtube.com" },
    @{ Name = "Discord"; URL = "https://discord.com" },
    @{ Name = "Instagram"; URL = "https://www.instagram.com" },
    @{ Name = "Cloudflare"; URL = "https://www.cloudflare.com" },
    @{ Name = "Ozon"; URL = "https://www.ozon.ru" }
)

function Probe-Targets {
    $results = @()
    foreach ($target in $Targets) {
        $sw = [System.Diagnostics.Stopwatch]::StartNew()
        $status = "FAIL"
        $statusCode = 0
        $errMessage = ""
        try {
            Clear-DnsClientCache -ErrorAction SilentlyContinue | Out-Null
            $req = [System.Net.HttpWebRequest]::Create($target.URL)
            $req.Timeout = 4000
            $req.ReadWriteTimeout = 4000
            $req.KeepAlive = $false
            $req.AllowAutoRedirect = $true
            $req.MaximumAutomaticRedirections = 5
            $req.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0"
            $req.Method = "GET"
            $resp = $req.GetResponse()
            $statusCode = [int]$resp.StatusCode
            if ($statusCode -ge 200 -and $statusCode -lt 400) {
                $status = "OK"
            }
            $resp.Close()
        } catch {
            $errMessage = $_.Exception.Message
            if ($_.Exception.Response) {
                $statusCode = [int]$_.Exception.Response.StatusCode
            }
        }
        $sw.Stop()
        $results += [PSCustomObject]@{
            Name       = $target.Name
            URL        = $target.URL
            Status     = $status
            StatusCode = $statusCode
            LatencyMs  = $sw.ElapsedMilliseconds
            Error      = $errMessage
        }
    }
    return $results
}

function Get-SystemState {
    $winwsProc = Get-Process -Name winws2 -ErrorAction SilentlyContinue
    $unboundProc = Get-Process -Name Unbound -ErrorAction SilentlyContinue
    $driver = Get-Service -Name WinDivert -ErrorAction SilentlyContinue

    return [PSCustomObject]@{
        Timestamp     = (Get-Date).ToString("o")
        WinwsActive   = ($winwsProc -ne $null)
        WinwsPIDs     = if ($winwsProc) { ($winwsProc | Select-Object -ExpandProperty Id) -join ", " } else { "" }
        UnboundActive = ($unboundProc -ne $null)
        UnboundPIDs   = if ($unboundProc) { ($unboundProc | Select-Object -ExpandProperty Id) -join ", " } else { "" }
        WinDivertState= if ($driver) { $driver.Status.ToString() } else { "NotLoaded" }
    }
}

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host " 🚀 UNBOUND Autonomous Controlled Network Validation Suite " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "Exe Path: $ExePath"
Write-Host "Profile:  $ProfileName"
Write-Host "Report:   $ReportPath"
Write-Host "==========================================================" -ForegroundColor Cyan

$ReportData = [ordered]@{
    Version   = "0.3.0-rc.3"
    StartTime = (Get-Date).ToString("o")
    Phases    = @{}
}

# PHASE A: Xray ON, UNBOUND OFF (Control Reference Point)
Write-Host "`n[PHASE A] Measuring Reference Point (Xray ON, UNBOUND OFF)..." -ForegroundColor Yellow
$ReportData.Phases["PhaseA_XrayON_UnboundOFF"] = [ordered]@{
    Description = "Control reference with Xray VPN active"
    SystemState = Get-SystemState
    Probes      = Probe-Targets
}

Write-Host "`n----------------------------------------------------------" -ForegroundColor Red
Write-Host " ⚠️  ACTION REQUIRED: PLEASE TURN OFF XRAY VPN NOW!" -ForegroundColor Red
Write-Host " ⚠️  Disconnect Xray / v2rayN / v2rayA before proceeding." -ForegroundColor Red
Write-Host "----------------------------------------------------------" -ForegroundColor Red
Read-Host "Press ENTER after Xray VPN is completely DISCONNECTED"

# PHASE B: Xray OFF, UNBOUND OFF (Baseline)
Write-Host "`n[PHASE B] Measuring Baseline (Xray OFF, UNBOUND OFF)..." -ForegroundColor Yellow
$ReportData.Phases["PhaseB_XrayOFF_UnboundOFF"] = [ordered]@{
    Description = "Baseline without VPN or UNBOUND bypass"
    SystemState = Get-SystemState
    Probes      = Probe-Targets
}

# PHASE C: Xray OFF, UNBOUND ON (Primary Proof Test)
Write-Host "`n[PHASE C] Launching UNBOUND engine as Administrator..." -ForegroundColor Yellow
# Run as Administrator via UAC Verb RunAs
$unboundJob = Start-Process -FilePath $ExePath -ArgumentList "--cli --profile=`"$ProfileName`" --run-duration=25s" -Verb RunAs -PassThru

Start-Sleep -Seconds 5
$stateC = Get-SystemState
Write-Host "  Engine Status: winws2 Active = $($stateC.WinwsActive), WinDivert = $($stateC.WinDivertState)"

$ReportData.Phases["PhaseC_XrayOFF_UnboundON"] = [ordered]@{
    Description = "UNBOUND desync bypass active without VPN"
    SystemState = $stateC
    Probes      = Probe-Targets
}

Write-Host "  Waiting for UNBOUND process duration to finish..."
$unboundJob | Wait-Process -Timeout 30 -ErrorAction SilentlyContinue

# PHASE D: Xray OFF, UNBOUND OFF Post-Stop (Cleanup & Baseline Return)
Start-Sleep -Seconds 3
Write-Host "`n[PHASE D] Measuring Post-Stop Return to Baseline (Xray OFF, UNBOUND OFF)..." -ForegroundColor Yellow
$stateD = Get-SystemState
Write-Host "  Post-Stop Status: winws2 Active = $($stateD.WinwsActive), WinDivert = $($stateD.WinDivertState)"

$ReportData.Phases["PhaseD_XrayOFF_UnboundOFF_PostStop"] = [ordered]@{
    Description = "Return to baseline after UNBOUND stop & cleanup"
    SystemState = $stateD
    Probes      = Probe-Targets
}

$ReportData.EndTime = (Get-Date).ToString("o")

# Save JSON Report
$jsonContent = $ReportData | ConvertTo-Json -Depth 5
[System.IO.File]::WriteAllText($ReportPath, $jsonContent)

Write-Host "`n----------------------------------------------------------" -ForegroundColor Green
Write-Host " ✅ Validation complete! Report written to $ReportPath" -ForegroundColor Green
Write-Host " ⚠️  ACTION REQUIRED: PLEASE RECONNECT XRAY VPN NOW!" -ForegroundColor Yellow
Write-Host "----------------------------------------------------------" -ForegroundColor Green
Read-Host "Press ENTER after Xray VPN is RECONNECTED to resume agent communication"
