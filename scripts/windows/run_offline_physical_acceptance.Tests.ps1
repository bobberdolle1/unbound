#Requires -Version 5.1

$workerPath = Join-Path $PSScriptRoot 'run_offline_physical_acceptance.ps1'

function Import-HarnessFunctions([string[]]$Names) {
    $tokens = $null
    $errors = $null
    $ast = [Management.Automation.Language.Parser]::ParseFile($workerPath, [ref]$tokens, [ref]$errors)
    if ($errors.Count -ne 0) { throw $errors[0].Message }

    $definitions = $ast.FindAll({
        param($node)
        $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -in $Names
    }, $true) | ForEach-Object { $_.Extent.Text }
    Invoke-Expression (($definitions -join [Environment]::NewLine) -replace '(?m)^function ', 'function global:')
}

Describe 'offline physical acceptance harness helpers' {
    BeforeAll {
        Import-HarnessFunctions @('Save-Results', 'Get-ProcessTreeIds', 'Stop-HarnessProcessTree')
    }

    It 'persists the latest complete JSON result without leaving a temporary file' {
        $resultPath = Join-Path $TestDrive 'result.json'
        $results = [ordered]@{ status = 'RUNNING'; sequence = 1 }

        Save-Results
        $results.status = 'COMPLETE'
        $results.sequence = 2
        Save-Results

        $saved = Get-Content $resultPath -Raw | ConvertFrom-Json
        $saved.status | Should Be 'COMPLETE'
        $saved.sequence | Should Be 2
        Test-Path "$resultPath.tmp" | Should Be $false
    }

    It 'stops only the tracked process tree' {
        $process = Start-Process cmd.exe -ArgumentList '/d','/c','ping -n 30 127.0.0.1 > nul' -PassThru
        try {
            Start-Sleep -Milliseconds 300
            @(Get-ProcessTreeIds $process.Id).Count | Should BeGreaterThan 0

            Stop-HarnessProcessTree $process
            $process.WaitForExit(5000) | Out-Null
            $process.HasExited | Should Be $true
        } finally {
            if (-not $process.HasExited) { Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue }
        }
    }
}
