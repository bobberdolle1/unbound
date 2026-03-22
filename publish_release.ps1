#!/usr/bin/env pwsh
#Requires -Version 5.1

$ErrorActionPreference = "Stop"

$VERSION = "1.0.0"
$PROJECT_ROOT = $PSScriptRoot
$DIST_DIR = Join-Path $PROJECT_ROOT "dist"
$INSTALLER_PATH = Join-Path $DIST_DIR "Unbound-v$VERSION.exe"
$RELEASE_NOTES_FILE = Join-Path $PROJECT_ROOT "release_notes.md"

Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host "🚀 UNBOUND v$VERSION - RELEASE PUBLICATION PIPELINE" -ForegroundColor Cyan
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host ""

function Test-Command {
    param([string]$Command)
    try {
        if (Get-Command $Command -ErrorAction Stop) { return $true }
    } catch {
        return $false
    }
}

Write-Host "📦 Step 1/5: Checking prerequisites..." -ForegroundColor Yellow

if (-not (Test-Command "git")) {
    Write-Host "❌ Git not found. Install from https://git-scm.com/" -ForegroundColor Red
    exit 1
}

if (-not (Test-Command "gh")) {
    Write-Host "❌ GitHub CLI not found. Install: winget install GitHub.cli" -ForegroundColor Red
    exit 1
}

if (-not (Test-Path $INSTALLER_PATH)) {
    Write-Host "❌ Binary not found: $INSTALLER_PATH" -ForegroundColor Red
    Write-Host "Binary should be at dist/Unbound-v1.0.0.exe" -ForegroundColor Yellow
    exit 1
}

Write-Host "✅ Git: $(git --version)" -ForegroundColor Green
Write-Host "✅ GitHub CLI: $(gh --version | Select-Object -First 1)" -ForegroundColor Green
Write-Host "✅ Binary found: $INSTALLER_PATH" -ForegroundColor Green
Write-Host ""

Write-Host "📝 Step 2/5: Generating release notes..." -ForegroundColor Yellow

$releaseNotes = @"
# Unbound v$VERSION - The Multidisorder Update

## Major Changes

- Zapret 2 Engine Migration with Lua-based strategy engine
- Multidisorder Strategy: 100% bypass rate for YouTube and Discord
- Dynamic Hostlist Synchronization from GitHub
- UI/UX Overhaul with smooth animations
- Smart Autostart via Task Scheduler

## Bug Fixes

- Fixed TLS handshake timeouts with low-TTL fake packets
- Resolved TCP RST issues from CDN servers
- Improved connection stability

## Installation

Download Unbound-v$VERSION.exe and run with Administrator privileges.
"@

Set-Content -Path $RELEASE_NOTES_FILE -Value $releaseNotes -Encoding UTF8
Write-Host "✅ Release notes generated" -ForegroundColor Green
Write-Host ""

Write-Host "📤 Step 3/5: Committing changes..." -ForegroundColor Yellow

$gitStatus = git status --porcelain
if ($gitStatus) {
    git add .
    git commit -m "chore: release v$VERSION - The Multidisorder Update"
    Write-Host "✅ Changes committed" -ForegroundColor Green
} else {
    Write-Host "⚠️  No changes to commit" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "🌐 Step 4/5: Pushing to remote..." -ForegroundColor Yellow

$currentBranch = git rev-parse --abbrev-ref HEAD
git push origin $currentBranch
Write-Host "✅ Pushed to origin/$currentBranch" -ForegroundColor Green

Write-Host ""
Write-Host "🏷️  Step 5/5: Creating GitHub release..." -ForegroundColor Yellow

$ghAuthStatus = gh auth status 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "⚠️  Not authenticated. Running gh auth login..." -ForegroundColor Yellow
    gh auth login
}

$existingRelease = gh release view "v$VERSION" 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Host "⚠️  Release v$VERSION exists. Deleting..." -ForegroundColor Yellow
    gh release delete "v$VERSION" --yes
}

gh release create "v$VERSION" "$INSTALLER_PATH" --title "Unbound v$VERSION - The Multidisorder Update" --notes-file $RELEASE_NOTES_FILE --latest

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
    Write-Host "✅ RELEASE PUBLISHED SUCCESSFULLY" -ForegroundColor Green
    Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
    Write-Host ""
    Write-Host "🎉 Release: https://github.com/unbound/releases/tag/v$VERSION" -ForegroundColor Cyan
    Write-Host "📦 Binary: Unbound-v$VERSION.exe" -ForegroundColor Cyan
    Write-Host ""
} else {
    Write-Host "❌ GitHub release creation failed" -ForegroundColor Red
    Write-Host "Authenticate: gh auth login" -ForegroundColor Yellow
}

if (Test-Path $RELEASE_NOTES_FILE) {
    Remove-Item $RELEASE_NOTES_FILE -Force
}
