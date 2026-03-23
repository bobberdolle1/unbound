# Test script to verify console window doesn't flash
Write-Host "Testing console window behavior..." -ForegroundColor Cyan

# Start the app
Write-Host "Starting Unbound..." -ForegroundColor Yellow
Start-Process -FilePath ".\build\bin\unbound.exe" -Verb RunAs

Write-Host ""
Write-Host "Application started. Please test:" -ForegroundColor Green
Write-Host "1. Check if admin warning appears when running without admin rights" -ForegroundColor White
Write-Host "2. Start a profile and check if console window flashes" -ForegroundColor White
Write-Host "3. Switch between profiles and verify no console flashing" -ForegroundColor White
Write-Host "4. Check if conflict warning appears when winws2.exe is already running" -ForegroundColor White
Write-Host ""
Write-Host "Press any key to kill all winws2.exe processes and exit..." -ForegroundColor Yellow
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")

# Cleanup
Write-Host "Cleaning up..." -ForegroundColor Cyan
taskkill /F /IM winws2.exe 2>$null
taskkill /F /IM unbound.exe 2>$null

Write-Host "Done!" -ForegroundColor Green
