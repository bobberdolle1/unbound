package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"
	"unbound/engine"
	"unbound/engine/providers"
)

type TestTarget struct {
	Name     string
	URL      string
	Category string
}

var comprehensiveTargets = []TestTarget{
	// Discord
	{"Discord Main", "https://discord.com", "Discord"},
	
	// YouTube
	{"YouTube Main", "https://www.youtube.com", "YouTube"},
	
	// Telegram
	{"Telegram Web", "https://web.telegram.org", "Telegram"},
	
	// Cloudflare Sites
	{"Cloudflare 1.1.1.1", "https://1.1.1.1", "Cloudflare"},
	
	// General
	{"Google", "https://www.google.com", "General"},
}

func mainDebug() {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🔬 UNBOUND BYPASS DEBUG SUITE")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Kill any existing winws2 processes
	exec.Command("taskkill", "/F", "/IM", "winws2.exe").Run()
	time.Sleep(1 * time.Second)
	
	fmt.Println("\n[PHASE 1] Testing WITHOUT bypass (baseline)...")
	baselineResults := runTests(nil)
	
	fmt.Println("\n[PHASE 2] Starting bypass engine...")
	assets, err := engine.ExtractAssets()
	if err != nil {
		fmt.Printf("❌ Failed to extract assets: %v\n", err)
		os.Exit(1)
	}
	
	if err := engine.EnsureListsExist(); err != nil {
		fmt.Printf("⚠️  Warning: Failed to update lists: %v\n", err)
	}
	
	listsDir, _ := engine.GetListsDir()
	provider := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, listsDir, true, true)
	
	for _, prof := range engine.GetProfiles(assets.LuaDir) {
		provider.RegisterProfile(prof.Name, prof.Args)
	}
	
	targetProfile := "Unbound Ultimate (God Mode)"
	fmt.Printf("🚀 Starting profile: %s\n", targetProfile)
	
	ctx := context.Background()
	if err := provider.Start(ctx, targetProfile); err != nil {
		fmt.Printf("❌ Failed to start engine: %v\n", err)
		os.Exit(1)
	}
	
	// Monitor logs
	stopLogs := make(chan bool)
	go func() {
		lastIdx := 0
		for {
			select {
			case <-stopLogs:
				return
			case <-time.After(500 * time.Millisecond):
				logs := provider.GetLogs()
				if len(logs) > lastIdx {
					for _, line := range logs[lastIdx:] {
						fmt.Printf("   [ENGINE] %s", line)
					}
					lastIdx = len(logs)
				}
			}
		}
	}()
	
	fmt.Println("\n⏳ Waiting 5 seconds for engine to initialize...")
	time.Sleep(5 * time.Second)
	
	fmt.Println("\n[PHASE 3] Testing WITH bypass...")
	bypassResults := runTests(provider)
	
	close(stopLogs)
	provider.Stop()
	
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📊 COMPARISON REPORT")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	categories := map[string]bool{}
	for _, t := range comprehensiveTargets {
		categories[t.Category] = true
	}
	
	for category := range categories {
		fmt.Printf("\n[%s]\n", category)
		for i, target := range comprehensiveTargets {
			if target.Category != category {
				continue
			}
			
			baseline := baselineResults[i]
			bypass := bypassResults[i]
			
			status := "❌"
			if bypass.Success {
				status = "✅"
			}
			
			improvement := ""
			if !baseline.Success && bypass.Success {
				improvement = " [FIXED BY BYPASS]"
			} else if baseline.Success && !bypass.Success {
				improvement = " [BROKEN BY BYPASS]"
			}
			
			fmt.Printf("  %s %-25s | Baseline: %v | Bypass: %v%s\n",
				status, target.Name, baseline.Success, bypass.Success, improvement)
			
			if bypass.Error != "" {
				fmt.Printf("      Error: %s\n", bypass.Error)
			}
		}
	}
	
	// Summary
	baselineSuccess := 0
	bypassSuccess := 0
	for i := range baselineResults {
		if baselineResults[i].Success {
			baselineSuccess++
		}
		if bypassResults[i].Success {
			bypassSuccess++
		}
	}
	
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("SUMMARY: Baseline %d/%d | Bypass %d/%d\n", 
		baselineSuccess, len(comprehensiveTargets),
		bypassSuccess, len(comprehensiveTargets))
	
	if bypassSuccess > baselineSuccess {
		fmt.Printf("✅ BYPASS WORKING: +%d sites unblocked\n", bypassSuccess-baselineSuccess)
	} else if bypassSuccess == baselineSuccess && baselineSuccess == len(comprehensiveTargets) {
		fmt.Println("✅ ALL SITES ACCESSIBLE (no blocking detected)")
	} else if bypassSuccess < baselineSuccess {
		fmt.Printf("❌ BYPASS BROKEN: -%d sites now blocked\n", baselineSuccess-bypassSuccess)
	} else {
		fmt.Println("⚠️  BYPASS NOT EFFECTIVE: no improvement")
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

type TestResult struct {
	Success    bool
	Latency    time.Duration
	StatusCode int
	Error      string
}

func runTests(provider *providers.Zapret2WindowsProvider) []TestResult {
	results := make([]TestResult, len(comprehensiveTargets))
	
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		},
	}
	
	for i, target := range comprehensiveTargets {
		fmt.Printf("  Testing %s... ", target.Name)
		
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		
		req, err := http.NewRequestWithContext(ctx, "GET", target.URL, nil)
		if err != nil {
			results[i] = TestResult{Error: err.Error()}
			cancel()
			fmt.Printf("❌ (request error)\n")
			continue
		}
		
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		
		resp, err := client.Do(req)
		latency := time.Since(start)
		cancel()
		
		if err != nil {
			results[i] = TestResult{Error: err.Error(), Latency: latency}
			fmt.Printf("❌ (%v)\n", latency.Truncate(time.Millisecond))
			continue
		}
		
		io.Copy(io.Discard, io.LimitReader(resp.Body, 8192))
		resp.Body.Close()
		
		results[i] = TestResult{
			Success:    resp.StatusCode >= 200 && resp.StatusCode < 400,
			StatusCode: resp.StatusCode,
			Latency:    latency,
		}
		
		if results[i].Success {
			fmt.Printf("✅ (%v, HTTP %d)\n", latency.Truncate(time.Millisecond), resp.StatusCode)
		} else {
			fmt.Printf("❌ (HTTP %d)\n", resp.StatusCode)
		}
	}
	
	return results
}
