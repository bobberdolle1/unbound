package engine

import (
	"context"
	"fmt"
	"net"
	"time"
)

type TTLProbeResult struct {
	TTL         int
	Success     bool
	Latency     time.Duration
	DPIDistance int
}

func DetectDPIDistance(ctx context.Context, targetHost string) (int, error) {
	maxTTL := 30
	minTTL := 1

	bestTTL := -1

	for minTTL <= maxTTL {
		midTTL := minTTL + (maxTTL-minTTL)/2
		result := probeTTL(ctx, targetHost, midTTL)

		if result.Success {
			bestTTL = midTTL
			maxTTL = midTTL - 1 // Try to find a smaller TTL that succeeds
		} else {
			minTTL = midTTL + 1 // Increase TTL
		}
	}

	if bestTTL != -1 {
		if bestTTL > 1 {
			return bestTTL - 1, nil
		}
		return 1, nil
	}

	return 0, fmt.Errorf("could not detect DPI distance for %s", targetHost)
}

func probeTTL(ctx context.Context, targetHost string, ttl int) TTLProbeResult {
	result := TTLProbeResult{
		TTL:     ttl,
		Success: false,
	}

	start := time.Now()

	dialer := &net.Dialer{
		Timeout: 3 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", targetHost+":443")
	if err != nil {
		result.Latency = time.Since(start)
		return result
	}
	defer conn.Close()

	result.Success = true
	result.Latency = time.Since(start)

	return result
}

func AutoTTLForProfile(ctx context.Context, targets []string) map[string]int {
	ttlMap := make(map[string]int)
	cache := GetGlobalIPCache()

	for _, target := range targets {
		host := extractHost(target)
		if host == "" {
			continue
		}

		cache.Resolve(ctx, host)

		distance, err := DetectDPIDistance(ctx, host)
		if err == nil {
			ttlMap[host] = distance
		}
	}

	return ttlMap
}

func GetOptimalTTL(ttlMap map[string]int) int {
	if len(ttlMap) == 0 {
		return 4
	}

	sum := 0
	count := 0

	for _, ttl := range ttlMap {
		sum += ttl
		count++
	}

	avg := sum / count

	if avg < 3 {
		return 3
	}
	if avg > 8 {
		return 8
	}

	return avg
}
