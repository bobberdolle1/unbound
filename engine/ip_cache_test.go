package engine

import (
	"context"
	"testing"
	"time"
)

func TestNewIPCache(t *testing.T) {
	cache := NewIPCache(5 * time.Minute)

	if cache == nil {
		t.Fatal("Failed to create IP cache")
	}

	if cache.Size() != 0 {
		t.Errorf("New cache should be empty, got size %d", cache.Size())
	}
}

func TestIPCacheResolve(t *testing.T) {
	cache := NewIPCache(5 * time.Minute)
	ctx := context.Background()

	ips, err := cache.Resolve(ctx, "example.com")

	if err != nil {
		t.Logf("Resolve failed (network may be unavailable): %v", err)
		return
	}

	if len(ips) == 0 {
		t.Error("Expected at least one IP address")
	}

	t.Logf("Resolved example.com to %d IPs: %v", len(ips), ips)

	cachedIPs, found := cache.Get("example.com")
	if !found {
		t.Error("Expected cached entry")
	}

	if len(cachedIPs) != len(ips) {
		t.Error("Cached IPs don't match resolved IPs")
	}
}

func TestIPCacheExpiration(t *testing.T) {
	cache := NewIPCache(1 * time.Second)

	cache.Set("test.com", []string{"1.2.3.4"}, 1*time.Second)

	ips, found := cache.Get("test.com")
	if !found {
		t.Error("Expected cached entry")
	}

	if len(ips) != 1 || ips[0] != "1.2.3.4" {
		t.Error("Cached IP mismatch")
	}

	time.Sleep(2 * time.Second)

	_, found = cache.Get("test.com")
	if found {
		t.Error("Expected cache entry to expire")
	}
}

func TestIPCacheSetGet(t *testing.T) {
	cache := NewIPCache(5 * time.Minute)

	testIPs := []string{"1.1.1.1", "8.8.8.8"}
	cache.Set("test.example.com", testIPs, 5*time.Minute)

	ips, found := cache.Get("test.example.com")
	if !found {
		t.Error("Expected cached entry")
	}

	if len(ips) != len(testIPs) {
		t.Errorf("Expected %d IPs, got %d", len(testIPs), len(ips))
	}

	for i, ip := range ips {
		if ip != testIPs[i] {
			t.Errorf("IP mismatch at index %d: expected %s, got %s", i, testIPs[i], ip)
		}
	}
}

func TestIPCacheDelete(t *testing.T) {
	cache := NewIPCache(5 * time.Minute)

	cache.Set("delete.test.com", []string{"1.2.3.4"}, 5*time.Minute)

	_, found := cache.Get("delete.test.com")
	if !found {
		t.Error("Expected cached entry before delete")
	}

	cache.Delete("delete.test.com")

	_, found = cache.Get("delete.test.com")
	if found {
		t.Error("Expected entry to be deleted")
	}
}

func TestIPCacheClear(t *testing.T) {
	cache := NewIPCache(5 * time.Minute)

	cache.Set("host1.com", []string{"1.1.1.1"}, 5*time.Minute)
	cache.Set("host2.com", []string{"2.2.2.2"}, 5*time.Minute)

	if cache.Size() != 2 {
		t.Errorf("Expected size 2, got %d", cache.Size())
	}

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", cache.Size())
	}
}

func TestIPCachePreloadHostnames(t *testing.T) {
	cache := NewIPCache(5 * time.Minute)
	ctx := context.Background()

	hostnames := []string{"example.com", "google.com"}

	err := cache.PreloadHostnames(ctx, hostnames)
	if err != nil {
		t.Logf("Preload failed (network may be unavailable): %v", err)
		return
	}

	for _, hostname := range hostnames {
		_, found := cache.Get(hostname)
		if !found {
			t.Errorf("Expected %s to be preloaded", hostname)
		}
	}

	t.Logf("Preloaded %d hostnames", len(hostnames))
}

func TestGetGlobalIPCache(t *testing.T) {
	cache := GetGlobalIPCache()

	if cache == nil {
		t.Fatal("Global IP cache is nil")
	}

	cache.Set("global.test.com", []string{"1.2.3.4"}, 5*time.Minute)

	cache2 := GetGlobalIPCache()

	ips, found := cache2.Get("global.test.com")
	if !found {
		t.Error("Expected entry in global cache")
	}

	if len(ips) != 1 || ips[0] != "1.2.3.4" {
		t.Error("Global cache data mismatch")
	}
}

func TestResolveWithCache(t *testing.T) {
	ctx := context.Background()

	ips, err := ResolveWithCache(ctx, "example.com")
	if err != nil {
		t.Logf("Resolve failed (network may be unavailable): %v", err)
		return
	}

	if len(ips) == 0 {
		t.Error("Expected at least one IP")
	}

	t.Logf("Resolved example.com: %v", ips)
}
