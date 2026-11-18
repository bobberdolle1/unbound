//go:build windows

package engine

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDefenderExclusionScriptEscapesSingleQuotesAndRejectsClearflow(t *testing.T) {
	paths := []string{
		`C:\ProgramData\Unbound`,
		`C:\Users\O'Connor$(calc)\AppData\Local\Unbound`,
		`c:\programdata\unbound`,
	}
	script := buildDefenderExclusionScript(paths)

	if strings.Contains(strings.ToLower(script), "clearflow") {
		t.Fatalf("script still references stale clearflow directory: %s", script)
	}
	wantEscaped := `'C:\Users\O''Connor$(calc)\AppData\Local\Unbound'`
	if !strings.Contains(script, wantEscaped) {
		t.Fatalf("script did not safely single-quote PowerShell path: got %s, want substring %s", script, wantEscaped)
	}
	if strings.Count(strings.ToLower(script), `c:\programdata\unbound`) != 1 {
		t.Fatalf("duplicate paths were not deduplicated: %s", script)
	}
}

func TestDefenderExclusionPathsIncludeProgramDataAndLocalAppData(t *testing.T) {
	cfgDir := t.TempDir()
	restore := SetConfigDirForTest(cfgDir)
	defer restore()

	localRoot := t.TempDir()
	t.Setenv("LOCALAPPDATA", localRoot)

	paths, err := defenderExclusionPaths()
	if err != nil {
		t.Fatalf("defenderExclusionPaths failed: %v", err)
	}

	joined := strings.ToLower(strings.Join(paths, "|"))
	if strings.Contains(joined, "clearflow") {
		t.Fatalf("defenderExclusionPaths contains stale clearflow path: %v", paths)
	}
	wantLocal := strings.ToLower(filepath.Join(localRoot, "Unbound"))
	if !strings.Contains(joined, wantLocal) {
		t.Fatalf("expected LOCALAPPDATA\\Unbound (%s) in %v", wantLocal, paths)
	}
	hasProgramDataUnbound := false
	for _, p := range paths {
		if strings.HasSuffix(strings.ToLower(p), `\unbound`) && p != filepath.Join(localRoot, "Unbound") {
			hasProgramDataUnbound = true
		}
	}
	if !hasProgramDataUnbound {
		t.Fatalf("expected ProgramData\\Unbound in %v", paths)
	}
}
