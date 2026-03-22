package engine

import (
	"context"
	"crypto/tls"
	"net/http"
	"testing"
	"time"

	"unbound/engine/providers"
)

func TestBypassRouting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	assets, err := ExtractAssets()
	if err != nil {
		t.Fatalf("Failed to extract assets: %v", err)
	}

	provider := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, assets.ListDir, false)

	hasPriv, err := provider.CheckPrivileges()
	if err != nil {
		t.Fatalf("Failed to check privileges: %v", err)
	}
	if !hasPriv {
		t.Skip("Test requires administrator privileges")
	}

	testCases := []struct {
		name        string
		profile     string
		targetURL   string
		expectOK    bool
		description string
	}{
		{
			name:        "Telegram Web with MTProto",
			profile:     "Telegram MTProto",
			targetURL:   "https://web.telegram.org",
			expectOK:    true,
			description: "Telegram should be accessible with any-protocol desync",
		},
		{
			name:        "GoogleVideo with Ultimate Combo",
			profile:     "The Ultimate Combo",
			targetURL:   "https://googlevideo.com",
			expectOK:    true,
			description: "GoogleVideo CDN should be accessible",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			t.Logf("Starting engine with profile: %s", tc.profile)
			err := provider.Start(ctx, tc.profile)
			if err != nil {
				t.Fatalf("Failed to start provider with profile %s: %v", tc.profile, err)
			}

			time.Sleep(3 * time.Second)

			if provider.GetStatus() != providers.StatusRunning {
				logs := provider.GetLogs()
				t.Fatalf("Provider not running after start. Status: %v\nLogs:\n%v", provider.GetStatus(), logs)
			}

			t.Logf("Testing connectivity to: %s", tc.targetURL)
			
			client := &http.Client{
				Timeout: 15 * time.Second,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}

			resp, err := client.Get(tc.targetURL)
			if err != nil {
				if tc.expectOK {
					t.Errorf("Expected success but got error: %v", err)
					t.Logf("Provider logs:\n%v", provider.GetLogs())
				}
			} else {
				defer resp.Body.Close()
				t.Logf("HTTP Status: %d", resp.StatusCode)
				
				if tc.expectOK && (resp.StatusCode < 200 || resp.StatusCode >= 500) {
					t.Errorf("Expected 2xx/3xx/4xx status but got %d", resp.StatusCode)
				}
				
				if resp.StatusCode >= 200 && resp.StatusCode < 500 {
					t.Logf("✓ %s: SUCCESS (HTTP %d)", tc.description, resp.StatusCode)
				}
			}

			err = provider.Stop()
			if err != nil {
				t.Errorf("Failed to stop provider: %v", err)
			}

			time.Sleep(1 * time.Second)
		})
	}
}

func TestHostlistAndIPsetExtraction(t *testing.T) {
	assets, err := ExtractAssets()
	if err != nil {
		t.Fatalf("Failed to extract assets: %v", err)
	}

	if assets.ListDir == "" {
		t.Fatal("ListDir is empty")
	}

	t.Logf("ListDir: %s", assets.ListDir)
}
