@echo off
chcp 65001 >nul
title UNBOUND — AutoTune

net session >nul 2>&1
if %errorLevel% neq 0 (
    powershell -Command "Start-Process '%~dpnx0' -Verb RunAs"
    exit /b
)

cd /d "%~dp0"
set "UNBOUND_EXE="
if exist "%~dp0Unbound.exe" set "UNBOUND_EXE=%~dp0Unbound.exe"
if not defined UNBOUND_EXE if exist "%~dp0..\Unbound.exe" set "UNBOUND_EXE=%~dp0..\Unbound.exe"
if not defined UNBOUND_EXE if exist "%~dp0..\..\build\bin\Unbound.exe" set "UNBOUND_EXE=%~dp0..\..\build\bin\Unbound.exe"
if not defined UNBOUND_EXE (
    echo [!] Ошибка: unbound.exe не найден!
    exit /b 1
)
"%UNBOUND_EXE%" --autotune
exit /b %errorlevel%
