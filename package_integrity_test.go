package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyBundleRootAcceptsCompleteManifest(t *testing.T) {
	root := t.TempDir()
	writeBundleFile(t, root, "unbound", "binary")
	writeBundleFile(t, root, "runtime/core_bin/linux/amd64/nfqws2", "engine")
	writeBundleManifest(t, root, []string{"unbound", "runtime/core_bin/linux/amd64/nfqws2"})
	if err := verifyBundleRoot(root); err != nil {
		t.Fatalf("verify complete bundle: %v", err)
	}
}

func TestVerifyBundleRootRejectsTamperedOrMissingPayload(t *testing.T) {
	root := t.TempDir()
	writeBundleFile(t, root, "unbound", "binary")
	writeBundleFile(t, root, "runtime/core_bin/linux/amd64/nfqws2", "engine")
	writeBundleManifest(t, root, []string{"unbound", "runtime/core_bin/linux/amd64/nfqws2"})
	if err := os.WriteFile(filepath.Join(root, "runtime/core_bin/linux/amd64/nfqws2"), []byte("tampered"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := verifyBundleRoot(root); err == nil {
		t.Fatal("tampered nfqws2 was accepted")
	}
	if err := os.Remove(filepath.Join(root, "unbound")); err != nil {
		t.Fatal(err)
	}
	if err := verifyBundleRoot(root); err == nil {
		t.Fatal("missing package payload was accepted")
	}
}

func TestVerifyBundleRootRejectsUnexpectedAndUnsafePayloads(t *testing.T) {
	root := t.TempDir()
	writeBundleFile(t, root, "unbound", "binary")
	writeBundleManifest(t, root, []string{"unbound"})
	writeBundleFile(t, root, "unexpected.log", "unexpected")
	if err := verifyBundleRoot(root); err == nil {
		t.Fatal("unexpected payload was accepted")
	}
	if _, err := safeBundlePath("../nfqws2"); err == nil {
		t.Fatal("traversal manifest path was accepted")
	}
}

func writeBundleFile(t *testing.T, root, name, content string) {
	t.Helper()
	filename := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeBundleManifest(t *testing.T, root string, names []string) {
	t.Helper()
	manifest := ""
	for _, name := range names {
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		manifest += fmt.Sprintf("%x  %s\n", sha256.Sum256(contents), name)
	}
	if err := os.WriteFile(filepath.Join(root, "BUNDLE_SHA256SUMS.txt"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}
