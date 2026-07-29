//go:build windows

package engine

import (
	"context"
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

	provider := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, assets.ListDir, assets.EngineSHA256, false, true)
	RegisterWindowsProfileCatalog(provider, assets.LuaDir)

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
			name:        "Discord with Recommended",
			profile:     "Recommended (hostfakesplit)",
			targetURL:   "https://discord.com",
			expectOK:    true,
			description: "Discord should establish verified TLS",
		},
		{
			name:        "GoogleVideo with Universal",
			profile:     "Universal 2026 (All-in-One)",
			targetURL:   "https://googlevideo.com",
			expectOK:    true,
			description: "GoogleVideo CDN should establish verified TLS",
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

			probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			result, probeErr := ProbeConnection(probeCtx, tc.targetURL)
			cancel()
			if probeErr != nil {
				if tc.expectOK {
					t.Errorf("Expected verified TLS success but got error: %v", probeErr)
					t.Logf("Provider logs:\n%v", provider.GetLogs())
				}
			} else if tc.expectOK && (!result.Success || !result.CertValid) {
				t.Errorf("Expected a verified TLS connection, got %+v", result)
			} else {
				t.Logf("✓ %s: SUCCESS (%dms, issuer=%s)", tc.description, result.Latency.Milliseconds(), result.CertIssuer)
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
