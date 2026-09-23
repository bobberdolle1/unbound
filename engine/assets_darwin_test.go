//go:build darwin
// +build darwin

package engine

import (
	"bytes"
	"crypto/sha256"
	"debug/macho"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const macOSTPWSCommit = "d437963452674faadfd45adcd62466272b5a2fcd"

type macOSTPWSProvenance struct {
	SchemaVersion int `json:"schemaVersion"`
	Bundles       []struct {
		Name          string `json:"name"`
		BaseTag       string `json:"baseTag"`
		Commit        string `json:"commit"`
		SourceKind    string `json:"sourceKind"`
		SourceArchive struct {
			URL       string `json:"url"`
			Reference string `json:"reference"`
			SHA256    string `json:"sha256"`
		} `json:"sourceArchive"`
		Build struct {
			Artifact struct {
				SHA256 string `json:"sha256"`
				Slices struct {
					ARM64  string `json:"arm64"`
					X86_64 string `json:"x86_64"`
				} `json:"slices"`
			} `json:"artifact"`
		} `json:"build"`
	} `json:"bundles"`
}

func TestMacOSTPWSAssetContract(t *testing.T) {
	const assetPath = "core_bin/darwin/tpws"

	embedded, err := EmbeddedAssets.ReadFile(assetPath)
	if err != nil {
		t.Fatalf("read embedded macOS tpws: %v", err)
	}
	if len(embedded) == 0 {
		t.Fatal("embedded macOS tpws is empty")
	}

	manifest, err := EmbeddedAssets.ReadFile("ENGINE_ASSETS.sha256")
	if err != nil {
		t.Fatalf("read asset manifest: %v", err)
	}
	wantHash := manifestHash(t, string(manifest), "engine/"+assetPath)
	actualHash := sha256.Sum256(embedded)
	if got := hex.EncodeToString(actualHash[:]); got != wantHash {
		t.Fatalf("embedded macOS tpws hash = %s, manifest = %s", got, wantHash)
	}

	info, err := os.Stat(assetPath)
	if err != nil {
		t.Fatalf("stat source tpws: %v", err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Fatalf("source tpws is not executable: mode %o", info.Mode().Perm())
	}

	fatPath := filepath.Join(t.TempDir(), "tpws")
	if err := os.WriteFile(fatPath, embedded, 0700); err != nil {
		t.Fatalf("stage embedded macOS tpws for Mach-O inspection: %v", err)
	}
	fat, err := macho.OpenFat(fatPath)
	if err != nil {
		t.Fatalf("open universal macOS tpws: %v", err)
	}
	defer fat.Close()
	architectures := map[macho.Cpu]bool{}
	for _, file := range fat.Arches {
		architectures[file.Cpu] = true
	}
	if !architectures[macho.CpuArm64] || !architectures[macho.CpuAmd64] {
		t.Fatalf("macOS tpws architectures = %v, want arm64 and x86_64", architectures)
	}

	provenanceBytes, err := os.ReadFile("ENGINE_PROVENANCE.json")
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}
	var provenance macOSTPWSProvenance
	if err := json.Unmarshal(provenanceBytes, &provenance); err != nil {
		t.Fatalf("parse provenance: %v", err)
	}
	if provenance.SchemaVersion < 2 {
		t.Fatalf("provenance schema = %d, want >= 2", provenance.SchemaVersion)
	}
	for _, bundle := range provenance.Bundles {
		if bundle.Name != "zapret" {
			continue
		}
		if bundle.BaseTag != "v72.13" || bundle.Commit != macOSTPWSCommit || bundle.SourceKind != "git_commit" {
			t.Fatalf("macOS tpws provenance = baseTag %q, commit %q, sourceKind %q", bundle.BaseTag, bundle.Commit, bundle.SourceKind)
		}
		if bundle.SourceArchive.Reference != macOSTPWSCommit || bundle.SourceArchive.URL == "" || bundle.SourceArchive.SHA256 == "" {
			t.Fatalf("macOS tpws source archive provenance is incomplete: %+v", bundle.SourceArchive)
		}
		if bundle.Build.Artifact.SHA256 != wantHash || bundle.Build.Artifact.Slices.ARM64 == "" || bundle.Build.Artifact.Slices.X86_64 == "" {
			t.Fatalf("macOS tpws build artifact provenance is incomplete: %+v", bundle.Build.Artifact)
		}
		return
	}
	t.Fatal("zapret macOS tpws provenance is missing")
}

func TestMacOSTPWSExtractionIsByteIdenticalAndExecutable(t *testing.T) {
	paths, err := ExtractAssets()
	if err != nil {
		t.Fatalf("ExtractAssets: %v", err)
	}

	embedded, err := EmbeddedAssets.ReadFile("core_bin/darwin/tpws")
	if err != nil {
		t.Fatalf("read embedded macOS tpws: %v", err)
	}
	extractedPath := filepath.Join(paths.BinDir, platformEngineBinary())
	extracted, err := os.ReadFile(extractedPath)
	if err != nil {
		t.Fatalf("read extracted macOS tpws: %v", err)
	}
	if !bytes.Equal(extracted, embedded) {
		t.Fatal("extracted macOS tpws differs from the embedded asset")
	}
	info, err := os.Stat(extractedPath)
	if err != nil {
		t.Fatalf("stat extracted macOS tpws: %v", err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Fatalf("extracted macOS tpws is not executable: mode %o", info.Mode().Perm())
	}
}

func manifestHash(t *testing.T, manifest, path string) string {
	t.Helper()
	for _, line := range strings.Split(manifest, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == path {
			return fields[0]
		}
	}
	t.Fatalf("manifest has no hash for %s", path)
	return ""
}
