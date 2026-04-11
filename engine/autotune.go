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
	Name     string
	URL      string
	Priority int // приоритет при подсчёте очков
}

// Расширенный список целей для тестирования — отражает реальные российские блокировки
var testTargets = []Target{
	// Высокий приоритет — основные цели (30 очков каждый)
	{Name: "YouTube",   URL: "https://www.youtube.com/favicon.ico",        Priority: 30},
	{Name: "Discord",   URL: "https://discord.com/favicon.ico",            Priority: 30},
	{Name: "Instagram", URL: "https://www.instagram.com/favicon.ico",      Priority: 20},
	// Средний приоритет — часто блокируемые
	{Name: "Twitter/X", URL: "https://twitter.com/favicon.ico",            Priority: 15},
	{Name: "Facebook",  URL: "https://www.facebook.com/favicon.ico",       Priority: 15},
	{Name: "RuTracker", URL: "https://rutracker.org/favicon.ico",          Priority: 15},
	// Низкий приоритет — VPN и прочее
	{Name: "NordVPN",   URL: "https://nordvpn.com/favicon.ico",            Priority: 10},
	{Name: "Proton",    URL: "https://proton.me/favicon.ico",              Priority: 10},
}

func RunAutoTuneV2WithContext(ctx context.Context, provider providers.BypassProvider, profiles []Profile) (*AutoTuneResult, error) {
	logger := GetLogger()
	notifMgr := GetNotificationManager()
	
	logger.Info("AutoTune", "Запуск AutoTune V2")
	logger.Infof("AutoTune", "Тестируем %d профилей на %d целях", len(profiles), len(testTargets))
	
	var bestResult *AutoTuneResult
	testedCount := 0

	for _, profile := range profiles {
		select {
		case <-ctx.Done():
			logger.Warn("AutoTune", "AutoTune отменён пользователем")
			notifMgr.Warning("AutoTune", "Процесс отменён")
			return nil, ctx.Err()
		default:
			testedCount++
			logger.Infof("AutoTune", "[%d/%d] Тестируем профиль: %s", testedCount, len(profiles), profile.Name)

			if err := provider.Start(ctx, profile.Name); err != nil {
				logger.Warnf("AutoTune", "Не удалось запустить профиль %s: %v", profile.Name, err)
				continue
			}

			// Ждём стабилизации
			time.Sleep(2 * time.Second)

			result := testBypassParallel(profile.Name)
			provider.Stop()
			time.Sleep(500 * time.Millisecond)

			// Логируем детали
			okCount := countOK(result)
			logger.Infof("AutoTune", "Профиль %s: %d/%d целей OK, счёт=%d", 
				profile.Name, okCount, len(testTargets), result.Score)
			
			for targetName, status := range result.Results {
				if status.OK {
					logger.Debugf("AutoTune", "  ✓ %s: %dмс (TLS1.3=%v)", 
						targetName, status.Latency.Milliseconds(), status.TLS13)
				} else {
					logger.Debugf("AutoTune", "  ✗ %s: %s", targetName, status.Error)
				}
			}

			if result.Success {
				if bestResult == nil || result.Score > bestResult.Score {
					bestResult = result
					logger.Infof("AutoTune", "Новый лучший профиль: %s (счёт=%d)", profile.Name, result.Score)
					
					// Если YouTube и Discord оба работают — это уже отличный результат
					ytOK := result.Results["YouTube"].OK
					dcOK := result.Results["Discord"].OK
					if ytOK && dcOK && okCount >= len(testTargets)/2 {
						logger.Infof("AutoTune", "Отличный профиль найден: %s", profile.Name)
						notifMgr.Success("AutoTune завершён", fmt.Sprintf("Лучший профиль: %s", profile.Name))
						return bestResult, nil
					}
				}
			}
		}
	}

	if bestResult != nil {
		logger.Infof("AutoTune", "AutoTune завершён. Лучший профиль: %s (счёт=%d)", 
			bestResult.ProfileName, bestResult.Score)
		notifMgr.Success("AutoTune завершён", fmt.Sprintf("Лучший профиль: %s", bestResult.ProfileName))
		return bestResult, nil
	}
	
	logger.Error("AutoTune", "Подходящий профиль не найден")
	notifMgr.Error("AutoTune не удался", "Рабочий профиль не найден")
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
	logger := GetLogger()
	result := &AutoTuneResult{
		ProfileName: profileName,
		Results:     make(map[string]TargetStatus),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	logger.Debugf("AutoTune", "Параллельное тестирование %d целей для профиля: %s", len(testTargets), profileName)

	for _, target := range testTargets {
		wg.Add(1)
		go func(t Target) {
			defer wg.Done()
			// Add strict 5-second timeout per probe
			probeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			done := make(chan TargetStatus, 1)
			go func() {
				status := testTargetWithContext(probeCtx, t.URL)
				done <- status
			}()
			
			select {
			case status := <-done:
				mu.Lock()
				result.Results[t.Name] = status
				mu.Unlock()
			case <-probeCtx.Done():
				logger.Warnf("AutoTune", "Target %s timed out after 5s", t.Name)
				mu.Lock()
				result.Results[t.Name] = TargetStatus{OK: false, Error: "timeout after 5s"}
				mu.Unlock()
			}
		}(target)
	}

	wg.Wait()

	score := 0
	successCount := 0
	totalLatency := time.Duration(0)
	
	for _, t := range testTargets {
		s := result.Results[t.Name]
		if s.OK {
			successCount++
			score += t.Priority
			if s.TLS13 { 
				score += 3
			}
			// Бонус за низкий пинг
			if s.Latency < 150*time.Millisecond {
				score += 5
			}
			totalLatency += s.Latency
		}
	}

	if successCount > 0 {
		result.Latency = totalLatency / time.Duration(successCount)
	}

	result.Score = score
	result.Success = successCount >= 2 // Минимум 2 цели для признания успеха
	
	logger.Debugf("AutoTune", "Профиль %s: %d/%d OK, счёт=%d, ср.пинг=%dмс", 
		profileName, successCount, len(testTargets), score, result.Latency.Milliseconds())
	
	return result
}

func testTarget(url string) TargetStatus {
	return testTargetWithContext(context.Background(), url)
}

func testTargetWithContext(ctx context.Context, url string) TargetStatus {
	logger := GetLogger()
	start := time.Now()
	
	// Используем HEAD для скорости (как probe.trolling.website)
	client := &http.Client{
		Timeout: 5 * time.Second, // Strict 5-second timeout
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				MaxVersion: tls.VersionTLS13,
			},
			DisableKeepAlives: true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return TargetStatus{OK: false, Error: err.Error()}
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		// Попробуем GET если HEAD не сработал
		req2, err2 := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err2 != nil {
			return TargetStatus{OK: false, Error: err2.Error()}
		}
		req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		resp2, err3 := client.Do(req2)
		if err3 != nil {
			logger.Debugf("AutoTune", "Цель %s недоступна: %v", url, err3)
			return TargetStatus{OK: false, Error: err3.Error()}
		}
		defer resp2.Body.Close()
		latency := time.Since(start)
		isTLS13 := resp2.TLS != nil && resp2.TLS.Version == tls.VersionTLS13
		isOK := resp2.StatusCode < 500
		return TargetStatus{OK: isOK, Latency: latency, TLS13: isTLS13}
	}
	defer resp.Body.Close()

	latency := time.Since(start)
	isTLS13 := resp.TLS != nil && resp.TLS.Version == tls.VersionTLS13
	isOK := resp.StatusCode < 500

	logger.Debugf("AutoTune", "Цель %s: статус=%d, пинг=%dмс, TLS1.3=%v", 
		url, resp.StatusCode, latency.Milliseconds(), isTLS13)

	return TargetStatus{
		OK:      isOK,
		Latency: latency,
		TLS13:   isTLS13,
	}
}
