# Windows Registry Setup for Native Messaging
# Run this script to register the Unbound Desktop host

$hostManifestPath = "F:\Projects\Unbound\extension-web\host_manifest.json"
$chromeKey = "HKCU:\SOFTWARE\Google\Chrome\NativeMessagingHosts\com.unbound.desktop"

# Create registry key
if (!(Test-Path $chromeKey)) {
    New-Item -Path $chromeKey -Force
}

# Set default value to manifest path
Set-ItemProperty -Path $chromeKey -Name "(default)" -Value $hostManifestPath

Write-Host "Native messaging host registered successfully"
Write-Host "Manifest path: $hostManifestPath"
Write-Host ""
Write-Host "IMPORTANT: Update host_manifest.json with:"
Write-Host "  1. Correct path to your host binary"
Write-Host "  2. Actual extension ID(s)"
