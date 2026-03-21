package engine

import (
"context"
"encoding/json"
"errors"
"fmt"
"os"
"os/exec"
"path/filepath"
"syscall"
"time"
)

var defaultTestTargets = []string{
"https://discord.com",
"https://googlevideo.com",
"https://youtube.com",
}

func RunAutoTune(ctx context.Context, updateLog func(string)) (Profile, error) {
updateLog("Initializing Auto-Tune Scanner with Smart Prober...")

assets, err := ExtractAssets()
if err != nil {
updateLog("Failed to extract assets: " + err.Error())
return Profile{}, err
}

updateLog("Assets extracted successfully")
updateLog("Loading available profiles...")

profiles := GetProfiles(assets.LuaDir)
updateLog(fmt.Sprintf("Found %d profiles to test", len(profiles)))

updateLog("Starting profile tests with TLS certificate verification...")

bestProfile := Profile{}
bestScore := -1
bestLatency := time.Duration(0)

for i, profile := range profiles {
updateLog(fmt.Sprintf("Testing [%d/%d]: %s", i+1, len(profiles), profile.Name))

winwsPath := filepath.Join(assets.BinDir, "winws.exe")
absLuaLib, _ := filepath.Abs(filepath.Join(assets.LuaDir, "zapret-lib.lua"))
absLuaAntiDpi, _ := filepath.Abs(filepath.Join(assets.LuaDir, "zapret-antidpi.lua"))

luaLib := filepath.ToSlash(absLuaLib)
luaAntiDpi := filepath.ToSlash(absLuaAntiDpi)

args := []string{
"--intercept=1",
"--lua-init=@" + luaLib,
"--lua-init=@" + luaAntiDpi,
}
args = append(args, profile.Args...)

cmd := exec.Command(winwsPath, args...)
cmd.Dir = assets.BinDir
cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

if err := cmd.Start(); err != nil {
updateLog(fmt.Sprintf("Failed to start: %s", err.Error()))
continue
}

updateLog("Waiting for WinDivert to bind (2s)...")
time.Sleep(2 * time.Second)

updateLog("Running TLS certificate verification probes...")
probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
results := ProbeMultipleTargets(probeCtx, defaultTestTargets, nil)
cancel()

successCount := 0
totalLatency := time.Duration(0)

for _, result := range results {
if result.Success && result.CertValid {
successCount++
totalLatency += result.Latency
updateLog(fmt.Sprintf("%s: %dms (Cert: %s)", result.URL, result.Latency.Milliseconds(), result.CertIssuer))
} else {
reason := result.Error
if result.ConnectionRST {
reason = "Connection RESET by DPI"
} else if !result.CertValid {
reason = fmt.Sprintf("Invalid cert from: %s", result.CertIssuer)
}
updateLog(fmt.Sprintf("%s: %s", result.URL, reason))
}
}

exec.Command("taskkill", "/F", "/T", "/IM", "winws.exe").Run()
updateLog("Stopped engine")
time.Sleep(1 * time.Second)

score := CalculateProbeScore(results)
avgLatency := time.Duration(0)
if successCount > 0 {
avgLatency = totalLatency / time.Duration(successCount)
}

updateLog(fmt.Sprintf("Score: %d | Success: %d/%d | Avg Latency: %dms", 
score, successCount, len(defaultTestTargets), avgLatency.Milliseconds()))

if score > bestScore || (score == bestScore && avgLatency < bestLatency) {
bestScore = score
bestProfile = profile
bestLatency = avgLatency
updateLog(fmt.Sprintf("New best profile!"))
}
}

if bestScore <= 0 {
updateLog("All profiles failed. Check network or DPI is too aggressive.")
return Profile{}, errors.New("all profiles failed")
}

updateLog(fmt.Sprintf("WINNER: %s (Score: %d, Latency: %dms)", 
bestProfile.Name, bestScore, bestLatency.Milliseconds()))
updateLog("Saving configuration...")

configData, _ := json.Marshal(map[string]string{"active_profile": bestProfile.Name})
os.WriteFile("config.json", configData, 0644)

updateLog("Auto-Tune completed successfully!")
return bestProfile, nil
}