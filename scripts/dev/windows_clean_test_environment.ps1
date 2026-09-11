#Requires -Version 5.1
[CmdletBinding()]
param(
    [ValidateSet('Snapshot', 'Enter', 'Exit')]
    [string]$Mode = 'Snapshot',
    [string]$StatePath = (Join-Path $env:TEMP 'unbound-clean-test-state.json')
)

$ErrorActionPreference = 'Stop'

function Get-CleanTestState {
    $proxy = Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings'
    $tailscale = Get-Command tailscale.exe -ErrorAction SilentlyContinue
    [pscustomobject]@{
        timestamp = (Get-Date).ToString('o')
        happService = @(Get-Service HappService -ErrorAction SilentlyContinue | Select-Object Name, Status, StartType)
        processes = @(Get-Process Happ, xray, sing-box, winws2, Unbound -ErrorAction SilentlyContinue | Select-Object ProcessName, Id)
        systemProxy = [pscustomobject]@{
            enabled = [bool]$proxy.ProxyEnable
            server = $proxy.ProxyServer
            autoConfigUrl = $proxy.AutoConfigURL
        }
        dns = @(Get-DnsClientServerAddress | Select-Object InterfaceAlias, AddressFamily, ServerAddresses)
        defaultRoutes = @(Get-NetRoute | Where-Object { $_.DestinationPrefix -in @('0.0.0.0/0', '::/0') } | Select-Object DestinationPrefix, InterfaceAlias, NextHop, RouteMetric, ifIndex)
        tailscale = [pscustomobject]@{
            installed = $null -ne $tailscale
            status = if ($tailscale) { (& $tailscale.Source status --json 2>$null | Out-String).Trim() } else { '' }
            prefs = if ($tailscale) { (& $tailscale.Source debug prefs 2>$null | Out-String).Trim() } else { '' }
        }
    }
}

switch ($Mode) {
    'Snapshot' {
        Get-CleanTestState | ConvertTo-Json -Depth 8 | Set-Content $StatePath -Encoding utf8
        Write-Output $StatePath
    }
    'Enter' {
        $state = Get-CleanTestState
        $state | ConvertTo-Json -Depth 8 | Set-Content $StatePath -Encoding utf8
        $active = @($state.processes | Where-Object { $_.ProcessName -in @('Happ', 'xray', 'sing-box', 'winws2', 'Unbound') })
        if ($active.Count -ne 0) {
            throw "Clean mode requires manual shutdown of active processes: $($active.ProcessName -join ', '). This helper never stops services, VPNs, proxies, or processes."
        }
        if ($state.systemProxy.enabled) {
            throw 'Clean mode requires a manually disabled system proxy. This helper never changes proxy settings.'
        }
        Write-Output $StatePath
    }
    'Exit' {
        if (-not (Test-Path $StatePath)) { throw "No test state found: $StatePath" }
        Get-Content $StatePath -Raw
    }
}
