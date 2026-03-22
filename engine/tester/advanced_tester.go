package tester

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

type TestMode string

const (
	TestModeStandard   TestMode = "standard"
	TestModeDPIChecker TestMode = "dpi_checker"
)

type AdvancedTestConfig struct {
	Mode            TestMode
	MaxConcurrent   int
	Timeout         time.Duration
	URLs            []string
	CheckTCPFreeze  bool
	MinDownloadSize int64
}

type AdvancedTestResult struct {
	ProfileName     string
	Mode            TestMode
	Results         []TestResult
	Score           int
	TCPFreezeDetected bool
	AverageLatency  time.Duration
	SuccessRate     float64
	Recommendation  string
}

type ProfileTestResult struct {
	ProfileName string
	Result      AdvancedTestResult
	Error       error
}

func RunAdvancedTests(ctx context.Context, profiles []string, config AdvancedTestConfig, startProfile func(string) error, stopProfile func() error) []AdvancedTestResult {
	results := make([]AdvancedTestResult, 0, len(profiles))
	
	for _, profile := range profiles {
		if err := startProfile(profile); err != nil {
			results = append(results, AdvancedTestResult{
				ProfileName:    profile,
				Mode:           config.Mode,
				Recommendation: fmt.Sprintf("Failed to start: %v", err),
			})
			continue
		}

		time.Sleep(2 * time.Second)

		result := testSingleProfile(ctx, profile, config)
		results = append(results, result)

		stopProfile()
		time.Sleep(1 * time.Second)
	}

	return results
}

func RunParallelTests(ctx context.Context, profiles []string, config AdvancedTestConfig, startProfile func(string) error, stopProfile func() error) []AdvancedTestResult {
	maxWorkers := config.MaxConcurrent
	if maxWorkers <= 0 {
		maxWorkers = 4
	}

	resultsChan := make(chan ProfileTestResult, len(profiles))
	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for _, profile := range profiles {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := startProfile(p); err != nil {
				resultsChan <- ProfileTestResult{
					ProfileName: p,
					Error:       err,
				}
				return
			}

			time.Sleep(2 * time.Second)

			result := testSingleProfile(ctx, p, config)
			resultsChan <- ProfileTestResult{
				ProfileName: p,
				Result:      result,
			}

			stopProfile()
			time.Sleep(500 * time.Millisecond)
		}(profile)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	results := make([]AdvancedTestResult, 0, len(profiles))
	for res := range resultsChan {
		if res.Error != nil {
			results = append(results, AdvancedTestResult{
				ProfileName:    res.ProfileName,
				Mode:           config.Mode,
				Recommendation: fmt.Sprintf("Failed: %v", res.Error),
			})
		} else {
			results = append(results, res.Result)
		}
	}

	return results
}

func testSingleProfile(ctx context.Context, profileName string, config AdvancedTestConfig) AdvancedTestResult {
	result := AdvancedTestResult{
		ProfileName: profileName,
		Mode:        config.Mode,
	}

	if config.Mode == TestModeDPIChecker {
		result.Results = testDPIChecker(ctx, config)
		result.TCPFreezeDetected = detectTCPFreeze(result.Results)
	} else {
		result.Results = TestProfile(ctx, config.URLs, config.Timeout)
	}

	result.Score = CalculateScore(result.Results)
	result.AverageLatency = calculateAverageLatency(result.Results)
	result.SuccessRate = calculateSuccessRate(result.Results)
	result.Recommendation = generateRecommendation(result)

	return result
}

func testDPIChecker(ctx context.Context, config AdvancedTestConfig) []TestResult {
	dpiTestURLs := []string{
		"https://www.youtube.com/generate_204",
		"https://discord.com/api/v9/gateway",
		"https://web.telegram.org/",
	}

	if len(config.URLs) > 0 {
		dpiTestURLs = config.URLs
	}

	results := make([]TestResult, 0, len(dpiTestURLs))
	
	client := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
			DialContext: (&net.Dialer{
				Timeout:   config.Timeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   config.Timeout,
			ResponseHeaderTimeout: config.Timeout,
		},
	}

	for _, url := range dpiTestURLs {
		result := TestResult{URL: url}
		start := time.Now()

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		resp, err := client.Do(req)
		result.Latency = time.Since(start)

		if err != nil {
			result.Error = err.Error()
			
			if result.Latency > 15*time.Second {
				result.Error += " [TCP FREEZE DETECTED]"
			}
			
			results = append(results, result)
			continue
		}
		defer resp.Body.Close()

		if config.CheckTCPFreeze && config.MinDownloadSize > 0 {
			downloaded, _ := io.Copy(io.Discard, io.LimitReader(resp.Body, config.MinDownloadSize))
			
			if downloaded < config.MinDownloadSize && result.Latency > 10*time.Second {
				result.Error = "Incomplete download - possible TCP freeze"
				result.Success = false
			} else {
				result.StatusCode = resp.StatusCode
				result.Success = resp.StatusCode >= 200 && resp.StatusCode < 400
			}
		} else {
			result.StatusCode = resp.StatusCode
			result.Success = resp.StatusCode >= 200 && resp.StatusCode < 400
		}

		results = append(results, result)
	}

	return results
}

