#Requires -Version 5.1

$workerPath = Join-Path $PSScriptRoot 'run_privileged_preflight.ps1'

function Import-PreflightFunction([string]$Name) {
    $tokens = $null
    $errors = $null
    $ast = [Management.Automation.Language.Parser]::ParseFile($workerPath, [ref]$tokens, [ref]$errors)
    if ($errors.Count -ne 0) { throw $errors[0].Message }

    $definition = $ast.FindAll({
        param($node)
        $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $Name
    }, $true) | Select-Object -First 1
    if ($null -eq $definition) { throw "Function not found: $Name" }
    Invoke-Expression ($definition.Extent.Text -replace '(?m)^function ', 'function global:')
}

Describe 'privileged preflight AutoTune result contract' {
    BeforeAll {
        Import-PreflightFunction 'Test-AutoTuneTerminalResult'
    }

    It 'accepts only the expected no-winner outcome while Happ provides connectivity' {
        $stdout = Join-Path $TestDrive 'no-winner.stdout.log'
        [IO.File]::WriteAllText($stdout, 'AUTOTUNE_RESULT_JSON={"cancelled":false,"completed":false,"error":"no profile improved connectivity without regressions","error_category":"AUTOTUNE_INTERNAL_FAILURE","lifecycle_failures":0,"profiles_attempted":0,"profiles_completed":0,"profiles_failed_to_start":0,"profiles_total":13}', [Text.Encoding]::UTF8)
        $capture = [pscustomobject]@{ timedOut=$false; exitCode=1; stdout=$stdout }

        (Test-AutoTuneTerminalResult $capture -AllowNoWinner) | Should Be $null
        (Test-AutoTuneTerminalResult $capture) | Should Be 'PROCESS_EXIT_1'
    }

    It 'rejects no-winner outcomes with lifecycle failures' {
        $stdout = Join-Path $TestDrive 'lifecycle.stdout.log'
        [IO.File]::WriteAllText($stdout, 'AUTOTUNE_RESULT_JSON={"cancelled":false,"completed":false,"error":"no profile improved connectivity without regressions","error_category":"AUTOTUNE_INTERNAL_FAILURE","lifecycle_failures":1,"profiles_attempted":0,"profiles_completed":0,"profiles_failed_to_start":0,"profiles_total":13}', [Text.Encoding]::UTF8)

        (Test-AutoTuneTerminalResult ([pscustomobject]@{ timedOut=$false; exitCode=1; stdout=$stdout }) -AllowNoWinner) | Should Be 'PROCESS_EXIT_1'
    }
}
