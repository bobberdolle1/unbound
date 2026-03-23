#!/usr/bin/env pwsh
#Requires -RunAsAdministrator

$ErrorActionPreference = "Stop"

$WINWS_PATH = "engine\core_bin\winws2.exe"
$LISTS_DIR = "engine\lists"
$BIN_DIR = "engine\core_bin"
$WINDIVERT_DIR = "engine\windivert.filter"

Write-Host "=== UNIVERSAL 2026 BYPASS TEST ===" -ForegroundColor Cyan
Write-Host ""

if (-not (Test-Path $WINWS_PATH)) {
    Write-Host "ERROR: winws2.exe not found" -ForegroundColor Red
    exit 1
}

# Resolve paths
$listsDir = (Resolve-Path $LISTS_DIR).Path -replace '\\', '/'
$windivertDir = (Resolve-Path $WINDIVERT_DIR).Path -replace '\\', '/'

$args = @(
    "--wf-l3=ipv4,ipv6",
    "--lua-init=@$((Resolve-Path ""engine\lua_scripts\zapret-antidpi.lua"").Path -replace '\\', '/')",
    "--blob=tls_google:@$((Resolve-Path ""engine\core_bin\tls_clienthello_www_google_com.bin"").Path -replace '\\', '/')",
    "--blob=quic_google:@$((Resolve-Path ""engine\core_bin\quic_initial_www_google_com.bin"").Path -replace '\\', '/')",
    "--ctrack-disable=0",
    "--ipcache-lifetime=8400",
    
    "--wf-tcp-out=80,443,1080,2053,2083,2087,2096,8443",
    "--wf-udp-out=443,19294-19344,50000-50100",
    "--wf-raw-part=@$windivertDir/windivert_part.discord_media.txt",
    "--wf-raw-part=@$windivertDir/windivert_part.stun.txt",
    "--wf-raw-part=@$windivertDir/windivert_part.wireguard.txt",
    "--filter-udp=443,50000-50100",
    "--filter-l7=discord,stun",
    "--payload=stun,discord_ip_discovery",
    "--lua-desync=fake:blob=fake_default_udp:repeats=6",
    "--new",
    "--filter-tcp=80,443,1080,2053,2083,2087,2096,8443",
    "--hostlist=$listsDir/discord.txt",
    "--hostlist-domains=discord.media",
    "--lua-desync=fake:blob=tls_google:repeats=6:tcp_seq=2",
    "--lua-desync=fakedsplit:pos=2:repeats=8:tcp_seq=2",
    "--new",
    "--filter-udp=443",
    "--hostlist=$listsDir/youtube.txt",
    "--hostlist-domains=googlevideo.com",
    "--payload=quic_initial",
    "--lua-desync=fake:blob=quic_google:repeats=6",
    "--new",
    "--filter-tcp=80,443",
    "--hostlist=$listsDir/youtube.txt",
    "--hostlist-domains=googlevideo.com",
    "--out-range=-d8",
    "--lua-desync=hostfakesplit:host=ozon.ru:tcp_ts=-1000:tcp_md5:repeats=4",
    "--new",
    "--filter-tcp=443",
    "--ipset=$listsDir/ipset-telegram.txt",
    "--lua-desync=fakedsplit:pos=2:repeats=6:tcp_seq=2",
    "--new",
    "--filter-tcp=443",
    "--ipset=$listsDir/ipset-twitter.txt",
    "--ipset=$listsDir/ipset-facebook.txt",
    "--ipset=$listsDir/ipset-instagram.txt",
    "--ipset=$listsDir/ipset-whatsapp.txt",
    "--payload=tls_client_hello",
    "--lua-desync=fake:blob=fake_default_tls:tls_mod=rnd,dupsid,sni=www.google.com:repeats=8:tcp_seq=2",
    "--lua-desync=fakedsplit:pos=2:repeats=8:tcp_seq=2",
    "--new",
    "--filter-udp=443",
    "--ipset=$listsDir/ipset-all.txt",
    "--ipset-exclude=$listsDir/ipset-exclude.txt",
    "--payload=quic_initial",
    "--lua-desync=fake:blob=quic_google:repeats=6",
    "--new",
    "--filter-tcp=80,443",
    "--ipset=$listsDir/ipset-all.txt",
    "--ipset-exclude=$listsDir/ipset-exclude.txt",
    "--payload=tls_client_hello",
    "--lua-desync=fake:blob=fake_default_tls:tls_mod=rnd,dupsid,sni=www.google.com:repeats=8:tcp_seq=2",
    "--lua-desync=fakedsplit:pos=2:repeats=8:tcp_seq=2"
)

Write-Host "Starting winws2.exe with Universal 2026 Profile..." -ForegroundColor Yellow
$process = Start-Process -FilePath $WINWS_PATH -ArgumentList $args -PassThru -WindowStyle Hidden

Write-Host "Engine started (PID: $($process.Id)). Waiting 5s for initialization..." -ForegroundColor Green
Start-Sleep -Seconds 5

Write-Host ""
Write-Host "Testing Targets..." -ForegroundColor Yellow

$testUrls = @(
    "https://www.youtube.com",
    "https://discord.com/api/v10/gateway",
    "https://x.com",
    "https://www.facebook.com",
    "https://web.telegram.org",
    "https://www.whatsapp.com"
)

$success = 0
foreach ($url in $testUrls) {
    try {
        $start = Get-Date
        $response = Invoke-WebRequest -Uri $url -TimeoutSec 10 -UseBasicParsing
        $elapsed = [math]::Round(((Get-Date) - $start).TotalMilliseconds, 0)
        
        if ($response.StatusCode -eq 200) {
            Write-Host "[OK] $url - ${elapsed}ms" -ForegroundColor Green
            $success++
        } else {
            Write-Host "[WARN] $url - HTTP $($response.StatusCode)" -ForegroundColor Yellow
        }
    } catch {
        Write-Host "[FAIL] $url - $($_.Exception.Message)" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "Result: $success/$($testUrls.Count) passed" -ForegroundColor $(if($success -eq $testUrls.Count){"Green"}else{"Yellow"})

Write-Host "Stopping Engine..." -ForegroundColor Yellow
Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
Write-Host "Engine stopped." -ForegroundColor Green