func detectTCPFreeze(results []TestResult) bool {
	for _, r := range results {
		if r.Latency > 15*time.Second || (r.Error != "" && r.Latency > 10*time.Second) {
			return true
		}
	}
	return false
}

func calculateAverageLatency(results []TestResult) time.Duration {
	if len(results) == 0 {
		return 0
	}

	var total time.Duration
	count := 0

	for _, r := range results {
		if r.Success {
			total += r.Latency
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return total / time.Duration(count)
}

func calculateSuccessRate(results []TestResult) float64 {
	if len(results) == 0 {
		return 0
	}

	successful := 0
	for _, r := range results {
		if r.Success {
			successful++
		}
	}

	return float64(successful) / float64(len(results)) * 100
}

func generateRecommendation(result AdvancedTestResult) string {
	if result.TCPFreezeDetected {
		return "NOT RECOMMENDED - TCP freeze detected, DPI is blocking this profile"
	}

	if result.SuccessRate == 100 {
		if result.AverageLatency < 500*time.Millisecond {
			return "EXCELLENT - Perfect success rate with low latency"
		} else if result.AverageLatency < 2*time.Second {
			return "GOOD - Perfect success rate with acceptable latency"
		} else {
			return "ACCEPTABLE - Works but high latency"
		}
	}

	if result.SuccessRate >= 75 {
		return "MODERATE - Most tests passed but some failures"
	}

	if result.SuccessRate >= 50 {
		return "POOR - Many failures, try another profile"
	}

	return "FAILED - Profile does not work"
}

func FindBestProfile(results []AdvancedTestResult) *AdvancedTestResult {
	if len(results) == 0 {
		return nil
	}

	var best *AdvancedTestResult
	bestScore := -1

	for i := range results {
		r := &results[i]
		
		if r.TCPFreezeDetected {
			continue
		}

		combinedScore := r.Score
		if r.SuccessRate == 100 {
			combinedScore += 200
		}
		
		if r.AverageLatency < 500*time.Millisecond {
			combinedScore += 100
		} else if r.AverageLatency < 1*time.Second {
			combinedScore += 50
		}

		if combinedScore > bestScore {
			bestScore = combinedScore
			best = r
		}
	}

	return best
}

func FormatAdvancedResults(results []AdvancedTestResult) string {
	output := "═══════════════════════════════════════════════════════════\n"
	output += "                  PROFILE TEST RESULTS\n"
	output += "═══════════════════════════════════════════════════════════\n\n"

	for _, r := range results {
		output += fmt.Sprintf("Profile: %s\n", r.ProfileName)
		output += fmt.Sprintf("Mode: %s\n", r.Mode)
		output += fmt.Sprintf("Score: %d\n", r.Score)
		output += fmt.Sprintf("Success Rate: %.1f%%\n", r.SuccessRate)
		output += fmt.Sprintf("Average Latency: %dms\n", r.AverageLatency.Milliseconds())
		
		if r.TCPFreezeDetected {
			output += "⚠ TCP FREEZE DETECTED\n"
		}
		
		output += fmt.Sprintf("Recommendation: %s\n", r.Recommendation)
		output += "\nTest Details:\n"
		
		for _, test := range r.Results {
			status := "✗"
			if test.Success {
				status = "✓"
			}
			output += fmt.Sprintf("  %s %s (%dms)\n", status, test.URL, test.Latency.Milliseconds())
			if test.Error != "" {
				output += fmt.Sprintf("    Error: %s\n", test.Error)
			}
		}
		
		output += "\n───────────────────────────────────────────────────────────\n\n"
	}

	best := FindBestProfile(results)
	if best != nil {
		output += "═══════════════════════════════════════════════════════════\n"
		output += fmt.Sprintf("🏆 RECOMMENDED PROFILE: %s\n", best.ProfileName)
		output += fmt.Sprintf("   Score: %d | Success: %.1f%% | Latency: %dms\n", 
			best.Score, best.SuccessRate, best.AverageLatency.Milliseconds())
		output += "═══════════════════════════════════════════════════════════\n"
	}

	return output
}
