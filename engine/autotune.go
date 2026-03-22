package engine

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"
	"unbound/engine/providers"
)

type AutoTuneResult struct {
	ProfileName string
	Success     bool
	Latency     time.Duration
	Error       error
	TestedURLs  map[string]bool
}

var testTargets = []string{
	"https://discord.com",
	"https://www.youtube.com",
	"https://twitter.com",
	"https://web.telegram.org",
}

func RunAutoTuneV2(provider *providers.Zapret2WindowsProvider, profiles []Profile) (*AutoTuneResult, error) {
	ctx := context.Background()
	
	for _, profile := range profiles {
		fmt.Printf("🔍 Testing profile: %s\n", profile.Name)
		
		if err := provider.Start(ctx, profile.Name); err != nil {
			fmt.Printf("❌ Failed to start: %v\n", err)
			continue
		}
		
		ready := provider.WaitReady(3 * time.Second)
		if !ready {
			fmt.Printf("⏱️ Engine initialization timeout\n")
			provider.Stop()
			continue
		}
		
		time.Sleep(1 * time.Second)
		
		result := testBypass(profile.Name)
		
		provider.Stop()
		time.Sleep(500 * time.Millisecond)
		
		if result.Success {
			fmt.Printf("✅ Profile works: %s\n", profile.Name)
			return result, nil
		}
		
		fmt.Printf("❌ Profile failed: %s (%v)\n", profile.Name, result.Error)
	}
	
	return nil, fmt.Errorf("no working profile found")
}

func testBypass(profileName string) *AutoTuneResult {
	result := &AutoTuneResult{
		ProfileName: profileName,
		TestedURLs:  make(map[string]bool),
	}
	
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   3 * time.Second,
				KeepAlive: 0,
			}).DialContext,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
			DisableKeepAlives:     true,
			MaxIdleConns:          1,
			IdleConnTimeout:       1 * time.Second,
			TLSHandshakeTimeout:   3 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	
	successCount := 0
	start := time.Now()
	
	for _, target := range testTargets {
		req, err := http.NewRequest("GET", target, nil)
		if err != nil {
			result.TestedURLs[target] = false
			continue
		}
		
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		
		resp, err := client.Do(req)
		if err != nil {
			result.TestedURLs[target] = false
			result.Error = err
			continue
		}
		
		resp.Body.Close()
		
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			result.TestedURLs[target] = true
			successCount++
		} else {
			result.TestedURLs[target] = false
		}
	}
	
	result.Latency = time.Since(start)
	result.Success = successCount >= 2
	
	return result
}

func QuickTest(provider *providers.Zapret2WindowsProvider, profileName string) (*AutoTuneResult, error) {
	ctx := context.Background()
	
	if err := provider.Start(ctx, profileName); err != nil {
		return nil, fmt.Errorf("failed to start engine: %w", err)
	}
	
	defer provider.Stop()
	
	if !provider.WaitReady(3 * time.Second) {
		return nil, fmt.Errorf("engine initialization timeout")
	}
	
	time.Sleep(1 * time.Second)
	
	result := testBypass(profileName)
	return result, nil
}
