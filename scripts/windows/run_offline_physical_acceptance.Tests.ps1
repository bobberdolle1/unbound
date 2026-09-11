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
        Import-HarnessFunctions @('Save-Results', 'Get-ProcessTreeIds', 'Stop-HarnessProcessTree', 'Test-AutoTuneTerminalResult')
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

    It 'fails closed on malformed, missing, duplicate, and contradictory AutoTune terminal results' {
        $valid = '{"completed":true,"cancelled":false,"lifecycle_failures":0,"profiles_total":2,"profiles_attempted":2,"profiles_completed":2,"profiles_failed_to_start":0,"winner":"Recommended","winner_score":1,"baseline":{}}'
        $cases = @(
            @{ name='valid'; text="AUTOTUNE_RESULT_JSON=$valid`n"; expected='PASS' },
            @{ name='missing'; text='ordinary output'; expected='FAIL' },
            @{ name='malformed'; text='AUTOTUNE_RESULT_JSON={'; expected='FAIL' },
            @{ name='duplicate'; text="AUTOTUNE_RESULT_JSON=$valid`nAUTOTUNE_RESULT_JSON=$valid`n"; expected='FAIL' },
            @{ name='lifecycle'; text='AUTOTUNE_RESULT_JSON={"completed":true,"cancelled":false,"lifecycle_failures":1,"profiles_total":1,"profiles_attempted":1,"profiles_completed":1,"profiles_failed_to_start":0,"winner":"Recommended","winner_score":1,"baseline":{}}'; expected='FAIL' }
        )
        foreach ($case in $cases) {
            $stdout = Join-Path $TestDrive "$($case.name).stdout.log"
            [IO.File]::WriteAllText($stdout, $case.text, [Text.Encoding]::UTF8)
            $result = Test-AutoTuneTerminalResult ([pscustomobject]@{ timedOut=$false; exitCode=0; stdout=$stdout })
            $result.status | Should Be $case.expected
        }
    }
}
