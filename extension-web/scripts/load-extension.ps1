# Quick Load Script for Development
# This script helps you quickly load the extension in Chrome

Write-Host "Unbound Web - Quick Load Guide" -ForegroundColor Green
Write-Host "==============================" -ForegroundColor Green
Write-Host ""

Write-Host "To load the extension in Chrome:" -ForegroundColor Yellow
Write-Host "1. Open Chrome and navigate to: chrome://extensions/" -ForegroundColor White
Write-Host "2. Enable 'Developer mode' (toggle in top-right)" -ForegroundColor White
Write-Host "3. Click 'Load unpacked'" -ForegroundColor White
Write-Host "4. Select the dist/chrome folder" -ForegroundColor White
Write-Host ""

Write-Host "To load the extension in Firefox:" -ForegroundColor Yellow
Write-Host "1. Open Firefox and navigate to: about:debugging#/runtime/this-firefox" -ForegroundColor White
Write-Host "2. Click 'Load Temporary Add-on'" -ForegroundColor White
Write-Host "3. Select any file from the dist/firefox folder" -ForegroundColor White
Write-Host ""

Write-Host "Build commands:" -ForegroundColor Yellow
Write-Host "  npm run build          - Build for both browsers" -ForegroundColor White
Write-Host "  npm run build:chrome   - Build for Chrome only" -ForegroundColor White
Write-Host "  npm run build:firefox  - Build for Firefox only" -ForegroundColor White
Write-Host "  npm run dev:chrome     - Watch mode for Chrome" -ForegroundColor White
Write-Host "  npm run dev:firefox    - Watch mode for Firefox" -ForegroundColor White
Write-Host ""
