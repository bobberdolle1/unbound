package tester

import (
	"context"
	"testing"
	"time"
)

func TestAdvancedTestConfig(t *testing.T) {
	config := AdvancedTestConfig{
		Mode:            TestModeStandard,
		MaxConcurrent:   4,
		Timeout:         10 * time.Second,
		URLs:            []string{"https://example.com"},
		CheckTCPFreeze:  true,
		MinDownloadSize: 16 * 1024,
	}
	
	if config.Mode != TestModeStandard {
		t.Errorf("Expected mode %s, got %s", TestModeStandard, config.Mode)
	}
	
	if config.MaxConcurrent != 4 {
		t.Errorf("Expected MaxConcurrent 4, got %d", config.MaxConcurrent)
	}
}

func TestDetectTCPFreeze(t *testing.T) {
	testCases := []struct {
		name     string
		results  []TestResult
		expected bool
	}{
		{
			name: "No freeze",
			results: []TestResult{
				{URL: "test1", Success: true, Latency: 500 * time.Millisecond},
				{URL: "test2", Success: true, Latency: 1 * time.Second},
			},
			expected: false,
		},
		{
			name: "Freeze detected by latency",
			results: []TestResult{
				{URL: "test1", Success: false, Latency: 16 * time.Second},
			},
			expected: true,
		},
		{
			name: "Freeze detected by error and latency",
			results: []TestResult{
				{URL: "test1", Success: false, Latency: 11 * time.Second, Error: "timeout"},
			},
			expected: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := detectTCPFreeze(tc.results)
			
			if result != tc.expected {
				t.Errorf("Expected freeze detection %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestCalculateAverageLatency(t *testing.T) {
	testCases := []struct {
		name     string
		results  []TestResult
		expected time.Duration
	}{
		{
			name:     "Empty results",
			results:  []TestResult{},
			expected: 0,
		},
		{
			name: "Single success",
			results: []TestResult{
				{URL: "test1", Success: true, Latency: 500 * time.Millisecond},
			},
			expected: 500 * time.Millisecond,
		},
		{
			name: "Multiple successes",
			results: []TestResult{
				{URL: "test1", Success: true, Latency: 400 * time.Millisecond},
				{URL: "test2", Success: true, Latency: 600 * time.Millisecond},
			},
			expected: 500 * time.Millisecond,
		},
		{
			name: "Mixed success and failure",
			results: []TestResult{
				{URL: "test1", Success: true, Latency: 500 * time.Millisecond},
				{URL: "test2", Success: false, Latency: 10 * time.Second},
			},
			expected: 500 * time.Millisecond,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := calculateAverageLatency(tc.results)
			
			if result != tc.expected {
				t.Errorf("Expected average latency %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestCalculateSuccessRate(t *testing.T) {
	testCases := []struct {
		name     string
		results  []TestResult
		expected float64
	}{
		{
			name:     "Empty results",
			results:  []TestResult{},
			expected: 0,
		},
		{
			name: "All success",
			results: []TestResult{
				{URL: "test1", Success: true},
				{URL: "test2", Success: true},
			},
			expected: 100.0,
		},
		{
			name: "All failure",
			results: []TestResult{
				{URL: "test1", Success: false},
				{URL: "test2", Success: false},
			},
			expected: 0.0,
		},
		{
			name: "50% success",
			results: []TestResult{
				{URL: "test1", Success: true},
				{URL: "test2", Success: false},
			},
			expected: 50.0,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := calculateSuccessRate(tc.results)
			
			if result != tc.expected {
				t.Errorf("Expected success rate %.1f%%, got %.1f%%", tc.expected, result)
			}
		})
	}
}

func TestGenerateRecommendation(t *testing.T) {
	testCases := []struct {
		name     string
		result   AdvancedTestResult
		contains string
	}{
		{
			name: "TCP freeze detected",
			result: AdvancedTestResult{
				TCPFreezeDetected: true,
			},
			contains: "NOT RECOMMENDED",
		},
		{
			name: "Perfect score low latency",
			result: AdvancedTestResult{
				SuccessRate:    100,
				AverageLatency: 300 * time.Millisecond,
			},
			contains: "EXCELLENT",
		},
		{
			name: "Perfect score high latency",
			result: AdvancedTestResult{
				SuccessRate:    100,
				AverageLatency: 3 * time.Second,
			},
			contains: "ACCEPTABLE",
		},
		{
			name: "Moderate success",
			result: AdvancedTestResult{
				SuccessRate: 80,
			},
			contains: "MODERATE",
		},
		{
			name: "Poor success",
			result: AdvancedTestResult{
				SuccessRate: 60,
			},
			contains: "POOR",
		},
		{
			name: "Failed",
			result: AdvancedTestResult{
				SuccessRate: 30,
			},
			contains: "FAILED",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recommendation := generateRecommendation(tc.result)
			
			if recommendation == "" {
				t.Error("Expected non-empty recommendation")
			}
			
			t.Logf("Recommendation: %s", recommendation)
		})
	}
}

func TestFindBestProfile(t *testing.T) {
	results := []AdvancedTestResult{
		{
			ProfileName:       "Profile A",
			Score:             300,
			SuccessRate:       100,
			AverageLatency:    400 * time.Millisecond,
			TCPFreezeDetected: false,
		},
		{
			ProfileName:       "Profile B",
			Score:             500,
			SuccessRate:       100,
			AverageLatency:    200 * time.Millisecond,
			TCPFreezeDetected: false,
		},
		{
			ProfileName:       "Profile C",
			Score:             600,
			SuccessRate:       80,
			AverageLatency:    100 * time.Millisecond,
			TCPFreezeDetected: false,
		},
		{
			ProfileName:       "Profile D",
			Score:             700,
			SuccessRate:       100,
			AverageLatency:    5 * time.Second,
			TCPFreezeDetected: true,
		},
	}
	
	best := FindBestProfile(results)
	
	if best == nil {
		t.Fatal("Expected best profile, got nil")
	}
	
	if best.ProfileName != "Profile B" {
		t.Errorf("Expected Profile B as best, got %s", best.ProfileName)
	}
	
	t.Logf("Best profile: %s (Score: %d, Success: %.1f%%, Latency: %dms)", 
		best.ProfileName, best.Score, best.SuccessRate, best.AverageLatency.Milliseconds())
}

func TestTestSingleProfile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	config := AdvancedTestConfig{
		Mode:            TestModeStandard,
		MaxConcurrent:   1,
		Timeout:         3 * time.Second,
		URLs:            []string{"https://example.com"},
		CheckTCPFreeze:  false,
		MinDownloadSize: 0,
	}
	
	result := testSingleProfile(ctx, "TestProfile", config)
	
	if result.ProfileName != "TestProfile" {
		t.Errorf("Expected profile name 'TestProfile', got '%s'", result.ProfileName)
	}
	
	if result.Mode != TestModeStandard {
		t.Errorf("Expected mode %s, got %s", TestModeStandard, result.Mode)
	}
	
	if len(result.Results) == 0 {
		t.Error("Expected test results, got none")
	}
	
	t.Logf("Profile: %s, Score: %d, Success Rate: %.1f%%", 
		result.ProfileName, result.Score, result.SuccessRate)
}
