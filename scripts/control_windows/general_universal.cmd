@echo off
chcp 65001 >nul
title UNBOUND — Universal Strategy

net session >nul 2>&1
if %errorLevel% neq 0 (
    powershell -Command "Start-Process '%~dpnx0' -Verb RunAs"
    exit /b
)

cd /d "%~dp0"
if exist "%~dp0unbound.exe" (
    "%~dp0unbound.exe" --cli --profile universal
) else if exist "%~dp0..\unbound.exe" (
    "%~dp0..\unbound.exe" --cli --profile universal
) else (
    echo [!] Ошибка: unbound.exe не найден!
    pause
)
