package engine

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"unbound/engine/providers"
)

func TestAutoTuneV2WithMockDPI(t *testing.T) {
	mockDPIServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer mockDPIServer.Close()

	mockBlockedServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		panic("simulated DPI drop")
	}))
	defer mockBlockedServer.Close()

	testTargets = []Target{{Name:"mock1", URL:mockDPIServer.URL, Priority:10}, {Name:"mock2", URL:mockBlockedServer.URL, Priority:10}}

	assets, err := ExtractAssets()
	if err != nil {
		t.Skipf("Skipping test: assets not available: %v", err)
	}

	provider := providers.NewZapret2WindowsProvider(
		assets.BinDir,
		assets.LuaDir,
		assets.ListDir,
		false,
		false,
	)

	profiles := []Profile{
		{Name: "Test Profile 1", Args: []string{"--filter-tcp=443"}},
		{Name: "Test Profile 2", Args: []string{"--filter-tcp=80,443"}},
	}

	for _, prof := range profiles {
		provider.RegisterProfile(prof.Name, prof.Args)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan bool)
	var result *AutoTuneResult
	var testErr error

	go func() {
		result, testErr = RunAutoTuneV2WithContext(context.Background(), provider, profiles)
		done <- true
	}()

	select {
	case <-done:
		if testErr != nil && ctx.Err() == nil {
			t.Logf("Auto-tune completed with error (expected): %v", testErr)
		}
		if result != nil {
			t.Logf("Result: %+v", result)
		}
	case <-ctx.Done():
		t.Fatal("Auto-tune test timed out - possible deadlock")
	}
}

func TestAutoTuneV2Timeout(t *testing.T) {
	slowServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	testTargets = []Target{{Name: "slow", URL: slowServer.URL, Priority: 10}}

	assets, err := ExtractAssets()
	if err != nil {
		t.Skipf("Skipping test: assets not available: %v", err)
	}

	provider := providers.NewZapret2WindowsProvider(
		assets.BinDir,
		assets.LuaDir,
		assets.ListDir,
		false,
		false,
	)

	profiles := []Profile{
		{Name: "Timeout Test", Args: []string{"--filter-tcp=443"}},
	}

	for _, prof := range profiles {
		provider.RegisterProfile(prof.Name, prof.Args)
	}

	start := time.Now()
	result, err := RunAutoTuneV2WithContext(context.Background(), provider, profiles)
	elapsed := time.Since(start)

	if elapsed > 20*time.Second {
		t.Errorf("Auto-tune took too long: %v", elapsed)
	}

	if err == nil && result == nil {
		t.Error("Expected error or result, got neither")
	}

	t.Logf("Test completed in %v", elapsed)
}

func TestTestBypassWithMockServers(t *testing.T) {
	goodServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	}))
	defer goodServer.Close()

	badServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
	}))
	defer badServer.Close()

	originalTargets := testTargets
	testTargets = []Target{
		{Name: "good", URL: goodServer.URL, Priority: 10},
		{Name: "bad", URL: badServer.URL, Priority: 10},
	}
	defer func() { testTargets = originalTargets }()

	result := testBypassParallel("Mock Profile")

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if result.ProfileName != "Mock Profile" {
		t.Errorf("Expected profile name 'Mock Profile', got '%s'", result.ProfileName)
	}

	successCount := 0
	for _, status := range result.Results {
		if status.OK {
			successCount++
		}
	}

	if successCount == 0 {
		t.Error("Expected at least one successful test")
	}

	t.Logf("Success rate: %d/%d", successCount, len(result.Results))
}

func TestHTTPClientConfiguration(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("User-Agent header missing")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}
