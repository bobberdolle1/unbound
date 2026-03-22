//go:build windows && !short
// +build windows,!short

package tests

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unbound/engine"
	"unbound/engine/providers"
)

type TestResult struct {
	ProfileName   string
	TCPStatus     string
	UDPStatus     string
	CleanupStatus string
}

var testProfiles = []string{
	"YouTube + Discord (ТСПУ Optimized)",
	"YouTube Only",
	"Discord Only",
}

func TestE2EBypassMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E matrix test in short mode")
	}

	binPath, luaDir, err := setupTestEnvironment()
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	provider := providers.NewZapret2WindowsProvider(binPath, luaDir, "", true, false)

	profiles := engine.GetProfiles(luaDir)
	for _, profile := range profiles {
		provider.RegisterProfile(profile.Name, profile.Args)
	}

	results := make([]TestResult, 0, len(testProfiles))

	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("🧪 E2E BYPASS MATRIX TEST")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for _, profileName := range testProfiles {
		t.Logf("\n🔍 Testing Profile: %s", profileName)

		result := TestResult{
			ProfileName: profileName,
		}

		for cycle := 1; cycle <= 3; cycle++ {
			t.Logf("  Cycle %d/3: Starting engine...", cycle)

			ctx := context.Background()
			if err := provider.Start(ctx, profileName); err != nil {
				result.TCPStatus = fmt.Sprintf("FAIL (start error: %v)", err)
				result.UDPStatus = "SKIP"
				result.CleanupStatus = "SKIP"
				break
			}

			if !provider.WaitReady(5 * time.Second) {
				t.Logf("  ⚠️  Engine initialization timeout")
				
				logs := provider.GetLogs()
				if len(logs) > 0 {
					t.Logf("  📋 Engine logs (last 10 lines):")
					start := len(logs) - 10
					if start < 0 {
						start = 0
					}
					for _, log := range logs[start:] {
						t.Logf("      %s", log)
					}
				}
				
				provider.Stop()
				result.TCPStatus = "FAIL (timeout)"
				result.UDPStatus = "SKIP"
				result.CleanupStatus = "SKIP"
				break
			}

			time.Sleep(2 * time.Second)

			if cycle == 1 {
				tcpOK := testTCPBypass(t, profileName)
				if tcpOK {
					result.TCPStatus = "✅ PASS"
				} else {
					result.TCPStatus = "❌ FAIL"
				}

				udpOK := testUDPBypass(t, profileName)
				if udpOK {
					result.UDPStatus = "✅ PASS"
				} else {
					result.UDPStatus = "❌ FAIL"
				}
			}

			t.Logf("  Cycle %d/3: Stopping engine...", cycle)
			if err := provider.Stop(); err != nil {
				result.CleanupStatus = fmt.Sprintf("❌ FAIL (stop error: %v)", err)
				break
			}

			time.Sleep(1 * time.Second)

			if !verifyCleanShutdown(t) {
				result.CleanupStatus = "❌ FAIL (zombie process detected)"
				break
			}

			if cycle == 3 {
				result.CleanupStatus = "✅ PASS (3 cycles)"
			}
		}

		results = append(results, result)
	}

	printResultsTable(t, results)

	for _, result := range results {
		if strings.Contains(result.TCPStatus, "FAIL") ||
			strings.Contains(result.UDPStatus, "FAIL") ||
			strings.Contains(result.CleanupStatus, "FAIL") {
			t.Errorf("Profile '%s' failed one or more checks", result.ProfileName)
		}
	}
}

func testTCPBypass(t *testing.T, profileName string) bool {
	t.Log("    [TCP] Testing HTTPS connectivity...")

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 0,
			}).DialContext,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
			DisableKeepAlives:     true,
			MaxIdleConns:          1,
			IdleConnTimeout:       1 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	testURLs := []string{
		"https://discord.com/api/v9/gateway",
		"https://www.youtube.com",
	}

	successCount := 0
	for _, url := range testURLs {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Logf("    [TCP] ❌ Failed to create request for %s: %v", url, err)
			continue
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("    [TCP] ❌ Request failed for %s: %v", url, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 500 {
			t.Logf("    [TCP] ✅ %s → HTTP %d", url, resp.StatusCode)
			successCount++
		} else {
			t.Logf("    [TCP] ❌ %s → HTTP %d", url, resp.StatusCode)
		}
	}

	return successCount == len(testURLs)
}

