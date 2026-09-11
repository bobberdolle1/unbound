#Requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string]$StatusPath,
    [Parameter(Mandatory)] [string]$EventLogPath,
    [ValidateRange(100, 5000)] [int]$PollMilliseconds = 250,
    [ValidateRange(1, 3600)] [int]$TerminalDisplaySeconds = 300
)

$ErrorActionPreference = 'Stop'

function Write-StatusWindowEvent([string]$Message) {
    Add-Content -Path $EventLogPath -Value "$(Get-Date -Format o) $Message" -Encoding utf8
}

try {
    Add-Type -AssemblyName System.Windows.Forms
    Add-Type -AssemblyName System.Drawing

    $form = New-Object Windows.Forms.Form
    $form.Text = 'UNBOUND Offline Acceptance'
    $form.Size = New-Object Drawing.Size(680, 260)
    $form.StartPosition = [Windows.Forms.FormStartPosition]::CenterScreen
    $form.TopMost = $true
    $form.ShowInTaskbar = $true
    $form.FormBorderStyle = [Windows.Forms.FormBorderStyle]::FixedDialog
    $form.MaximizeBox = $false
    $form.MinimizeBox = $true

    $headline = New-Object Windows.Forms.Label
    $headline.Location = New-Object Drawing.Point(24, 28)
    $headline.Size = New-Object Drawing.Size(620, 44)
    $headline.Font = New-Object Drawing.Font('Segoe UI', 18, [Drawing.FontStyle]::Bold)
    $headline.Text = 'ACCEPTANCE WORKER STARTED'

    $detail = New-Object Windows.Forms.Label
    $detail.Location = New-Object Drawing.Point(26, 88)
    $detail.Size = New-Object Drawing.Size(610, 82)
    $detail.Font = New-Object Drawing.Font('Segoe UI', 11)
    $detail.Text = 'Waiting for local acceptance status…'

    $footer = New-Object Windows.Forms.Label
    $footer.Location = New-Object Drawing.Point(26, 190)
    $footer.Size = New-Object Drawing.Size(610, 28)
    $footer.Font = New-Object Drawing.Font('Segoe UI', 9)
    $footer.Text = 'This status window is local and does not depend on OMP or cloud services.'

    [void]$form.Controls.AddRange(@($headline, $detail, $footer))
    $terminalAt = $null
    $lastError = $null
    $timer = New-Object Windows.Forms.Timer
    $timer.Interval = $PollMilliseconds
    $timer.Add_Tick({
        try {
            if (-not (Test-Path $StatusPath -PathType Leaf)) { return }
            $status = Get-Content -Path $StatusPath -Raw -Encoding utf8 | ConvertFrom-Json
            $headline.Text = $status.headline
            $detail.Text = $status.detail
            $footer.Text = "Updated $($status.updatedAt)"
            if ($status.headline -match 'FAILED') {
                $headline.ForeColor = [Drawing.Color]::Firebrick
            } elseif ($status.terminal) {
                $headline.ForeColor = [Drawing.Color]::ForestGreen
            } else {
                $headline.ForeColor = [Drawing.Color]::DarkBlue
            }
            if ($status.terminal -and $null -eq $terminalAt) {
                $script:terminalAt = Get-Date
                Write-StatusWindowEvent "STATUS_WINDOW_TERMINAL: $($status.headline)"
            }
            if ($null -ne $terminalAt -and ((Get-Date) - $terminalAt).TotalSeconds -ge $TerminalDisplaySeconds) {
                $form.Close()
            }
        } catch {
            if ($lastError -ne $_.Exception.Message) {
                $script:lastError = $_.Exception.Message
                Write-StatusWindowEvent "STATUS_WINDOW_UPDATE_FAILED: $($_.Exception.Message)"
            }
        }
    })
    $form.Add_Shown({ Write-StatusWindowEvent 'STATUS_WINDOW_VISIBLE' })
    $timer.Start()
    [void]$form.ShowDialog()
    $timer.Stop()
    Write-StatusWindowEvent 'STATUS_WINDOW_CLOSED'
} catch {
    try {
        Write-StatusWindowEvent "STATUS_WINDOW_FAILED: $($_.Exception.Message)"
    } catch {
        Write-Error "STATUS_WINDOW_FAILED: $($_.Exception.Message)"
    }
    exit 1
}
