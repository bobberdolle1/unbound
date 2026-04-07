# Unregister the Unbound Desktop host from Windows Registry

$chromeKey = "HKCU:\SOFTWARE\Google\Chrome\NativeMessagingHosts\com.unbound.desktop"

if (Test-Path $chromeKey) {
    Remove-Item -Path $chromeKey -Force
    Write-Host "Native messaging host unregistered successfully"
} else {
    Write-Host "Host was not registered"
}
