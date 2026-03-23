#!/usr/bin/env pwsh
#Requires -RunAsAdministrator

$ErrorActionPreference = "Stop"

$WINWS_PATH = "engine\core_bin\winws2.exe"
$LUA_DIR = "engine\lua_scripts"
$LISTS_DIR = "engine\lists"
$BIN_DIR = "engine\core_bin"

Write-Host "=== DIRECT ENGINE TEST ===" -ForegroundColor Cyan
Write-Host ""

if (-not (Test-Path $WINWS_PATH)) {
    Write-Host "ERROR: winws2.exe not found" -ForegroundColor Red
    exit 1
}

$luaLib = (Resolve-Path "$LUA_DIR\zapret-lib.lua").Path -replace '\\', '/'
$luaAntiDpi = (Resolve-Path "$LUA_DIR\zapret-antidpi.lua").Path -replace '\\', '/'
$luaAuto = (Resolve-Path "$LUA_DIR\zapret-auto.lua").Path -replace '\\', '/'
$customFuncs = (Resolve-Path "$LUA_DIR\custom_funcs.lua").Path -replace '\\', '/'
$customDiag = (Resolve-Path "$LUA_DIR\custom_diag.lua").Path -replace '\\', '/'
$multishake = (Resolve-Path "$LUA_DIR\zapret-multishake.lua").Path -replace '\\', '/'

$tls_google = (Resolve-Path "$BIN_DIR\tls_clienthello_www_google_com.bin").Path -replace '\\', '/'
$quic_google = (Resolve-Path "$BIN_DIR\quic_initial_www_google_com.bin").Path -replace '\\', '/'

$youtube_list = (Resolve-Path "$LISTS_DIR\youtube.txt").Path -replace '\\', '/'
$discord_list = (Resolve-Path "$LISTS_DIR\discord.txt").Path -replace '\\', '/'
$other_list = (Resolve-Path "$LISTS_DIR\other.txt").Path -replace '\\', '/'
$ipset_all = (Resolve-Path "$LISTS_DIR\ipset-all.txt").Path -replace '\\', '/'
$ipset_exclude = (Resolve-Path "$LISTS_DIR\ipset-exclude.txt").Path -replace '\\', '/'

$args = @(
    "--wf-l3=ipv4,ipv6",
    "--lua-init=@$luaLib",
    "--lua-init=@$luaAntiDpi",
    "--lua-init=@$luaAuto",
    "--lua-init=@$customFuncs",
    "--lua-init=@$customDiag",
    "--lua-init=@$multishake",
    "--blob=tls_google:@$tls_google",
    "--blob=quic_google:@$quic_google",
    "--blob=fake_default_udp:0x00000000000000000000000000000000",
    "--ctrack-disable=0",
    "--ipcache-lifetime=8400",
    "--ipcache-hostname=1",
    "--wf-tcp-out=80,443,2053,2083,2087,2096,8443",
    "--wf-udp-out=443,19294-19344,50000-50100",
    "--filter-udp=443",
    "--ipset-exclude=$ipset_exclude",
    "--payload=quic_initial",
    "--lua-desync=fake:blob=quic_google:repeats=11",
    "--new",
    "--filter-l7=discord,stun",
    "--payload=stun,discord_ip_discovery",
    "--lua-desync=fake:blob=fake_default_udp:repeats=6",
    "--new",
    "--filter-tcp=80,443",
    "--hostlist-domains=googlevideo.com",
    "--out-range=-d8",
    "--lua-desync=fake:blob=0x00000000:repeats=11:tcp_ack=-66000:tcp_ts_up",
    "--lua-desync=fake:blob=fake_default_tls:tls_mod=rnd,dupsid,sni=www.google.com:repeats=11:tcp_ack=-66000:tcp_ts_up",
    "--lua-desync=multidisorder_legacy:pos=1,midsld:repeats=11",
    "--new",
    "--filter-tcp=80,443",
    "--hostlist=$youtube_list",
    "--out-range=-d8",
    "--lua-desync=fake:blob=0x00000000:repeats=11:tcp_ack=-66000:tcp_ts_up",
    "--lua-desync=fake:blob=fake_default_tls:tls_mod=rnd,dupsid,sni=www.google.com:repeats=11:tcp_ack=-66000:tcp_ts_up",
    "--lua-desync=multidisorder_legacy:pos=1,midsld:repeats=11",
    "--new",
    "--filter-tcp=80,443",
    "--hostlist=$discord_list",
    "--hostlist=$other_list",
    "--out-range=-d10",
    "--lua-desync=fake:blob=0x00000000:repeats=11:tcp_ack=-66000:tcp_ts_up",
    "--lua-desync=fake:blob=fake_default_tls:tls_mod=rnd,dupsid,sni=www.google.com:repeats=11:tcp_ack=-66000:tcp_ts_up",
    "--lua-desync=multidisorder_legacy:pos=1,midsld:repeats=11",
    "--new",
    "--filter-udp=443",
    "--ipset=$ipset_all",
    "--ipset-exclude=$ipset_exclude",
    "--lua-desync=fake:blob=quic_google:repeats=11",
    "--new",
    "--filter-tcp=80,443",
    "--ipset=$ipset_all",
    "--ipset-exclude=$ipset_exclude",
    "--lua-desync=fake:blob=0x00000000:repeats=11:tcp_ack=-66000:tcp_ts_up",
    "--lua-desync=fake:blob=fake_default_tls:tls_mod=rnd,dupsid,sni=www.google.com:repeats=11:tcp_ack=-66000:tcp_ts_up",
    "--lua-desync=multidisorder_legacy:pos=1,midsld:repeats=11"
)

Write-Host "Starting winws2.exe..." -ForegroundColor Yellow
$process = Start-Process -FilePath $WINWS_PATH -ArgumentList $args -PassThru -NoNewWindow

Write-Host "Engine started (PID: $($process.Id))" -ForegroundColor Green
Start-Sleep -Seconds 4

Write-Host ""
Write-Host "Testing URLs..." -ForegroundColor Yellow

$testUrls = @(
    "https://www.youtube.com",
    "https://discord.com",
    "https://www.google.com",
    "https://www.cloudflare.com"
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
        }
    } catch {
        Write-Host "[FAIL] $url - $($_.Exception.Message)" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "Result: $success/$($testUrls.Count) passed" -ForegroundColor $(if($success -eq $testUrls.Count){"Green"}else{"Yellow"})

Stop-Process -Id $process.Id -Force
Write-Host "Engine stopped" -ForegroundColor Yellow
