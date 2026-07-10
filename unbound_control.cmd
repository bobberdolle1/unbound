@echo off
chcp 65001 >nul
title UNBOUND Control Center v2.5.0

:: Check Admin Privileges
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo ===================================================
    echo  [!] ТРЕБУЮТСЯ ПРАВА АДМИНИСТРАТОРА
    echo ===================================================
    echo Этот скрипт должен быть запущен от имени Администратора.
    echo Перезапускаю с повышением прав...
    powershell -Command "Start-Process '%~dpnx0' -Verb RunAs"
    exit /b
)

:menu
cls
echo ===================================================
echo   🚀 UNBOUND CONTROL CENTER v2.5.0
echo ===================================================
echo.
echo [1] Запустить в графическом режиме (GUI)
echo [2] Запустить в фоновом режиме (CLI) - Universal 2026
echo [3] Запустить в фоновом режиме (CLI) - Recommended
echo [4] Запустить в фоновом режиме (CLI) - Alternative 1
echo [5] Запустить в фоновом режиме (CLI) - Advanced 1
echo.
echo [6] Установить в автозагрузку (Запуск в трее при старте Windows)
echo [7] Удалить из автозагрузки
echo.
echo [8] Редактировать список YouTube (youtube.txt)
echo [9] Редактировать список Discord (discord.txt)
echo [10] Редактировать список исключений (ipset-exclude.txt)
echo.
echo [11] Остановить все службы обхода и очистить WinDivert
echo [12] Выход
echo.
echo ===================================================
set /p choice="Выберите пункт меню (1-12): "

if "%choice%"=="1" goto run_gui
if "%choice%"=="2" goto run_cli_universal
if "%choice%"=="3" goto run_cli_recommended
if "%choice%"=="4" goto run_cli_alternative
if "%choice%"=="5" goto run_cli_advanced
if "%choice%"=="6" goto install_autostart
if "%choice%"=="7" goto remove_autostart
if "%choice%"=="8" goto edit_youtube
if "%choice%"=="9" goto edit_discord
if "%choice%"=="10" goto edit_exclude
if "%choice%"=="11" goto stop_all
if "%choice%"=="12" exit /b
goto menu

:run_gui
echo Запуск GUI...
start "" "%~dp0unbound.exe"
goto menu

:run_cli_universal
echo Запуск CLI: Universal 2026 (All-in-One)...
"%~dp0unbound.exe" --cli --profile "Universal 2026 (All-in-One)"
pause
goto menu

:run_cli_recommended
echo Запуск CLI: Recommended (hostfakesplit)...
"%~dp0unbound.exe" --cli --profile "Recommended (hostfakesplit)"
pause
goto menu

:run_cli_alternative
echo Запуск CLI: Alternative 1 (multisplit)...
"%~dp0unbound.exe" --cli --profile "Alternative 1 (multisplit)"
pause
goto menu

:run_cli_advanced
echo Запуск CLI: Advanced 1 (hostfakesplit + MD5)...
"%~dp0unbound.exe" --cli --profile "Advanced 1 (hostfakesplit + MD5)"
pause
goto menu

:install_autostart
echo Установка в автозагрузку...
schtasks /create /tn "UnboundDPIBypass" /tr "\"%~dp0unbound.exe\" -tray" /sc onlogon /rl highest /f
echo Готово! Задача автозапуска создана.
pause
goto menu

:remove_autostart
echo Удаление автозагрузки...
schtasks /delete /tn "UnboundDPIBypass" /f
echo Готово! Задача автозапуска удалена.
pause
goto menu

:edit_youtube
if not exist "%APPDATA%\Unbound\lists\youtube.txt" (
    echo Создание списка youtube.txt...
    powershell -Command "New-Item -Path '$env:APPDATA\Unbound\lists\youtube.txt' -ItemType File -Force" >nul
)
notepad "%APPDATA%\Unbound\lists\youtube.txt"
goto menu

:edit_discord
if not exist "%APPDATA%\Unbound\lists\discord.txt" (
    echo Создание списка discord.txt...
    powershell -Command "New-Item -Path '$env:APPDATA\Unbound\lists\discord.txt' -ItemType File -Force" >nul
)
notepad "%APPDATA%\Unbound\lists\discord.txt"
goto menu

:edit_exclude
if not exist "%APPDATA%\Unbound\lists\ipset-exclude.txt" (
    echo Создание списка ipset-exclude.txt...
    powershell -Command "New-Item -Path '$env:APPDATA\Unbound\lists\ipset-exclude.txt' -ItemType File -Force" >nul
)
notepad "%APPDATA%\Unbound\lists\ipset-exclude.txt"
goto menu

:stop_all
echo Останавливаю все процессы обхода...
taskkill /f /im winws2.exe >nul 2>&1
taskkill /f /im winws.exe >nul 2>&1
taskkill /f /im unbound.exe >nul 2>&1
sc stop WinDivert >nul 2>&1
echo Все процессы остановлены и драйверы сброшены.
pause
goto menu
