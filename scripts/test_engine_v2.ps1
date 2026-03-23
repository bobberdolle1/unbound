#!/usr/bin/env pwsh
#Requires -RunAsAdministrator

$ErrorActionPreference = "Stop"

Write-Host "=== ENGINE TEST V2 ===" -ForegroundColor Cyan

$ENGINE_DIR = "engine\core_bin"
cd $ENGINE_DIR

$args = @(
    "--wf-l3=ipv4,ipv6",
    "--lua-init=@../lua_scripts/zapret-lib.lua",
    "--lua-init=@../lua_scripts/zapret-antidpi.lua",
    "--lua-init=@../lua_scripts/zapret-auto.lua",
    "--lua-init=@../lua_scripts/custom_funcs.lua",
    "--lua-init=@../lua_scripts/custom_diag.lua",
    "--lua-init=@../lua_scripts/zapret-multishake.lua",
    "--blob=tls_google:@tls_clienthello_www_google_com.bin",
    "--blob=quic_google:@quic_initial_www_google_com.bin",
    "--blob=fake_default_udp:0x00000000000000000000000000000000",
    "--ctrack-disable=0",
    "--ipcache-lifetime=8400",
    "--ipcache-hostname=1",
    "--wf-tcp-out=80,443,2053,2083,2087,2096,8443",
    "--wf-udp-out=443,19294-19344,50000-50100",
    "--filter-udp=443",
    "--ipset-exclude=../lists/ipset-exclude.txt",
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
    "--hostlist=../lists/youtube.txt",
    "--out-range=-d8",
    "--lua-desync=fake:blob=0x00000000:repeats=11:tcp_ack=-66000:tcp_ts_up",
    "--lua-desync=fake:blob=fake_default_tls:tls_mod=rnd,dupsid,sni=www.google.com:repeats=11:tcp_ack=-66000:tcp_ts_up",
    "--lua-desync=multidisorder_legacy:pos=1,midsld:repeats=11",
    "--new",
    "--filter-tcp=80,443",
    "--hostlist=../lists/discord.txt",
    "--out-range=-d10",
    "--lua-desync=fake:blob=0x00000000:repeats=11:tcp_ack=-66000:tcp_ts_up",
    "--lua-desync=fake:blob=fake_default_tls:tls_mod=rnd,dupsid,sni=www.google.com:repeats=11:tcp_ack=-66000:tcp_ts_up",
    "--lua-desync=multidisorder_legacy:pos=1,midsld:repeats=11",
    "--new",
    "--filter-udp=443",
    "--ipset=../lists/ipset-all.txt",
    "--ipset-exclude=../lists/ipset-exclude.txt",
    "--lua-desync=fake:blob=quic_google:repeats=11",
    "--new",
    "--filter-tcp=80,443",
    "--ipset=../lists/ipset-all.txt",
    "--ipset-exclude=../lists/ipset-exclude.txt",
    "--lua-desync=fake:blob=0x00000000:repeats=11:tcp_ack=-66000:tcp_ts_up",
    "--lua-desync=fake:blob=fake_default_tls:tls_mod=rnd,dupsid,sni=www.google.com:repeats=11:tcp_ack=-66000:tcp_ts_up",
    "--lua-desync=multidisorder_legacy:pos=1,midsld:repeats=11"
)

Write-Host "Starting winws2.exe from engine directory..." -ForegroundColor Yellow
$process = Start-Process -FilePath ".\winws2.exe" -ArgumentList $args -PassThru -NoNewWindow

Write-Host "Engine PID: $($process.Id)" -ForegroundColor Green
Start-Sleep -Seconds 5

cd ..\..

Write-Host "Testing..." -ForegroundColor Yellow

$testUrls = @("https://www.youtube.com", "https://discord.com", "https://www.google.com")
$success = 0

foreach ($url in $testUrls) {
    try {
        $start = Get-Date
        $response = Invoke-WebRequest -Uri $url -TimeoutSec 8 -UseBasicParsing
        $elapsed = [math]::Round(((Get-Date) - $start).TotalMilliseconds, 0)
        
        if ($response.StatusCode -eq 200) {
            Write-Host "[OK] $url - ${elapsed}ms" -ForegroundColor Green
            $success++
        }
    } catch {
        Write-Host "[FAIL] $url" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "Result: $success/3" -ForegroundColor $(if($success -eq 3){"Green"}else{"Red"})

Stop-Process -Id $process.Id -Force
