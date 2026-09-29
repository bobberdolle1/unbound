package main

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var bundleManifestLine = regexp.MustCompile(`^([a-f0-9]{64})  ([^\r\n]+)$`)

// verifyAdjacentBundle validates a packaged distribution before any CLI mode
// executes. Source and development builds have no adjacent bundle manifest and
// retain their existing embedded-asset behavior.
func verifyAdjacentBundle() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable for package verification: %w", err)
	}
	root := filepath.Dir(executable)
	manifest := filepath.Join(root, "BUNDLE_SHA256SUMS.txt")
	if _, err := os.Stat(manifest); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect package manifest: %w", err)
	}
	return verifyBundleRoot(root)
}

func verifyBundleRoot(root string) error {
	manifestPath := filepath.Join(root, "BUNDLE_SHA256SUMS.txt")
	file, err := os.Open(manifestPath)
	if err != nil {
		return fmt.Errorf("open package manifest: %w", err)
	}
	defer file.Close()

	expected := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		match := bundleManifestLine.FindStringSubmatch(line)
		if match == nil {
			return fmt.Errorf("invalid package manifest entry %q", line)
		}
		rel, err := safeBundlePath(match[2])
		if err != nil {
			return err
		}
		if _, exists := expected[rel]; exists {
			return fmt.Errorf("duplicate package manifest entry %q", rel)
		}
		expected[rel] = match[1]
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read package manifest: %w", err)
	}
	if len(expected) == 0 {
		return fmt.Errorf("package manifest is empty")
	}

	actual := make(map[string]struct{})
	err = filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filename == root {
			return nil
		}
		rel, err := filepath.Rel(root, filename)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "BUNDLE_SHA256SUMS.txt" {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("package contains unsupported symlink %q", rel)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("package contains non-regular payload %q", rel)
		}
		expectedHash, listed := expected[rel]
		if !listed {
			return fmt.Errorf("unexpected package payload %q", rel)
		}
		actualHash, err := fileSHA256(filename)
		if err != nil {
			return err
		}
		if actualHash != expectedHash {
			return fmt.Errorf("package payload hash mismatch for %q", rel)
		}
		actual[rel] = struct{}{}
		return nil
	})
	if err != nil {
		return fmt.Errorf("verify package payload: %w", err)
	}
	for rel := range expected {
		if _, found := actual[rel]; !found {
			return fmt.Errorf("package payload missing %q", rel)
		}
	}
	return nil
}

func safeBundlePath(raw string) (string, error) {
	if strings.Contains(raw, "\\") || path.IsAbs(raw) {
		return "", fmt.Errorf("unsafe package manifest path %q", raw)
	}
	clean := path.Clean(raw)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("unsafe package manifest path %q", raw)
	}
	return clean, nil
}

func fileSHA256(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", filename, err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash %s: %w", filename, err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
