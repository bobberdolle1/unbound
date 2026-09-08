@echo off
chcp 65001 >nul
title UNBOUND — Alternative 1 Strategy

net session >nul 2>&1
if %errorLevel% neq 0 (
    powershell -Command "Start-Process '%~dpnx0' -Verb RunAs"
    exit /b
)

cd /d "%~dp0"
if exist "%~dp0Unbound.exe" (
    "%~dp0Unbound.exe" --cli --profile alt1
) else if exist "%~dp0unbound.exe" (
    "%~dp0unbound.exe" --cli --profile alt1
) else if exist "%~dp0..\Unbound.exe" (
    "%~dp0..\Unbound.exe" --cli --profile alt1
) else if exist "%~dp0..\..\build\bin\Unbound.exe" (
    "%~dp0..\..\build\bin\Unbound.exe" --cli --profile alt1
) else (
    echo [!] Ошибка: Unbound.exe не найден!
    pause
)
