package providers

import (
	"os"
	"strings"
	"testing"
)

// TestMacOSProviderSourceHasNoSudoersOrPfctlEscalation runs on every OS to
// ensure orphaned passwordless-root sudoers/pfctl code never re-enters
// zapret_macos.go.
func TestMacOSProviderSourceHasNoSudoersOrPfctlEscalation(t *testing.T) {
	data, err := os.ReadFile("zapret_macos.go")
	if err != nil {
		t.Fatalf("read zapret_macos.go: %v", err)
	}
	content := string(data)
	forbidden := []string{
		"NOPASSWD",
		"sudoers",
		"/etc/sudoers",
		"EnsureSudoersConfigured",
		"loadPfAnchor",
		"runPfctlPrivileged",
		"pfctl",
		"/etc/pf.conf",
	}
	for _, needle := range forbidden {
		if strings.Contains(content, needle) {
			t.Errorf("zapret_macos.go contains forbidden privilege-escalation/pf token %q", needle)
		}
	}
}
