#Requires -Version 5.1
$scriptPath = Join-Path $PSScriptRoot 'run_published_release_acceptance.ps1'
function Import-CoordinatorFunctions([string[]]$Names) {
    $tokens = $null; $errors = $null
    $ast = [Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$errors)
    if ($errors.Count) { throw $errors[0].Message }
    $ast.FindAll({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -in $Names }, $true) | ForEach-Object { Invoke-Expression (($_.Extent.Text) -replace '(?m)^function ', 'function global:') }
}
Describe 'published release acceptance coordinator' {
    BeforeAll { Import-CoordinatorFunctions @('Save-Result','Stop-Tree','Invoke-BoundedCommand') }
    It 'parses without errors' {
        $tokens = $null; $errors = $null; [void][Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$errors); $errors.Count | Should Be 0
    }
    It 'retains stdout, stderr, exit code, and result.json for a completed command' {
        $bundle=$TestDrive; $commandDirectory=Join-Path $bundle 'commands'; New-Item -ItemType Directory $commandDirectory|Out-Null; $resultPath=Join-Path $bundle 'result.json'; $results=[ordered]@{commands=@()}
        $record=Invoke-BoundedCommand 'exit-zero' $env:ComSpec @('/d','/c','echo output & echo error 1>&2 & exit 0') 5
        $record.timed_out | Should Be $false; $record.exit_code | Should Be 0; Test-Path $record.stdout | Should Be $true; Test-Path $record.stderr | Should Be $true; Test-Path $resultPath | Should Be $true
    }
    It 'times out and terminates a command tree while retaining logs' {
        $bundle=$TestDrive; $commandDirectory=Join-Path $bundle 'commands-timeout'; New-Item -ItemType Directory $commandDirectory|Out-Null; $resultPath=Join-Path $bundle 'timeout-result.json'; $results=[ordered]@{commands=@()}
        $record=Invoke-BoundedCommand 'hang' $env:ComSpec @('/d','/c','ping -n 30 127.0.0.1 > nul') 1
        $record.timed_out | Should Be $true; $record.cleanup_succeeded | Should Be $true; Test-Path $record.stdout | Should Be $true; Test-Path $record.stderr | Should Be $true
    }
    It 'routes every NetTCPIP diagnostic through the bounded wrapper' {
        $text=Get-Content $scriptPath -Raw; foreach($diagnostic in 'Get-NetAdapter','Get-NetIPConfiguration','Get-NetRoute','Get-DnsClientServerAddress'){$text|Should Match ([regex]::Escape($diagnostic))}; ([regex]::Matches($text,'Invoke-Diagnostic').Count)|Should BeGreaterThan 4
    }
}
