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

	for _, path := range []string{"README.md", "CHANGELOG.md", filepath.Join("frontend", "package-lock.json")} {
		contents, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		if !strings.Contains(string(contents), version) {
			t.Errorf("%s does not contain canonical version %q", path, version)
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
