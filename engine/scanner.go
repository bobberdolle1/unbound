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
	assets, err := ExtractAssets()
	if err != nil {
		return Profile{}, err
	}

	profiles := GetProfiles(assets.LuaDir)
	winwsPath := filepath.Join(assets.BinDir, "nfqws.exe")

	for _, profile := range profiles {
		updateLog("Testing profile: " + profile.Name)

		// Create a temporary lua script for the profile
		tmpLuaPath := filepath.Join(os.TempDir(), "autotune_profile.lua")
		
		// Ensure scanner.go actually writes the profile into a temporary .lua file
		luaContent := fmt.Sprintf("-- Profile Strategy: %s\n", profile.Name)
		err := os.WriteFile(tmpLuaPath, []byte(luaContent), 0644)
		if err != nil {
			updateLog("Failed to write lua file: " + err.Error())
			continue
		}

		// Fixed Zapret 2 Syntax
		args := []string{
			"--filter-tcp=80,443", 
			"--filter-udp=50000-65535",
			"--lua=\"" + tmpLuaPath + "\"",
		}
		// Append profile specific flags
		args = append(args, profile.Args...)

		cmd := exec.Command(winwsPath, args...)
		cmd.Dir = assets.BinDir
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

		if err := cmd.Start(); err != nil {
			updateLog("Failed to start nfqws: " + err.Error())
			continue
		}

		time.Sleep(1 * time.Second) // wait for WinDivert to bind

		// Ensure HTTP Client ignores TLS cert errors
		client := &http.Client{
			Timeout: 4 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		urls := []string{"https://googlevideo.com", "https://discord.com"}
		success := true
		for _, u := range urls {
			resp, err := client.Get(u)
			if err != nil {
				success = false
				break
			}
			if resp != nil {
				resp.Body.Close()
			}
		}

		// kill process
		exec.Command("taskkill", "/F", "/T", "/IM", "nfqws.exe").Run()
		time.Sleep(1 * time.Second) // wait for kernel to free WinDivert handle
		os.Remove(tmpLuaPath)

		if success {
			updateLog("Success! Profile selected: " + profile.Name)
			// save to config.json
			configData, _ := json.Marshal(map[string]string{"active_profile": profile.Name})
			os.WriteFile("config.json", configData, 0644)
			return profile, nil
		} else {
			updateLog("Failed: " + profile.Name)
		}
	}

	return Profile{}, errors.New("all profiles failed")
}
