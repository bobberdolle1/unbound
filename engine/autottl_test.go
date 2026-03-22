package engine

import (
	"context"
	"testing"
	"time"
)

func TestDetectDPIDistance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	testHosts := []string{
		"discord.com",
		"youtube.com",
	}
	
	for _, host := range testHosts {
		distance, err := DetectDPIDistance(ctx, host)
		
		if err != nil {
			t.Logf("Host %s: Could not detect DPI distance: %v", host, err)
			continue
		}
		
		if distance < 1 || distance > 30 {
			t.Errorf("Host %s: Invalid DPI distance %d (expected 1-30)", host, distance)
		}
		
		t.Logf("Host %s: DPI distance detected at %d hops", host, distance)
	}
}

func TestAutoTTLForProfile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	targets := []string{
		"https://discord.com",
		"https://youtube.com",
	}
	
	ttlMap := AutoTTLForProfile(ctx, targets)
	
	if len(ttlMap) == 0 {
		t.Log("No TTL values detected (network may be unavailable)")
		return
	}
	
	for host, ttl := range ttlMap {
		if ttl < 1 || ttl > 30 {
			t.Errorf("Host %s: Invalid TTL %d", host, ttl)
		}
		t.Logf("Host %s: TTL = %d", host, ttl)
	}
}

func TestGetOptimalTTL(t *testing.T) {
	testCases := []struct {
		name     string
		ttlMap   map[string]int
		expected int
	}{
		{
			name:     "Empty map",
			ttlMap:   map[string]int{},
			expected: 4,
		},
		{
			name: "Single value",
			ttlMap: map[string]int{
				"host1": 5,
			},
			expected: 5,
		},
		{
			name: "Multiple values average",
			ttlMap: map[string]int{
				"host1": 4,
				"host2": 6,
			},
			expected: 5,
		},
		{
			name: "Low values clamped",
			ttlMap: map[string]int{
				"host1": 1,
				"host2": 2,
			},
			expected: 3,
		},
		{
			name: "High values clamped",
			ttlMap: map[string]int{
				"host1": 10,
				"host2": 12,
			},
			expected: 8,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GetOptimalTTL(tc.ttlMap)
			
			if result != tc.expected {
				t.Errorf("Expected optimal TTL %d, got %d", tc.expected, result)
			}
		})
	}
}

func TestProbeTTL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	result := probeTTL(ctx, "discord.com", 10)
	
	if result.TTL != 10 {
		t.Errorf("Expected TTL 10, got %d", result.TTL)
	}
	
	if result.Success {
		t.Logf("Probe successful: latency %dms", result.Latency.Milliseconds())
	} else {
		t.Logf("Probe failed (expected for TTL testing)")
	}
}
