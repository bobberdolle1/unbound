package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteFileAtomicVerifiedReplacesExistingAsset(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "winws2.exe")
	if err := os.WriteFile(target, []byte("untrusted"), 0600); err != nil {
		t.Fatal(err)
	}

	trusted := []byte("trusted engine")
	expected := testSHA256(trusted)
	if err := writeFileAtomicVerified(target, trusted, 0700, expected); err != nil {
		t.Fatalf("writeFileAtomicVerified: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(trusted) {
		t.Fatalf("activated bytes = %q, want %q", got, trusted)
	}
}

func TestWriteFileAtomicVerifiedFailsClosed(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "winws2.exe")
	original := []byte("existing engine")
	if err := os.WriteFile(target, original, 0600); err != nil {
		t.Fatal(err)
	}

	if err := writeFileAtomicVerified(target, []byte("new engine"), 0700, strings.Repeat("0", 64)); err == nil {
		t.Fatal("expected staged hash mismatch")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("failed staging changed active file: got %q, want %q", got, original)
	}
}

func TestVerifyExtractedFilesDetectsTampering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asset.bin")
	trusted := []byte("trusted")
	if err := os.WriteFile(path, trusted, 0600); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{path: testSHA256(trusted)}
	if err := verifyExtractedFiles(files); err != nil {
		t.Fatalf("trusted asset rejected: %v", err)
	}
	if err := os.WriteFile(path, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyExtractedFiles(files); err == nil {
		t.Fatal("tampered asset passed verification")
	}
}

func TestExtractAssetsUsesPrivateRuntime(t *testing.T) {
	paths, err := ExtractAssets()
	if err != nil {
		t.Fatalf("ExtractAssets: %v", err)
	}
	if filepath.Base(paths.RootDir) == "clearflow" {
		t.Fatalf("legacy predictable runtime path is still in use: %s", paths.RootDir)
	}
	if err := VerifyExtractedAssets(paths); err != nil {
		t.Fatalf("VerifyExtractedAssets: %v", err)
	}
	if runtime.GOOS == "windows" && paths.EngineSHA256 == "" {
		t.Fatal("Windows engine SHA-256 was not captured")
	}
}

func testSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
