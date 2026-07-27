package tester

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type TestResult struct {
	URL        string
	Success    bool
	Latency    time.Duration
	Error      string
	StatusCode int
}

type ProfileScore struct {
	ProfileName string
	Score       int
	Results     []TestResult
}

var TestURLs = []string{
	"https://www.youtube.com",
	"https://discord.com",
	"https://web.telegram.org",
	"https://www.google.com",
}

func TestProfile(ctx context.Context, urls []string, timeout time.Duration) []TestResult {
	if len(urls) == 0 {
		urls = TestURLs
	}

	results := make([]TestResult, len(urls))
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
			DialContext: (&net.Dialer{
				Timeout:   timeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: timeout,
		},
	}

	var wg sync.WaitGroup
	for i, url := range urls {
		wg.Add(1)
		go func(idx int, targetURL string) {
			defer wg.Done()
			res := TestResult{URL: targetURL}
			start := time.Now()

			req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
			if err != nil {
				res.Error = err.Error()
				results[idx] = res
				return
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

			resp, err := client.Do(req)
			res.Latency = time.Since(start)

			if err != nil {
				res.Error = err.Error()
				results[idx] = res
				return
			}
			defer resp.Body.Close()

			res.StatusCode = resp.StatusCode
			res.Success = resp.StatusCode >= 200 && resp.StatusCode < 400
			results[idx] = res
		}(i, url)
	}
	wg.Wait()

	return results
}

func CalculateScore(results []TestResult) int {
	score := 0
	for _, r := range results {
		if r.Success {
			// YouTube and Discord carry extra weight for DPI bypass
			weight := 100
			if strings.Contains(r.URL, "youtube") || strings.Contains(r.URL, "discord") {
				weight = 150
			}
			score += weight

			if r.Latency < 500*time.Millisecond {
				score += 60
			} else if r.Latency < 1500*time.Millisecond {
				score += 35
			} else if r.Latency < 3*time.Second {
				score += 15
			}
		}
	}
	return score
}

func FormatResults(results []TestResult) string {
	output := ""
	for _, r := range results {
		status := "✗ FAIL"
		if r.Success {
			status = "✓ OK"
		}
		output += fmt.Sprintf("%s %s (%dms)\n", status, r.URL, r.Latency.Milliseconds())
		if r.Error != "" {
			output += fmt.Sprintf("  Error: %s\n", r.Error)
		}
	}
	return output
}
