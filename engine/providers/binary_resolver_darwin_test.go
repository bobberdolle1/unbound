//go:build darwin
// +build darwin

package providers

import (
	"path/filepath"
	"testing"
)

func TestResolveEngineBinaryDarwinPrefersExtractedAssetOverPATH(t *testing.T) {
	assetDir := t.TempDir()
	assetPath := writeExecutable(t, assetDir, MacOSEngineBinary, 0o755)

	pathDir := t.TempDir()
	writeExecutable(t, pathDir, MacOSEngineBinary, 0o755)
	t.Setenv("PATH", pathDir)

	got, err := ResolveEngineBinary(MacOSEngineBinary, assetDir)
	if err != nil {
		t.Fatalf("ResolveEngineBinary: %v", err)
	}
	if got != assetPath {
		t.Fatalf("resolved %q, want extracted asset %q before PATH fallback", got, assetPath)
	}
	if filepath.Dir(got) != assetDir {
		t.Fatalf("resolved tpws from %q, want extracted asset directory %q", filepath.Dir(got), assetDir)
	}
}
