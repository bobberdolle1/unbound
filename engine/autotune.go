package engine

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"
	"unbound/engine/providers"
)

type AutoTuneResult struct {
	ProfileName string
	Success     bool
	Score       int
	Latency     time.Duration
	Results     map[string]TargetStatus
}

type TargetStatus struct {
	OK      bool
	Latency time.Duration
	TLS13   bool
	Error   string
}

type Target struct {
	Name string
	URL  string
}

var testTargets = []Target{
	{Name: "Discord", URL: "https://discord.com/api/v9/gateway"},
	{Name: "YouTube", URL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ"},
	{Name: "Telegram", URL: "https://api.telegram.org"},
	{Name: "RuTracker", URL: "https://rutracker.org/forum/index.php"},
	{Name: "Facebook", URL: "https://www.facebook.com"},
}

func RunAutoTuneV2WithContext(ctx context.Context, provider *providers.Zapret2WindowsProvider, profiles []Profile) (*AutoTuneResult, error) {
	var bestResult *AutoTuneResult

	for _, profile := range profiles {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			fmt.Printf("🔍 Testing profile: %s\n", profile.Name)

			if err := provider.Start(ctx, profile.Name); err != nil {
				continue
			}

			// Wait for stabilization
			time.Sleep(2 * time.Second)

			result := testBypassParallel(profile.Name)
			provider.Stop()
			time.Sleep(500 * time.Millisecond)

			if result.Success {
				if bestResult == nil || result.Score > bestResult.Score {
					bestResult = result
					if countOK(result) == len(testTargets) {
						return bestResult, nil
					}
				}
			}
		}
	}

	if bestResult != nil {
		return bestResult, nil
	}
	return nil, fmt.Errorf("no profile found")
}

func countOK(res *AutoTuneResult) int {
	ok := 0
	for _, s := range res.Results {
		if s.OK { ok++ }
	}
	return ok
}

func testBypassParallel(profileName string) *AutoTuneResult {
	result := &AutoTuneResult{
		ProfileName: profileName,
		Results:     make(map[string]TargetStatus),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, target := range testTargets {
		wg.Add(1)
		go func(t Target) {
			defer wg.Done()
			status := testTarget(t.URL)
			mu.Lock()
			result.Results[t.Name] = status
			mu.Unlock()
		}(target)
	}

	wg.Wait()

	score := 0
	successCount := 0
	for _, s := range result.Results {
		if s.OK {
			successCount++
			score += 10
			if s.TLS13 { score += 2 }
		}
	}

	result.Score = score
	result.Success = successCount >= 2 // Minimum 2 targets for success
	return result
}

func testTarget(url string) TargetStatus {
	start := time.Now()
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				MaxVersion: tls.VersionTLS13,
			},
			DisableKeepAlives: true,
		},
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return TargetStatus{OK: false, Error: err.Error()}
	}
	defer resp.Body.Close()

	return TargetStatus{
		OK:      resp.StatusCode < 500,
		Latency: time.Since(start),
		TLS13:   resp.TLS != nil && resp.TLS.Version == tls.VersionTLS13,
	}
}
