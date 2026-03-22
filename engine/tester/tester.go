package tester

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
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

	results := make([]TestResult, 0, len(urls))
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

	for _, url := range urls {
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
			results = append(results, result)
			continue
		}
		defer resp.Body.Close()

		result.StatusCode = resp.StatusCode
		result.Success = resp.StatusCode >= 200 && resp.StatusCode < 400
		results = append(results, result)
	}

	return results
}

func CalculateScore(results []TestResult) int {
	score := 0
	for _, r := range results {
		if r.Success {
			score += 100
			if r.Latency < 1*time.Second {
				score += 50
			} else if r.Latency < 3*time.Second {
				score += 25
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
