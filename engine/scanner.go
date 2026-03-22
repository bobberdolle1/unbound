package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	
	"unbound/engine/providers"
)

var defaultTestTargets = []string{
	"https://discord.com",
	"https://cdn.discordapp.com/attachments/1234567890/1234567890/test.txt",
	"https://api.telegram.org/",
	"https://googlevideo.com",
	"https://youtube.com",
}

var (
	autoTuneCache Profile
	cacheTime     time.Time
	cacheMutex    sync.Mutex
)

func RunAutoTune(ctx context.Context, updateLog func(string)) (Profile, error) {
	cacheMutex.Lock()
	if !cacheTime.IsZero() && time.Since(cacheTime) < 1*time.Hour {
		cached := autoTuneCache
		cacheMutex.Unlock()
		updateLog("Using cached Auto-Tune result from previous run")
		return cached, nil
	}
	cacheMutex.Unlock()

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
	
	logAndUpdate("Detecting DPI distance with AutoTTL (Binary Search)...")
	
	// Pre-check DNS
	logAndUpdate("Checking DNS resolution...")
	for _, target := range defaultTestTargets {
		host := extractHost(target)
		ips, err := net.LookupIP(host)
		if err != nil {
			logAndUpdate(fmt.Sprintf("WARNING: DNS resolution failed for %s: %v. Bypassing may fail.", host, err))
		} else {
			logAndUpdate(fmt.Sprintf("DNS OK: %s -> %v", host, ips[0]))
		}
	}

	ttlMap := AutoTTLForProfile(ctx, defaultTestTargets)
	optimalTTL := GetOptimalTTL(ttlMap)
	logAndUpdate(fmt.Sprintf("Optimal TTL detected: %d hops", optimalTTL))
	
	logAndUpdate("Loading available profiles...")

	zapretProfiles := GetProfiles(assets.LuaDir)
	advancedProfiles := GetAdvancedProfiles(assets.LuaDir)
	
	allProfiles := make([]Profile, 0, len(zapretProfiles)+len(advancedProfiles))
	for _, p := range zapretProfiles {
		allProfiles = append(allProfiles, p)
	}
	for _, ap := range advancedProfiles {
		allProfiles = append(allProfiles, Profile{Name: ap.Name, Args: ap.Args})
	}
	
	totalProfiles := len(allProfiles)
	logAndUpdate(fmt.Sprintf("Found %d profiles to test", totalProfiles))

	var bestProfile Profile
	var bestScore int = -1
	var bestLatency time.Duration = 9999999 * time.Millisecond
	var mu sync.Mutex

	var testedCount int32

	maxConcurrent := 4
	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for i, profile := range allProfiles {
		wg.Add(1)
		
		go func(idx int, p Profile) {
			defer wg.Done()
			
			select {
			case <-ctx.Done():
				return
			case semaphore <- struct{}{}:
			}
			defer func() { <-semaphore }()

			current := atomic.AddInt32(&testedCount, 1)
			logAndUpdate(fmt.Sprintf("Progress: [%d/%d] Testing: %s", current, totalProfiles, p.Name))

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
			args = append(args, p.Args...)

			cmd := exec.CommandContext(ctx, winwsPath, args...)
			cmd.Dir = assets.BinDir
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

			if err := cmd.Start(); err != nil {
				return
			}

			// Wait a bit or exit if context is cancelled
			select {
			case <-ctx.Done():
				exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run()
				return
			case <-time.After(500 * time.Millisecond):
			}

			probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			results := ProbeMultipleTargets(probeCtx, defaultTestTargets, nil)
			cancel()

			successCount := 0
			totalLat := time.Duration(0)

			for _, result := range results {
				if result.Success {
					successCount++
					totalLat += result.Latency
				}
			}

			exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run()

			if ctx.Err() != nil {
				return
			}

			score := CalculateProbeScore(results)
			avgLatency := time.Duration(9999999 * time.Millisecond)
			if successCount > 0 {
				avgLatency = totalLat / time.Duration(successCount)
			}

			mu.Lock()
			if score > bestScore || (score == bestScore && avgLatency < bestLatency) {
				bestScore = score
				bestProfile = p
				bestLatency = avgLatency
			}
			mu.Unlock()
			
		}(i, profile)
	}

	wg.Wait()

	// Kill any dangling processes just in case
	exec.Command("taskkill", "/F", "/T", "/IM", "winws.exe").Run()

	if bestScore <= 0 {
		logAndUpdate("All profiles failed. Selecting first profile as fallback...")
		if len(allProfiles) > 0 {
			bestProfile = allProfiles[0]
			logAndUpdate(fmt.Sprintf("FALLBACK: %s", bestProfile.Name))
		} else {
			return Profile{}, errors.New("no profiles available")
		}
	} else {
		logAndUpdate(fmt.Sprintf("WINNER: %s (Score: %d, Latency: %dms)", 
			bestProfile.Name, bestScore, bestLatency.Milliseconds()))
	}
	
	cacheMutex.Lock()
	autoTuneCache = bestProfile
	cacheTime = time.Now()
	cacheMutex.Unlock()

	configData, _ := json.Marshal(map[string]string{"active_profile": bestProfile.Name})
	os.WriteFile("config.json", configData, 0644)

	logAndUpdate("Auto-Tune completed successfully!")
	return bestProfile, nil
}
