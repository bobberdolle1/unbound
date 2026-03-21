package engine

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

func RunAutoTune(ctx context.Context, updateLog func(string)) (Profile, error) {
	updateLog("🔍 Initializing Auto-Tune Scanner...")
	
	assets, err := ExtractAssets()
	if err != nil {
		updateLog("❌ Failed to extract assets: " + err.Error())
		return Profile{}, err
	}

	updateLog("✓ Assets extracted successfully")
	updateLog("📋 Loading available profiles...")

	profiles := GetProfiles(assets.LuaDir)
	updateLog(fmt.Sprintf("✓ Found %d profiles to test", len(profiles)))
	
	winwsPath := filepath.Join(assets.BinDir, "nfqws.exe")
	updateLog("🚀 Starting profile tests...")

	for i, profile := range profiles {
		updateLog(fmt.Sprintf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
		updateLog(fmt.Sprintf("🧪 Testing [%d/%d]: %s", i+1, len(profiles), profile.Name))

		absLuaLib, _ := filepath.Abs(filepath.Join(assets.LuaDir, "zapret-lib.lua"))
		absLuaAntiDpi, _ := filepath.Abs(filepath.Join(assets.LuaDir, "zapret-antidpi.lua"))
		
		luaLib := filepath.ToSlash(absLuaLib)
		luaAntiDpi := filepath.ToSlash(absLuaAntiDpi)

		args := []string{
			"--lua=\"" + luaLib + "\"",
			"--lua=\"" + luaAntiDpi + "\"",
		}
		args = append(args, profile.Args...)

		updateLog(fmt.Sprintf("   Command: nfqws.exe %v", args))

		cmd := exec.Command(winwsPath, args...)
		cmd.Dir = assets.BinDir
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

		if err := cmd.Start(); err != nil {
			updateLog("   ❌ Failed to start: " + err.Error())
			continue
		}

		updateLog("   ⏳ Waiting for WinDivert to bind...")
		time.Sleep(2 * time.Second)

		client := &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		urls := []string{"https://googlevideo.com", "https://discord.com"}
		successCount := 0
		
		for _, u := range urls {
			updateLog(fmt.Sprintf("   🌐 Testing: %s", u))
			resp, err := client.Get(u)
			if err != nil {
				updateLog(fmt.Sprintf("   ⚠️  Failed: %s", err.Error()))
			} else {
				updateLog("   ✓ Success!")
				successCount++
				if resp != nil {
					resp.Body.Close()
				}
			}
		}

		exec.Command("taskkill", "/F", "/T", "/IM", "nfqws.exe").Run()
		updateLog("   🛑 Stopped engine")
		time.Sleep(1 * time.Second)

		if successCount == len(urls) {
			updateLog("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			updateLog(fmt.Sprintf("🎉 WINNER: %s", profile.Name))
			updateLog("💾 Saving configuration...")
			
			configData, _ := json.Marshal(map[string]string{"active_profile": profile.Name})
			os.WriteFile("config.json", configData, 0644)
			
			updateLog("✅ Auto-Tune completed successfully!")
			return profile, nil
		} else {
			updateLog(fmt.Sprintf("   ❌ Failed (%d/%d tests passed)", successCount, len(urls)))
		}
	}

	updateLog("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	updateLog("❌ All profiles failed. Check your network connection.")
	return Profile{}, errors.New("all profiles failed")
}
