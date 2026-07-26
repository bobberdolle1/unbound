package providers

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeExecutable(t *testing.T, dir, name string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestResolveEngineBinaryPrefersAssetDir(t *testing.T) {
	assetDir := t.TempDir()
	want := writeExecutable(t, assetDir, "nfqws", 0o755)

	got, err := ResolveEngineBinary("nfqws", assetDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want the bundled binary %q", got, want)
	}
}

// A binary extracted onto a filesystem that drops the executable bit (or
// restored from an archive that does not carry permissions) should be repaired
// rather than reported as missing.
func TestResolveEngineBinaryRestoresExecutableBit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not meaningful on Windows")
	}

	assetDir := t.TempDir()
	path := writeExecutable(t, assetDir, "nfqws", 0o644)

	got, err := ResolveEngineBinary("nfqws", assetDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != path {
		t.Fatalf("got %q, want %q", got, path)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Error("executable bit was not restored")
	}
}

func TestResolveEngineBinaryIgnoresDirectories(t *testing.T) {
	assetDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(assetDir, "nfqws"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := ResolveEngineBinary("nfqws", assetDir); err == nil {
		t.Error("expected an error when the only match is a directory")
	}
}

// The error is the sole diagnostic a user gets when no engine is installed, so
// it must name the paths that were checked.
func TestResolveEngineBinaryErrorListsSearchedPaths(t *testing.T) {
	_, err := ResolveEngineBinary("definitely-not-a-real-engine", t.TempDir())
	if err == nil {
		t.Fatal("expected an error for a nonexistent binary")
	}
	if !strings.Contains(err.Error(), "definitely-not-a-real-engine") {
		t.Errorf("error should name the binary, got: %v", err)
	}
	if !strings.Contains(err.Error(), "$PATH") {
		t.Errorf("error should report that $PATH was searched, got: %v", err)
	}
}

func TestResolveEngineBinaryAddsExeSuffixOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific naming")
	}

	assetDir := t.TempDir()
	want := writeExecutable(t, assetDir, "winws2.exe", 0o755)

	got, err := ResolveEngineBinary("winws2", assetDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
