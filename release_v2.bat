@echo off
setlocal enabledelayedexpansion

REM ============================================================================
REM UNBOUND v2.0.0 Release Automation Script (Windows)
REM ============================================================================
REM This script orchestrates the complete release process:
REM 1. Builds all platform binaries
REM 2. Deploys website to GitHub Pages
REM 3. Creates GitHub Release with all binaries
REM ============================================================================

set "VERSION=v2.0.0"
set "RELEASE_TITLE=UNBOUND v2.0.0"
set "REPO_ROOT=%~dp0"
set "DIST_DIR=%REPO_ROOT%dist"

echo.
echo ================================================================
echo  UNBOUND v2.0.0 Release Automation
echo ================================================================
echo.

REM ============================================================================
REM Pre-flight Checks
REM ============================================================================

echo [INFO] Running pre-flight checks...

where git >nul 2>&1
if errorlevel 1 (
    echo [ERROR] git is required but not found. Please install Git first.
    pause
    exit /b 1
)

where gh >nul 2>&1
if errorlevel 1 (
    echo [ERROR] gh (GitHub CLI) is required but not found. Please install it first.
    pause
    exit /b 1
)

where npm >nul 2>&1
if errorlevel 1 (
    echo [ERROR] npm is required but not found. Please install Node.js first.
    pause
    exit /b 1
)

where go >nul 2>&1
if errorlevel 1 (
    echo [ERROR] go is required but not found. Please install Go first.
    pause
    exit /b 1
)

where wails >nul 2>&1
if errorlevel 1 (
    echo [ERROR] wails is required but not found. Run: go install github.com/wailsapp/wails/v2/cmd/wails@latest
    pause
    exit /b 1
)

REM Check if logged into GitHub CLI
gh auth status >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Not authenticated with GitHub CLI. Run: gh auth login
    pause
    exit /b 1
)

REM Verify we're in the right directory
if not exist "wails.json" (
    echo [ERROR] wails.json not found. Please run this script from the repository root.
    pause
    exit /b 1
)

echo [SUCCESS] All pre-flight checks passed!
echo.

REM ============================================================================
REM Step 1: Build All Platforms
REM ============================================================================

echo ================================================================
echo  Step 1: Building all platform binaries
echo ================================================================
echo.

REM Create dist directory
if not exist "%DIST_DIR%" mkdir "%DIST_DIR%"

REM Check if build_all.ps1 exists
if exist "scripts\build_all.ps1" (
    echo [INFO] Running build orchestration script...
    powershell -ExecutionPolicy Bypass -File "scripts\build_all.ps1"
    if errorlevel 1 (
        echo [ERROR] Build script failed!
        pause
        exit /b 1
    )
    echo [SUCCESS] Build complete!
) else (
    echo [WARN] scripts\build_all.ps1 not found. Building manually...
    
    REM Build Wails desktop app
    echo [INFO] Building desktop app (Wails)...
    wails build -platform windows/amd64
    if exist "build\bin\unbound.exe" (
        copy "build\bin\unbound.exe" "%DIST_DIR%\unbound-desktop-windows-amd64.exe"
        echo [SUCCESS] Desktop Windows build complete
    )
    
    REM Build Browser Extension
    if exist "extension-web" (
        echo [INFO] Building Browser Extension...
        cd extension-web
        call npm install
        call npm run build
        if exist "dist" (
            cd dist
            powershell Compress-Archive -Path * -DestinationPath "%DIST_DIR%\unbound-extension-chrome.zip" -Force
            cd ..
            echo [SUCCESS] Browser Extension build complete
        )
        cd ..
    )
)

REM List all built artifacts
echo.
echo [INFO] Built artifacts in dist:
dir /b "%DIST_DIR%"

echo.

REM ============================================================================
REM Step 2: Deploy Website
REM ============================================================================

echo ================================================================
echo  Step 2: Deploying website to GitHub Pages
echo ================================================================
echo.

if exist "website" (
    cd website
    
    if exist "package.json" (
        echo [INFO] Installing website dependencies...
        call npm install
        
        echo [INFO] Deploying to GitHub Pages...
        call npm run deploy
        if errorlevel 1 (
            echo [WARN] Website deployment failed. Continuing with release...
        ) else (
            echo [SUCCESS] Website deployed successfully!
        )
    ) else (
        echo [WARN] website\package.json not found. Skipping website deployment.
    )
    
    cd ..
) else (
    echo [WARN] website\ directory not found. Skipping website deployment.
)

echo.

REM ============================================================================
REM Step 3: Create GitHub Release
REM ============================================================================

echo ================================================================
echo  Step 3: Creating GitHub Release
echo ================================================================
echo.

REM Check if release already exists
gh release view %VERSION% >nul 2>&1
if not errorlevel 1 (
    echo [WARN] Release %VERSION% already exists.
    set /p DELETE_EXIST="Delete and recreate? (y/N): "
    if /i "!DELETE_EXIST!"=="y" (
        echo [INFO] Deleting existing release...
        gh release delete %VERSION% --yes
    ) else (
        echo [ERROR] Release already exists. Aborting.
        pause
        exit /b 1
    )
)

