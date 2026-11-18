package engine

import (
	"sync"
	"testing"
)

func TestSecureDNSConcurrentAccessAndDefensiveCopy(t *testing.T) {
	defer SetDNSProvider("Cloudflare")

	SetDNSProvider("Google")
	got := GetSecureDNSServers()
	if len(got) != 2 || got[0] != "8.8.8.8" || got[1] != "8.8.4.4" {
		t.Fatalf("GetSecureDNSServers() = %v, want Google resolvers", got)
	}

	// Mutating caller's slice must not mutate global state.
	got[0] = "127.0.0.1"
	fresh := GetSecureDNSServers()
	if fresh[0] != "8.8.8.8" {
		t.Fatalf("GetSecureDNSServers() leaked internal slice: got %v", fresh)
	}

	names := []string{"Cloudflare", "Google", "Quad9", "AdGuard", "Unknown"}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				SetDNSProvider(names[(idx+j)%len(names)])
			}
		}(i)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				servers := GetSecureDNSServers()
				if len(servers) < 2 || servers[0] == "" {
					t.Errorf("invalid servers during concurrent access: %v", servers)
				}
			}
		}()
	}
	wg.Wait()
}
