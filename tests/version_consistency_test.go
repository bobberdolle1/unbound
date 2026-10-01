package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unbound/engine"
)

func TestReleaseVisibleVersionsMatchWailsMetadata(t *testing.T) {
	root := ".."
	wailsBytes, err := os.ReadFile(filepath.Join(root, "wails.json"))
	if err != nil {
		t.Fatalf("read wails.json: %v", err)
	}
	var wails struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(wailsBytes, &wails); err != nil {
		t.Fatalf("parse wails.json: %v", err)
	}
	version := wails.Info.ProductVersion
	if version == "" {
		t.Fatal("wails.json info.productVersion is empty")
	}

	frontendBytes, err := os.ReadFile(filepath.Join(root, "frontend", "package.json"))
	if err != nil {
		t.Fatalf("read frontend/package.json: %v", err)
	}
	var frontend struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(frontendBytes, &frontend); err != nil {
		t.Fatalf("parse frontend/package.json: %v", err)
	}
	if frontend.Version != version {
		t.Errorf("frontend version = %q, want canonical %q", frontend.Version, version)
	}

	lockBytes, err := os.ReadFile(filepath.Join(root, "frontend", "package-lock.json"))
	if err != nil {
		t.Fatalf("read frontend/package-lock.json: %v", err)
	}
	var lock struct {
		Version  string `json:"version"`
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(lockBytes, &lock); err != nil {
		t.Fatalf("parse frontend/package-lock.json: %v", err)
	}
	if lock.Version != version || lock.Packages[""].Version != version {
		t.Errorf("frontend/package-lock.json version = %q/%q, want canonical %q", lock.Version, lock.Packages[""].Version, version)
	}

	// The release-VISIBLE version strings are asserted ANCHORED, not by a
	// whole-file substring search. A substring check cannot tell a correct version
	// buried in prose from a wrong one in the document's title, and that is
	// exactly where drift ships: the README header and badge can advertise the
	// previous release while some unrelated line still contains the current one.
	anchored := []struct {
		path   string
		anchor string
	}{
		{"README.md", "# UNBOUND `v" + version + "`"},
		{"README.md", "badge/Version-v" + version + "-"},
		{"CHANGELOG.md", "## [" + version + "] - "},
		{"docs/PLATFORMS.md", "unbound-v" + version + "-linux-amd64"},
		{"docs/PLATFORMS.md", "**EXPERIMENTAL** CLI `tar.gz`"},
	}
	for _, want := range anchored {
		contents, err := os.ReadFile(filepath.Join(root, want.path))
		if err != nil {
			t.Errorf("read %s: %v", want.path, err)
			continue
		}
		if !strings.Contains(string(contents), want.anchor) {
			t.Errorf("%s does not contain release-visible anchor %q", want.path, want.anchor)
		}
	}

	makefile, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	if !strings.Contains(string(makefile), "require('./wails.json').info.productVersion") {
		t.Error("Makefile does not derive VERSION from wails.json")
	}
	if engine.Version != version {
		t.Errorf("engine.Version = %q, want canonical %q", engine.Version, version)
	}
}
