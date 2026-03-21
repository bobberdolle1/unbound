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
	
	"unbound/engine/providers"
)

var defaultTestTargets = []string{
	"https://discord.com",
	"https://googlevideo.com",
	"https://youtube.com",
}

func RunAutoTune(ctx context.Context, updateLog func(string)) (Profile, error) {
	logAndUpdate := func(msg string) {
		providers.WriteLog("[AUTO-TUNE] " + msg)
		updateLog(msg)
	}
	
	logAndUpdate("Initializing Auto-Tune Scanner with Smart Prober...")

	assets, err := ExtractAssets()
	if err != nil {
		logAndUpdate("Failed to extract assets: " + err.Error())
		return Profile{}, err
	}

	logAndUpdate("Assets extracted successfully")
	logAndUpdate("Loading available profiles...")

	profiles := GetProfiles(assets.LuaDir)
	logAndUpdate(fmt.Sprintf("Found %d profiles to test", len(profiles)))

	logAndUpdate("Starting profile tests...")

	bestProfile := Profile{}
	bestScore := -1
	bestLatency := time.Duration(0)

	for i, profile := range profiles {
		logAndUpdate(fmt.Sprintf("Testing [%d/%d]: %s", i+1, len(profiles), profile.Name))

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
			logAndUpdate(fmt.Sprintf("Failed to start: %s", err.Error()))
			continue
		}

		logAndUpdate("Waiting for WinDivert to bind (2s)...")
		time.Sleep(2 * time.Second)

		logAndUpdate("Running connectivity probes...")
		probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		results := ProbeMultipleTargets(probeCtx, defaultTestTargets, nil)
		cancel()

		successCount := 0
		totalLatency := time.Duration(0)

		for _, result := range results {
			if result.Success {
				successCount++
				totalLatency += result.Latency
				logAndUpdate(fmt.Sprintf("%s: %dms ✓", result.URL, result.Latency.Milliseconds()))
			} else {
				logAndUpdate(fmt.Sprintf("%s: FAILED", result.URL))
			}
		}

		exec.Command("taskkill", "/F", "/T", "/IM", "winws.exe").Run()
		logAndUpdate("Stopped engine")
		time.Sleep(1 * time.Second)

		score := CalculateProbeScore(results)
		avgLatency := time.Duration(0)
		if successCount > 0 {
			avgLatency = totalLatency / time.Duration(successCount)
		}

		logAndUpdate(fmt.Sprintf("Score: %d | Success: %d/%d | Avg Latency: %dms", 
			score, successCount, len(defaultTestTargets), avgLatency.Milliseconds()))

		if score > bestScore || (score == bestScore && avgLatency < bestLatency) {
			bestScore = score
			bestProfile = profile
			bestLatency = avgLatency
			logAndUpdate(fmt.Sprintf("New best profile!"))
		}
	}

	if bestScore <= 0 {
		logAndUpdate("All profiles failed. Selecting first profile as fallback...")
		if len(profiles) > 0 {
			bestProfile = profiles[0]
			logAndUpdate(fmt.Sprintf("FALLBACK: %s", bestProfile.Name))
		} else {
			logAndUpdate("No profiles available.")
			return Profile{}, errors.New("no profiles available")
		}
	} else {
		logAndUpdate(fmt.Sprintf("WINNER: %s (Score: %d, Latency: %dms)", 
			bestProfile.Name, bestScore, bestLatency.Milliseconds()))
	}
	logAndUpdate("Saving configuration...")

	configData, _ := json.Marshal(map[string]string{"active_profile": bestProfile.Name})
	os.WriteFile("config.json", configData, 0644)

	logAndUpdate("Auto-Tune completed successfully!")
	return bestProfile, nil
}
