@echo off
setlocal
cd /d "%~dp0"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0final_acceptance_v0.6.9.ps1"
exit /b %errorlevel%