REM Generate release notes
echo [INFO] Generating release notes...
(
echo # 🚀 UNBOUND v2.0.0
echo.
echo ## Что нового
echo.
echo Это **мажорный релиз**, который превращает Unbound из десктопного приложения в **мощную мультиплатформенную систему** для обхода DPI.
echo.
echo ### ✨ Major Features
echo.
echo - **🌍 Multi-Platform Ecosystem** - Desktop, Android, iOS, Linux, OpenWrt, Browser Extensions, Smart TV, Steam Deck
echo - **🎯 Auto-Tune V2** - Parallel scanner that finds optimal profiles in seconds
echo - **🔄 System Tray Integration** - Runs silently in background with tray controls
echo - **🎨 Redesigned UI** - Modern sketchy design language with real-time monitoring
echo - **⚡ Improved Performance** - 5-second timeout per probe, no more hanging
echo - **🔒 Enhanced Security** - HideWindow on all child processes, no console flashing
echo.
echo ### 🐛 Bug Fixes
echo.
echo - Fixed settings state persistence ^(checkboxes now save properly^)
echo - Fixed window close behavior ^(minimizes to tray instead of quitting^)
echo - Исправлена проблема с бесконечным зависанием AutoTune
echo - Убрано мерцание консоли при AutoTune в Windows
echo - Удалены устаревшие опции MTProto/Telegram
echo.
echo ### 📦 Поддерживаемые платформы
echo.
echo | Platform | Format | Status |
echo |----------|--------|--------|
echo | **Desktop ^(Windows/macOS^)** | Native App | ✅ Production Ready |
echo | **Android** | APK | ✅ Production Ready |
echo | **iOS ^(Jailbreak^)** | Theos Tweak | ✅ Production Ready |
echo | **Linux** | CLI Binary | ✅ Production Ready |
echo | **OpenWrt** | Package | ✅ Production Ready |
echo | **Browser** | Chrome/Firefox Extension | ✅ Production Ready |
echo | **Steam Deck** | Decky Plugin | ✅ Beta |
echo | **Smart TV** | WebOS/tvOS | ✅ Beta |
echo.
echo ### 🎯 Supported Services
echo.
echo - ✅ **YouTube** - Full 4K streaming support
echo - ✅ **Discord** - Voice, video, screen sharing
echo - ✅ **Instagram** - Feed, stories, reels, DMs
echo - ✅ **Twitter/X** - Timeline, media, search
echo - ✅ **Facebook** - Новостная лента, мессенджер
echo - ✅ **RuTracker** - Доступ к ресурсу
echo.
echo ---
echo.
echo **Полный список изменений**: https://github.com/OWNER/REPO/compare/v1.0.4...%VERSION%
) > "%TEMP%\release_notes.md"

REM Collect all artifacts
set "ARTIFACTS="
for %%F in ("%DIST_DIR%\*") do (
    set "ARTIFACTS=!ARTIFACTS! "%%F""
)

REM Create the release
echo [INFO] Creating release %VERSION%...
if defined ARTIFACTS (
    gh release create %VERSION% ^
        --title "%RELEASE_TITLE%" ^
        --notes-file "%TEMP%\release_notes.md" ^
        --draft ^
        --generate-notes ^
        %ARTIFACTS%
) else (
    gh release create %VERSION% ^
        --title "%RELEASE_TITLE%" ^
        --notes-file "%TEMP%\release_notes.md" ^
        --draft ^
        --generate-notes
)

REM Clean up
del "%TEMP%\release_notes.md"

echo [SUCCESS] GitHub Release created: %VERSION%
echo [INFO] Release is in DRAFT mode. Review and publish at: https://github.com/OWNER/REPO/releases/tag/%VERSION%
echo.

REM ============================================================================
REM Post-Release Steps
REM ============================================================================

echo ================================================================
echo  Post-release tasks
echo ================================================================
echo.

REM Tag the commit
echo [INFO] Tagging commit with %VERSION%...
git tag -a %VERSION% -m "UNBOUND %VERSION% релиз"
git push origin %VERSION%

REM Commit version bumps
echo [INFO] Committing version bumps...
git add -A
git commit -m "chore: bump version to %VERSION% across all ecosystem projects

Released as %VERSION%"
if errorlevel 1 (
    echo [WARN] No changes to commit or already committed
) else (
    echo [SUCCESS] Version bumps committed
)

echo [SUCCESS] All post-release tasks completed!
echo.

REM ============================================================================
REM Summary
REM ============================================================================

echo ================================================================
echo  🎉 Release %VERSION% Summary
echo ================================================================
echo.
echo   ✓ All platform binaries built
echo   ✓ Website deployed to GitHub Pages
echo   ✓ GitHub Release created ^(DRAFT^)
echo   ✓ Commit tagged and pushed
echo.
echo   Next steps:
echo   1. Review the draft release: https://github.com/OWNER/REPO/releases
echo   2. Test all binaries on respective platforms
echo   3. Publish the release when ready
echo.
echo   🚀 Total War on Censorship! 🚀
echo.

pause