func testUDPBypass(t *testing.T, profileName string) bool {
	t.Log("    [UDP] Testing UDP connectivity...")

	dnsServers := []string{
		"8.8.8.8:53",
		"1.1.1.1:53",
	}

	successCount := 0
	for _, server := range dnsServers {
		conn, err := net.DialTimeout("udp", server, 3*time.Second)
		if err != nil {
			t.Logf("    [UDP] ❌ Failed to connect to %s: %v", server, err)
			continue
		}
		defer conn.Close()

		conn.SetDeadline(time.Now().Add(3 * time.Second))

		dnsQuery := []byte{
			0x00, 0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x06, 0x67, 0x6f, 0x6f,
			0x67, 0x6c, 0x65, 0x03, 0x63, 0x6f, 0x6d, 0x00,
			0x00, 0x01, 0x00, 0x01,
		}

		_, err = conn.Write(dnsQuery)
		if err != nil {
			t.Logf("    [UDP] ❌ Failed to send DNS query to %s: %v", server, err)
			continue
		}

		response := make([]byte, 512)
		n, err := conn.Read(response)
		if err != nil {
			t.Logf("    [UDP] ❌ Failed to receive DNS response from %s: %v", server, err)
			continue
		}

		if n > 12 {
			t.Logf("    [UDP] ✅ %s → Response received (%d bytes)", server, n)
			successCount++
		} else {
			t.Logf("    [UDP] ❌ %s → Invalid response size (%d bytes)", server, n)
		}
	}

	return successCount >= 1
}

func verifyCleanShutdown(t *testing.T) bool {
	t.Log("    [CLEANUP] Verifying clean shutdown...")

	time.Sleep(500 * time.Millisecond)

	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq winws2.exe")
	output, err := cmd.Output()
	if err != nil {
		t.Logf("    [CLEANUP] ⚠️  Failed to check process list: %v", err)
		return true
	}

	if strings.Contains(string(output), "winws2.exe") {
		t.Log("    [CLEANUP] ❌ Zombie process detected: winws2.exe still running")
		exec.Command("taskkill", "/F", "/IM", "winws2.exe").Run()
		return false
	}

	t.Log("    [CLEANUP] ✅ Clean shutdown verified")
	return true
}

func printResultsTable(t *testing.T, results []TestResult) {
	t.Log("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("📊 TEST RESULTS MATRIX")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("%-40s | %-12s | %-12s | %-20s", "Profile", "TCP Status", "UDP Status", "Cleanup Status")
	t.Log("────────────────────────────────────────────────────────────────────────────────────────")

	for _, result := range results {
		t.Logf("%-40s | %-12s | %-12s | %-20s",
			truncate(result.ProfileName, 40),
			result.TCPStatus,
			result.UDPStatus,
			result.CleanupStatus)
	}

	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func setupTestEnvironment() (string, string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	
	if strings.HasSuffix(wd, "tests") {
		wd = filepath.Dir(wd)
	}
	
	binPath := filepath.Join(wd, "engine", "core_bin", "windows")
	luaDir := filepath.Join(wd, "engine", "lua_scripts")
	
	if _, err := os.Stat(filepath.Join(binPath, "winws2.exe")); os.IsNotExist(err) {
		return "", "", fmt.Errorf("winws2.exe not found at %s", binPath)
	}
	
	if _, err := os.Stat(filepath.Join(luaDir, "zapret-lib.lua")); os.IsNotExist(err) {
		return "", "", fmt.Errorf("zapret-lib.lua not found at %s", luaDir)
	}
	
	return binPath, luaDir, nil
}
