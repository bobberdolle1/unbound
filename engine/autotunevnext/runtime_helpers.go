package autotunevnext

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unbound/engine"
	"unbound/engine/providers"
)

type providerSnapshot struct {
	status  providers.Status
	profile string
}

// candidateExecutionContext preserves the caller's deadline. Teardown has a
// separate bounded timeout, so a valid experiment is never shortened here.
func candidateExecutionContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(ctx)
}

func unsupported(code string) HostPreflightResult {
	return HostPreflightResult{Status: PreflightUnsupported, Reasons: []Reason{{Code: code}}}
}

func errored(code string, err error) HostPreflightResult {
	return HostPreflightResult{Status: PreflightError, Reasons: []Reason{{Code: code, Detail: err.Error()}}}
}

func verifyRuntimeBinary(path, expected string) (string, error) {
	if expected == "" {
		return "", fmt.Errorf("missing pinned engine identity")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	if !strings.EqualFold(actual, expected) {
		return "", fmt.Errorf("pinned engine SHA-256 mismatch")
	}
	return actual, nil
}

func trustedLuaInitArgs(luaDir string) []string {
	names := []string{"zapret-lib.lua", "zapret-antidpi.lua", "init_vars.lua", "custom_funcs.lua"}
	args := make([]string, 0, len(names))
	for _, name := range names {
		args = append(args, "--lua-init=@"+filepath.ToSlash(filepath.Join(luaDir, name)))
	}
	return args
}

func runtimeTimestampsActive() bool {
	for _, diagnostic := range engine.RunDiagnostics() {
		if diagnostic.Component == "TCP Stack" || diagnostic.Component == "TCP timestamps" {
			return diagnostic.Status == "OK"
		}
	}
	return false
}

func shortFingerprint(fingerprint string) string {
	if len(fingerprint) > 12 {
		return fingerprint[:12]
	}
	return fingerprint
}
