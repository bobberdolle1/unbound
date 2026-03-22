package engine

import (
	"context"
	"net"
	"sync"
	"time"
)

type IPCacheEntry struct {
	Hostname   string
	IPs        []string
	ResolvedAt time.Time
	TTL        time.Duration
}

type IPCache struct {
	mu      sync.RWMutex
	entries map[string]*IPCacheEntry
	ttl     time.Duration
}

func NewIPCache(defaultTTL time.Duration) *IPCache {
	cache := &IPCache{
		entries: make(map[string]*IPCacheEntry),
		ttl:     defaultTTL,
	}
	
	go cache.cleanupLoop()
	
	return cache
}

func (c *IPCache) Resolve(ctx context.Context, hostname string) ([]string, error) {
	c.mu.RLock()
	entry, exists := c.entries[hostname]
	c.mu.RUnlock()
	
	if exists && time.Since(entry.ResolvedAt) < entry.TTL {
		return entry.IPs, nil
	}
	
	ips, err := net.DefaultResolver.LookupHost(ctx, hostname)
	if err != nil {
		return nil, err
	}
	
	c.mu.Lock()
	c.entries[hostname] = &IPCacheEntry{
		Hostname:   hostname,
		IPs:        ips,
		ResolvedAt: time.Now(),
		TTL:        c.ttl,
	}
	c.mu.Unlock()
	
	return ips, nil
}

func (c *IPCache) Get(hostname string) ([]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.entries[hostname]
	if !exists {
		return nil, false
	}
	
	if time.Since(entry.ResolvedAt) >= entry.TTL {
		return nil, false
	}
	
	return entry.IPs, true
}

func (c *IPCache) Set(hostname string, ips []string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.entries[hostname] = &IPCacheEntry{
		Hostname:   hostname,
		IPs:        ips,
		ResolvedAt: time.Now(),
		TTL:        ttl,
	}
}

func (c *IPCache) Delete(hostname string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.entries, hostname)
}

func (c *IPCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.entries = make(map[string]*IPCacheEntry)
}

func (c *IPCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return len(c.entries)
}

func (c *IPCache) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		c.cleanup()
	}
}

func (c *IPCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	for hostname, entry := range c.entries {
		if now.Sub(entry.ResolvedAt) >= entry.TTL {
			delete(c.entries, hostname)
		}
	}
}

func (c *IPCache) GetAll() map[string]*IPCacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	result := make(map[string]*IPCacheEntry, len(c.entries))
	for k, v := range c.entries {
		result[k] = v
	}
	
	return result
}

func (c *IPCache) PreloadHostnames(ctx context.Context, hostnames []string) error {
	for _, hostname := range hostnames {
		_, err := c.Resolve(ctx, hostname)
		if err != nil {
			return err
		}
	}
	return nil
}

var globalIPCache = NewIPCache(10 * time.Minute)

func GetGlobalIPCache() *IPCache {
	return globalIPCache
}

func ResolveWithCache(ctx context.Context, hostname string) ([]string, error) {
	return globalIPCache.Resolve(ctx, hostname)
}

func PreloadCommonHosts(ctx context.Context) error {
	commonHosts := []string{
		"discord.com",
		"youtube.com",
		"googlevideo.com",
		"web.telegram.org",
		"cloudflare.com",
	}
	
	return globalIPCache.PreloadHostnames(ctx, commonHosts)
}
