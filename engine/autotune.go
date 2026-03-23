package engine

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
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
	{Name: "Google", URL: "https://www.google.com"},
}

func RunAutoTuneV2(provider *providers.Zapret2WindowsProvider, profiles []Profile) (*AutoTuneResult, error) {
	ctx := context.Background()
	var bestResult *AutoTuneResult

	for _, profile := range profiles {
		fmt.Printf("🔍 Testing profile: %s\n", profile.Name)

		if err := provider.Start(ctx, profile.Name); err != nil {
			fmt.Printf("❌ Failed to start: %v\n", err)
			continue
		}

		// Wait for engine to initialize WinDivert
		ready := provider.WaitReady(5 * time.Second)
		if !ready {
			fmt.Printf("⏱️ Engine initialization timeout\n")
			provider.Stop()
			continue
		}

		// Пауза для стабилизации сетевого стека
		time.Sleep(2 * time.Second)

		result := testBypassParallel(profile.Name)
		provider.Stop()
		time.Sleep(500 * time.Millisecond)

		fmt.Printf("📊 Score: %d | OK: %d/%d\n", result.Score, countOK(result), len(testTargets))

		if result.Success {
			if bestResult == nil || result.Score > bestResult.Score {
				bestResult = result
				// Если профиль идеален (все цели доступны), можно закончить досрочно
				if countOK(result) == len(testTargets) {
					fmt.Printf("⭐ Perfect profile found: %s\n", profile.Name)
					return bestResult, nil
				}
			}
		}
	}

	if bestResult != nil {
		return bestResult, nil
	}

	return nil, fmt.Errorf("no working profile found")
}

func countOK(res *AutoTuneResult) int {
	ok := 0
	for _, s := range res.Results {
		if s.OK {
			ok++
		}
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

	// Расчет очков (как в референсе: OK тесты + бонусы за TLS 1.3)
	score := 0
	successCount := 0
	for _, s := range result.Results {
		if s.OK {
			successCount++
			score += 10
			if s.TLS13 {
				score += 2
			}
		}
	}

	result.Score = score
	// Считаем успех, если хотя бы 50% целей доступны (в 2026-м 100% — редкость)
	result.Success = successCount >= (len(testTargets) / 2)
	
	return result
}

func testTarget(url string) TargetStatus {
	start := time.Now()
	
	// Тестируем с поддержкой TLS 1.3 (самый сложный для DPI)
	client := &http.Client{
		Timeout: 7 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   4 * time.Second,
				KeepAlive: 0,
			}).DialContext,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				MaxVersion: tls.VersionTLS13,
			},
			DisableKeepAlives: true,
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return TargetStatus{OK: false, Error: err.Error()}
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return TargetStatus{OK: false, Error: err.Error()}
	}
	defer resp.Body.Close()

	isTLS13 := resp.TLS != nil && resp.TLS.Version == tls.VersionTLS13

	// В 2026-м статус 200, 403 или даже 404 — это часто УСПЕХ, 
	// если соединение не было разорвано по TLS/TCP.
	return TargetStatus{
		OK:      resp.StatusCode < 500,
		Latency: time.Since(start),
		TLS13:   isTLS13,
	}
}
