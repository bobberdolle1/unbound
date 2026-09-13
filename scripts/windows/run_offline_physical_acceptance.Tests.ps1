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
        Import-HarnessFunctions @('Save-Results', 'Get-ProcessTreeIds', 'Stop-HarnessProcessTree', 'Register-HarnessProcess', 'Invoke-Captured', 'Test-AutoTuneTerminalResult', 'Get-CapturedStageResult')
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
            @{ name='lifecycle'; text='AUTOTUNE_RESULT_JSON={"completed":true,"cancelled":false,"lifecycle_failures":1,"profiles_total":1,"profiles_attempted":1,"profiles_completed":1,"profiles_failed_to_start":0,"winner":"Recommended","winner_score":1,"baseline":{}}'; expected='FAIL' },
            @{ name='incomplete'; text='AUTOTUNE_RESULT_JSON={"completed":false,"cancelled":false,"lifecycle_failures":0,"profiles_total":1,"profiles_attempted":1,"profiles_completed":1,"profiles_failed_to_start":0,"winner":"Recommended","winner_score":1,"baseline":{}}'; expected='FAIL' }
        )
        foreach ($case in $cases) {
            $stdout = Join-Path $TestDrive "$($case.name).stdout.log"
            [IO.File]::WriteAllText($stdout, $case.text, [Text.Encoding]::UTF8)
            $result = Test-AutoTuneTerminalResult ([pscustomobject]@{ timedOut=$false; exitCode=0; stdout=$stdout })
            $result.status | Should Be $case.expected
        }
    }

    It 'captures a completed process exit code without a stale process state' {
        $bundle = $TestDrive
        $trackedProcesses = @()

        $capture = Invoke-Captured 'exit-zero' $env:ComSpec @('/d','/c','exit 0') 5

        $capture.timedOut | Should Be $false
        $capture.exitCode | Should Be 0
    }

    It 'fails kernel and doctor stages on process failure or timeout' {
        (Get-CapturedStageResult 'KERNEL' ([pscustomobject]@{ timedOut=$false; exitCode=0 })).status | Should Be 'PASS'
        (Get-CapturedStageResult 'DOCTOR' ([pscustomobject]@{ timedOut=$false; exitCode=1 })).status | Should Be 'FAIL'
        (Get-CapturedStageResult 'DOCTOR' ([pscustomobject]@{ timedOut=$true; exitCode=$null })).error | Should Be 'PROCESS_TIMEOUT'
    }

    It 'fails closed unless every required acceptance stage passes exactly once' {
        Import-HarnessFunctions @('Get-AcceptanceVerdict')
        $required = @('CLEAN_WINDOW','KERNEL','RECOMMENDED_FIELD','DOCTOR','CONCURRENT_START','AUTOTUNE','LAUNCHERS','CLEANUP')
        $passing = @($required | ForEach-Object { [pscustomobject]@{ name=$_; status='PASS' } })
        (Get-AcceptanceVerdict $passing) | Should Be 'PASS'
        (Get-AcceptanceVerdict @($passing + [pscustomobject]@{ name='DOCTOR'; status='FAIL' })) | Should Be 'FAIL'
        (Get-AcceptanceVerdict @($passing | Where-Object name -ne 'CLEANUP')) | Should Be 'FAIL'
        (Get-AcceptanceVerdict @($passing + [pscustomobject]@{ name='AUTOTUNE'; status='PASS' })) | Should Be 'FAIL'
        $doctorFailure = @($passing | ForEach-Object { [pscustomobject]@{ name=$_.name; status=$_.status } })
        ($doctorFailure | Where-Object name -eq 'DOCTOR').status = 'FAIL'
        (Get-AcceptanceVerdict $doctorFailure) | Should Be 'FAIL'
    }
}
