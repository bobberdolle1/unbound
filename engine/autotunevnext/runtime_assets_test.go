package autotunevnext

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"unbound/engine"
	"unbound/engine/backendcap"
)

func TestNewProductAssetResolverPrefersUserListsDirWithBundleFallback(t *testing.T) {
	extractConfig := t.TempDir()
	restoreExtract := engine.SetConfigDirForTest(extractConfig)
	assets, err := engine.ExtractAssets()
	restoreExtract()
	if err != nil {
		t.Fatalf("ExtractAssets failed: %v", err)
	}

	// 1. Empty user lists dir -> falls back to extracted paths.ListDir.
	emptyConfig := t.TempDir()
	restoreEmpty := engine.SetConfigDirForTest(emptyConfig)
	resolverFallback, err := NewProductAssetResolver(assets)
	restoreEmpty()
	if err != nil {
		t.Fatalf("NewProductAssetResolver with empty GetListsDir failed: %v", err)
	}
	fallbackResolved, err := resolverFallback.Resolve(context.Background(), backendcap.Zapret2Windows, []string{"youtube", "ipset-all"})
	if err != nil {
		t.Fatalf("Resolve fallback assets failed: %v", err)
	}
	wantFallbackYoutube := filepath.ToSlash(filepath.Join(assets.ListDir, "youtube.txt"))
	for _, a := range fallbackResolved {
		if a.ID == "youtube" && a.EngineValue != wantFallbackYoutube {
			t.Fatalf("fallback youtube EngineValue = %q, want %q", a.EngineValue, wantFallbackYoutube)
		}
	}

	// 2. Populated user lists dir -> prefers GetListsDir() for synced files, falls back to paths.ListDir for others.
	userConfig := t.TempDir()
	restoreUser := engine.SetConfigDirForTest(userConfig)
	defer restoreUser()

	userListsDir, err := engine.GetListsDir()
	if err != nil {
		t.Fatalf("GetListsDir failed: %v", err)
	}
	customYoutubePath := filepath.Join(userListsDir, "youtube.txt")
	if err := os.WriteFile(customYoutubePath, []byte("custom-youtube.example\n"), 0644); err != nil {
		t.Fatalf("write custom youtube.txt failed: %v", err)
	}

	resolverUser, err := NewProductAssetResolver(assets)
	if err != nil {
		t.Fatalf("NewProductAssetResolver with populated GetListsDir failed: %v", err)
	}
	resolved, err := resolverUser.Resolve(context.Background(), backendcap.Zapret2Windows, []string{"youtube", "ipset-all", "tls-google"})
	if err != nil {
		t.Fatalf("Resolve mixed assets failed: %v", err)
	}

	byID := map[string]ResolvedAsset{}
	for _, a := range resolved {
		byID[a.ID] = a
	}
	wantUserYoutube := filepath.ToSlash(customYoutubePath)
	if byID["youtube"].EngineValue != wantUserYoutube || byID["youtube"].Kind != AssetKindHostlistFile {
		t.Fatalf("youtube resolved to %+v, want user list %q", byID["youtube"], wantUserYoutube)
	}
	wantBundleIPSet := filepath.ToSlash(filepath.Join(assets.ListDir, "ipset-all.txt"))
	if byID["ipset-all"].EngineValue != wantBundleIPSet || byID["ipset-all"].Kind != AssetKindIPSetFile {
		t.Fatalf("ipset-all resolved to %+v, want bundle fallback %q", byID["ipset-all"], wantBundleIPSet)
	}
	if byID["tls-google"].EngineValue != "tls_google" || byID["tls-google"].Kind != AssetKindBlobSymbol {
		t.Fatalf("tls-google blob symbol broken: %+v", byID["tls-google"])
	}
}
