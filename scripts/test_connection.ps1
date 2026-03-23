# Test connection manually
$binPath = "C:\Users\Administrator\AppData\Local\Temp\clearflow\core_bin"
$winws = Join-Path $binPath "winws2.exe"

Write-Host "Testing winws2 connection..."
Write-Host "Binary path: $winws"
Write-Host "Binary exists: $(Test-Path $winws)"

# Test simple command
$args = @(
    "--wf-l3=ipv4,ipv6",
    "--filter-tcp=443",
    "--lua-desync=fake:blob=tls1:repeats=6"
)

Write-Host "`nStarting winws2 with test args..."
Write-Host "Command: $winws $($args -join ' ')"

$proc = Start-Process -FilePath $winws -ArgumentList $args -PassThru -NoNewWindow
Start-Sleep -Seconds 2

if ($proc.HasExited) {
    Write-Host "Process exited with code: $($proc.ExitCode)"
} else {
    Write-Host "Process running with PID: $($proc.Id)"
    Stop-Process -Id $proc.Id -Force
    Write-Host "Process stopped"
}
