[CmdletBinding()]
param(
    [ValidateSet("all", "frontend", "go")]
    [string]$Target = "all"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
Set-Location $ProjectRoot

function Invoke-Required {
    param([string]$Name, [scriptblock]$Command)
    & $Command
    if ($LASTEXITCODE -ne 0) {
        throw "$Name failed with exit code $LASTEXITCODE"
    }
}

function Test-CheckoutIdentity {
    $actual = (& git rev-parse HEAD).Trim()
    $expected = if ($env:EXPECTED_SHA) { $env:EXPECTED_SHA } elseif ($env:BUILDKITE_COMMIT) { $env:BUILDKITE_COMMIT } elseif ($env:GITHUB_SHA) { $env:GITHUB_SHA } else { $actual }
    Write-Output "EXPECTED_SHA=$expected"
    Write-Output "ACTUAL_SHA=$actual"
    if ($expected -ne $actual) {
        throw "checkout does not match the requested commit"
    }
}

function Test-GoToolchain {
    $directive = ((Get-Content go.mod | Where-Object { $_ -match '^go ' }) -replace '^go ', '').Trim()
    $minor = ($directive -split '\.')[0..1] -join '.'
    $actual = (& go env GOVERSION).Trim()
    Write-Output "GO_REQUIRED=$directive"
    Write-Output "GO_ACTUAL=$actual"
    if (-not $actual.StartsWith("go$minor.")) {
        throw "Go $directive-compatible toolchain is required"
    }
}

function Test-NodeToolchain {
    $required = if ($env:NODE_VERSION) { $env:NODE_VERSION } else { '22.13.0' }
    $actual = ((& node --version).Trim()).TrimStart('v')
    $requiredMajor = [int](($required -split '\.')[0])
    $actualMajor = [int](($actual -split '\.')[0])
    Write-Output "NODE_REQUIRED=$required"
    Write-Output "NODE_ACTUAL=$actual"
    if ($actualMajor -lt $requiredMajor) {
        throw "Node $required or newer is required"
    }
}

function Invoke-FrontendChecks {
    Test-NodeToolchain
    Push-Location frontend
    try {
        Invoke-Required 'npm ci' { npm ci }
        Invoke-Required 'frontend typecheck' { npm run typecheck }
        Invoke-Required 'frontend tests' { npm test }
        Invoke-Required 'frontend build' { npm run build }
    } finally {
        Pop-Location
    }
}

function Invoke-GoChecks {
    Test-GoToolchain
    $goFiles = @(git ls-files -- '*.go')
    $unformatted = @(& gofmt -l $goFiles)
    if ($unformatted.Count -gt 0) {
        throw "gofmt required for: $($unformatted -join ', ')"
    }
    Invoke-Required 'go vet' { go vet ./... }
    Invoke-Required 'go test' { go test ./... -count=1 }
    if ((& go env CGO_ENABLED).Trim() -eq '1') {
        Invoke-Required 'go test -race' { go test -race ./... -count=1 }
    } else {
        Write-Output 'RACE_TEST=SKIPPED_CGO_DISABLED'
    }
    Invoke-Required 'Windows Go build' { go build ./... }
    Invoke-Required 'winws2 dry-run parser suite' { go test ./engine -run '^TestRealWinws2DryRunVerifiesArgvAndLuaBootstrap$' -count=1 -v }
}

Test-CheckoutIdentity
switch ($Target) {
    'frontend' { Invoke-FrontendChecks }
    'go' { Invoke-GoChecks }
    'all' {
        Invoke-FrontendChecks
        Invoke-GoChecks
        if (Get-Command bash -ErrorAction SilentlyContinue) {
            Invoke-Required 'engine assets' { bash ./scripts/engine-assets.sh verify }
        } else {
            Invoke-Required 'engine asset verification' { go test ./tests -run '^TestEngine_AssetVerification$' -count=1 }
        }
    }
}
