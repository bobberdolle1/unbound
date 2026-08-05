@echo off
chcp 65001 >nul
title UNBOUND — AutoTune

net session >nul 2>&1
if %errorLevel% neq 0 (
    powershell -Command "Start-Process '%~dpnx0' -Verb RunAs"
    exit /b
)

cd /d "%~dp0"
if exist "%~dp0Unbound.exe" (
    "%~dp0Unbound.exe" --autotune
) else if exist "%~dp0..\Unbound.exe" (
    "%~dp0..\Unbound.exe" --autotune
) else if exist "%~dp0..\..\build\bin\Unbound.exe" (
    "%~dp0..\..\build\bin\Unbound.exe" --autotune
) else (
    echo [!] Ошибка: unbound.exe не найден!
    pause
)
