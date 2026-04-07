# Unbound Legacy -- PowerShell Build Script (Windows + WSL)
param(
    [ValidateSet("armv7","arm64","both","clean","deploy")]
    [string]$Action = "both",
    [string]$DeviceIP = "",
    [switch]$Clean,
    [switch]$UseWSL = $true,
    [string]$WSLDistro = "Ubuntu"
)

$ProjectRoot = Split-Path -Parent $PSScriptRoot

function Invoke-WSLBuild {
    param([string]$Arch)
    $WslPath = wsl -d $WSLDistro wslpath -u "`"$ProjectRoot`"" 2>$null
    if(!$WslPath){Write-Host "WSL error" -ForegroundColor Red; exit 1}
    if($Clean){wsl -d $WSLDistro bash -c "cd '$WslPath' && chmod +x scripts/build.sh && ./scripts/build.sh clean"; return}
    $a = if($Arch -ne "both"){$Arch}else{"both"}
    wsl -d $WSLDistro bash -c "cd '$WslPath' && chmod +x scripts/build.sh && ./scripts/build.sh $a" 2>&1
}

function Invoke-Deploy {
    if(!$DeviceIP){Write-Host "-DeviceIP required" -ForegroundColor Red; exit 1}
    $Deb = Get-ChildItem -Path (Join-Path $ProjectRoot "packages\*.deb") -ErrorAction SilentlyContinue | Sort-Object LastWriteTime -Descending | Select-Object -First 1
    if(!$Deb){Write-Host "No DEB found. Build first." -ForegroundColor Red; exit 1}
    scp "$($Deb.FullName)" "root@${DeviceIP}:/tmp/" 2>&1
    ssh root@$DeviceIP "dpkg -i /tmp/$($Deb.Name) && rm /tmp/$($Deb.Name) && killall -HUP SpringBoard 2>/dev/null" 2>&1
    Write-Host "Deployed!" -ForegroundColor Green
}

if($Clean){$Action="clean"}
switch($Action){
    "clean"{Invoke-WSLBuild -Arch "clean"}
    "armv7"{Invoke-WSLBuild -Arch "armv7"}
    "arm64"{Invoke-WSLBuild -Arch "arm64"}
    "both"{Invoke-WSLBuild -Arch "both"}
    "deploy"{if(!$Clean){Invoke-WSLBuild -Arch "both"}; Invoke-Deploy}
}
