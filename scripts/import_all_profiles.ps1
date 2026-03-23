# Import All Reference Profiles to Go Code
# Converts all .txt presets to Go Profile structs

$projectRoot = Split-Path -Parent $PSScriptRoot
$presetsDir = Join-Path $projectRoot "reference\zapret2-youtube-discord-main\presets"
$outputFile = Join-Path $projectRoot "engine\strategies_imported.go"

$presets = Get-ChildItem -Path $presetsDir -Filter "*.txt" | Where-Object { 
    $_.Name -notlike "*game filter*" 
} | Sort-Object Name

$goCode = @"
package engine

import "path/filepath"

// Auto-generated profiles from reference implementation
func GetImportedProfiles(luaDir string) []Profile {
	listsDir, _ := GetListsDir()
	windivertDir, _ := GetWinDivertFilterDir()
	
	return []Profile{
"@

foreach ($preset in $presets) {
    $name = $preset.BaseName
    $content = Get-Content $preset.FullName
    
    $goCode += "`n`t`t{`n"
    $goCode += "`t`t`tName: `"$name`",`n"
    $goCode += "`t`t`tArgs: []string{`n"
    
    foreach ($line in $content) {
        $line = $line.Trim()
        if ($line -and -not $line.StartsWith("#")) {
            # Convert paths
            $line = $line -replace '@lua/', '@" + filepath.ToSlash(filepath.Join(luaDir, "'
            $line = $line -replace '@bin/', '@" + filepath.ToSlash(filepath.Join(binDir, "'
            $line = $line -replace '@windivert\.filter/', '@" + filepath.ToSlash(filepath.Join(windivertDir, "'
            $line = $line -replace 'lists/', '" + filepath.ToSlash(filepath.Join(listsDir, "'
            
            # Escape quotes
            $line = $line -replace '"', '\"'
            
            $goCode += "`t`t`t`t`"$line`",`n"
        }
    }
    
    $goCode += "`t`t`t},`n"
    $goCode += "`t`t},`n"
}

$goCode += @"
	}
}
"@

$goCode | Out-File $outputFile -Encoding UTF8
Write-Host "Generated $outputFile with $($presets.Count) profiles" -ForegroundColor Green
