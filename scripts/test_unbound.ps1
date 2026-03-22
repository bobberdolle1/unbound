# Unbound Auto-Test Script
Write-Host "=== UNBOUND AUTO-TEST ===" -ForegroundColor Cyan
Write-Host ""

# 1. Check process
Write-Host "[1/5] Checking if Unbound is running..." -ForegroundColor Yellow
$process = Get-Process unbound -ErrorAction SilentlyContinue
if ($process) {
    Write-Host "OK Unbound running (PID: $($process.Id))" -ForegroundColor Green
} else {
    Write-Host "FAIL Unbound not running" -ForegroundColor Red
    Start-Process -FilePath ".\build\bin\unbound.exe"
    Start-Sleep -Seconds 3
}

# 2. Check logs
Write-Host ""
Write-Host "[2/5] Checking logs..." -ForegroundColor Yellow
$logPath = "$env:APPDATA\Unbound\unbound.log"
if (Test-Path $logPath) {
    Write-Host "OK Log file exists: $logPath" -ForegroundColor Green
    $lastLines = Get-Content $logPath -Tail 10
    Write-Host "Last 10 lines:" -ForegroundColor Gray
    $lastLines | ForEach-Object { Write-Host "  $_" -ForegroundColor DarkGray }
} else {
    Write-Host "FAIL Log file not found" -ForegroundColor Red
}

# 3. Check winws
Write-Host ""
Write-Host "[3/5] Checking winws.exe status..." -ForegroundColor Yellow
$winws = Get-Process winws -ErrorAction SilentlyContinue
if ($winws) {
    Write-Host "OK winws.exe is running (PID: $($winws.Id))" -ForegroundColor Green
} else {
    Write-Host "INFO winws.exe not running (engine stopped)" -ForegroundColor Gray
}

# 4. Check binaries
Write-Host ""
Write-Host "[4/5] Checking extracted binaries..." -ForegroundColor Yellow
$tempPath = "$env:TEMP\clearflow\core_bin"
if (Test-Path $tempPath) {
    $files = Get-ChildItem $tempPath -File
    Write-Host "OK Found $($files.Count) files in $tempPath" -ForegroundColor Green
    $files | ForEach-Object { Write-Host "  - $($_.Name)" -ForegroundColor DarkGray }
} else {
    Write-Host "FAIL Temp binaries not extracted" -ForegroundColor Red
}

# 5. Test connectivity
Write-Host ""
Write-Host "[5/5] Testing connectivity..." -ForegroundColor Yellow
try {
    $result = Test-NetConnection -ComputerName discord.com -Port 443 -WarningAction SilentlyContinue
    if ($result.TcpTestSucceeded) {
        Write-Host "OK discord.com:443 reachable" -ForegroundColor Green
    } else {
        Write-Host "FAIL discord.com:443 unreachable" -ForegroundColor Red
    }
} catch {
    Write-Host "FAIL Connectivity test failed" -ForegroundColor Red
}

Write-Host ""
Write-Host "=== TEST COMPLETE ===" -ForegroundColor Cyan
