package main

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path"
	"strings"
	"testing"
)

func TestLinuxPackagingArchive(t *testing.T) {
	archivePath := os.Getenv("UNBOUND_LINUX_ARCHIVE")
	if archivePath == "" {
		t.Skip("set UNBOUND_LINUX_ARCHIVE to validate a staged Linux release archive")
	}
	file, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gzipReader.Close()

	required := map[string]bool{
		"unbound":                                false,
		"README.md":                              false,
		"CHANGELOG.md":                           false,
		"LICENSE":                                false,
		"ZAPRET2_LICENSE.txt":                    false,
		"ZAPRET_LICENSE.txt":                     false,
		"ENGINE_PROVENANCE.json":                 false,
		"ENGINE_ASSETS.sha256":                   false,
		"BUNDLE_SHA256SUMS.txt":                  false,
		"runtime/core_bin/linux/amd64/nfqws2":    false,
		"runtime/core_bin/linux/amd64/ip2net":    false,
		"runtime/core_bin/linux/amd64/mdig":      false,
		"runtime/lua_scripts/zapret-lib.lua":     false,
		"runtime/lua_scripts/zapret-antidpi.lua": false,
		"runtime/lists":                          false,
		"scripts/general_autotune.sh":            false,
	}
	allowedPrefixes := []string{"runtime/", "scripts/"}
	var root string
	entries := map[string]struct{}{}
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSuffix(header.Name, "/")
		if path.IsAbs(name) || strings.HasPrefix(path.Clean(name), "../") || name == ".." {
			t.Fatalf("unsafe archive path %q", header.Name)
		}
		parts := strings.Split(name, "/")
		if root == "" {
			root = parts[0]
			if !strings.HasSuffix(root, "-linux-amd64") {
				t.Fatalf("unexpected package root %q", root)
			}
		}
		if parts[0] != root {
			t.Fatalf("archive has multiple roots: %q", header.Name)
		}
		if len(parts) == 1 {
			continue
		}
		rel := strings.Join(parts[1:], "/")
		if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
			t.Fatalf("package must not contain link payloads: %q", header.Name)
		}
		if header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA {
			if header.FileInfo().Mode()&06000 != 0 || header.FileInfo().Mode()&0002 != 0 && header.FileInfo().Mode()&0111 != 0 {
				t.Fatalf("unsafe file mode for %q: %o", header.Name, header.FileInfo().Mode())
			}
			entries[rel] = struct{}{}
			if _, known := required[rel]; known {
				required[rel] = true
			}
			if !isLinuxPackageTopLevel(rel, allowedPrefixes) {
				t.Fatalf("unexpected package payload %q", rel)
			}
			if strings.Contains(rel, "/windows/") || strings.Contains(rel, "/darwin/") || strings.Contains(rel, "WinDivert") {
				t.Fatalf("non-Linux platform payload %q", rel)
			}
		}
	}
	if root == "" {
		t.Fatal("archive has no root")
	}
	for name, found := range required {
		if !found && name != "runtime/lists" {
			t.Errorf("required package payload missing: %s", name)
		}
	}
	if !hasPrefix(entries, "runtime/lists/") {
		t.Error("required runtime lists are missing")
	}
}

func isLinuxPackageTopLevel(rel string, allowedPrefixes []string) bool {
	if strings.Contains(rel, "/") {
		for _, prefix := range allowedPrefixes {
			if strings.HasPrefix(rel, prefix) {
				return true
			}
		}
		return false
	}
	for _, name := range []string{"unbound", "README.md", "CHANGELOG.md", "LICENSE", "ZAPRET2_LICENSE.txt", "ZAPRET_LICENSE.txt", "ENGINE_PROVENANCE.json", "ENGINE_ASSETS.sha256", "BUNDLE_SHA256SUMS.txt"} {
		if rel == name {
			return true
		}
	}
	return false
}

func hasPrefix(entries map[string]struct{}, prefix string) bool {
	for entry := range entries {
		if strings.HasPrefix(entry, prefix) {
			return true
		}
	}
	return false
}
