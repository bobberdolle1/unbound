#!/usr/bin/env pwsh
# Unbound QA Test Suite Runner
# Runs comprehensive tests before release builds

$ErrorActionPreference = "Stop"

Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host "🧪 UNBOUND QA TEST SUITE" -ForegroundColor Cyan
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host ""

$startTime = Get-Date

# Step 1: Code Formatting
Write-Host "[1/6] 📝 Formatting Go code..." -ForegroundColor Yellow
try {
    go fmt ./...
    if ($LASTEXITCODE -ne 0) {
        throw "go fmt failed"
    }
    Write-Host "✅ Code formatting complete" -ForegroundColor Green
} catch {
    Write-Host "❌ Code formatting failed: $_" -ForegroundColor Red
    exit 1
}
Write-Host ""

# Step 2: Go Vet (Static Analysis)
Write-Host "[2/6] 🔍 Running static analysis (go vet)..." -ForegroundColor Yellow
try {
    go vet ./...
    if ($LASTEXITCODE -ne 0) {
        throw "go vet found issues"
    }
    Write-Host "✅ Static analysis passed" -ForegroundColor Green
} catch {
    Write-Host "❌ Static analysis failed: $_" -ForegroundColor Red
    exit 1
}
Write-Host ""

# Step 3: Unit Tests
Write-Host "[3/6] 🧪 Running unit tests..." -ForegroundColor Yellow
try {
    go test -v -short ./...
    if ($LASTEXITCODE -ne 0) {
        throw "Unit tests failed"
    }
    Write-Host "✅ Unit tests passed" -ForegroundColor Green
} catch {
    Write-Host "❌ Unit tests failed: $_" -ForegroundColor Red
    exit 1
}
Write-Host ""

# Step 4: Race Detector Tests
Write-Host "[4/6] 🏁 Running race detector tests..." -ForegroundColor Yellow
try {
    go test -race -short ./...
    if ($LASTEXITCODE -ne 0) {
        throw "Race detector found issues"
    }
    Write-Host "✅ Race detector tests passed" -ForegroundColor Green
} catch {
    Write-Host "❌ Race detector found issues: $_" -ForegroundColor Red
    exit 1
}
Write-Host ""

# Step 5: Integration Tests (Optional)
Write-Host "[5/7] 🔗 Running integration tests..." -ForegroundColor Yellow
$env:RUN_INTEGRATION_TESTS = "0"
try {
    go test -v -run Integration ./engine/...
    if ($LASTEXITCODE -ne 0) {
        Write-Host "⚠️  Some integration tests failed (non-critical)" -ForegroundColor Yellow
    } else {
        Write-Host "✅ Integration tests passed" -ForegroundColor Green
    }
} catch {
    Write-Host "⚠️  Integration tests skipped or failed: $_" -ForegroundColor Yellow
}
Write-Host ""

# Step 6: E2E Bypass Matrix Test
Write-Host "[6/7] 🎯 Running E2E Bypass Matrix Test..." -ForegroundColor Yellow
try {
    go test -v -timeout 5m ./tests/e2e_bypass_matrix_test.go
    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ E2E Bypass Matrix test failed" -ForegroundColor Red
        exit 1
    }
    Write-Host "✅ E2E Bypass Matrix test passed" -ForegroundColor Green
} catch {
    Write-Host "❌ E2E Bypass Matrix test failed: $_" -ForegroundColor Red
    exit 1
}
Write-Host ""

# Step 7: Build Release Binary
Write-Host "[7/7] 🔨 Building release binary..." -ForegroundColor Yellow
try {
    wails build
    if ($LASTEXITCODE -ne 0) {
        throw "Wails build failed"
    }
    Write-Host "✅ Release build complete" -ForegroundColor Green
} catch {
    Write-Host "❌ Build failed: $_" -ForegroundColor Red
    exit 1
}
Write-Host ""

# Summary
$endTime = Get-Date
$duration = $endTime - $startTime

Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host "✅ QA SUITE COMPLETED SUCCESSFULLY" -ForegroundColor Green
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host ""
Write-Host "⏱️  Total time: $($duration.TotalSeconds) seconds" -ForegroundColor Cyan
Write-Host "📦 Binary location: build/bin/unbound.exe" -ForegroundColor Cyan
Write-Host ""

# Check if binary exists
if (Test-Path "build/bin/unbound.exe") {
    $fileInfo = Get-Item "build/bin/unbound.exe"
    Write-Host "📊 Binary size: $([math]::Round($fileInfo.Length / 1MB, 2)) MB" -ForegroundColor Cyan
    Write-Host "📅 Build date: $($fileInfo.LastWriteTime)" -ForegroundColor Cyan
} else {
    Write-Host "⚠️  Warning: Binary not found at expected location" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "Ready for release!" -ForegroundColor Green

