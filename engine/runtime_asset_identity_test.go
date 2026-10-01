package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var runtimeIdentityFormat = regexp.MustCompile(`^[0-9a-f]{64}$`)

// writeFakeAsset creates a file inside a synthetic extraction and records the
// hash exactly the way extractAssets does.
func writeFakeAsset(t *testing.T, files map[string]string, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	sum := sha256.Sum256([]byte(content))
	files[path] = hex.EncodeToString(sum[:])
}

// fakeExtractedAssets builds a synthetic, already verified extraction tree in a
// fresh temporary directory. It never touches the embedded assets, the network
// or the real runtime directory.
func fakeExtractedAssets(t *testing.T, engineContent string, lua map[string]string, listContent string) *AssetPaths {
	t.Helper()
	root := t.TempDir()
	files := make(map[string]string, len(lua)+2)
	binDir := filepath.Join(root, "core_bin")
	luaDir := filepath.Join(root, "lua_scripts")
	listDir := filepath.Join(root, "lists")
	writeFakeAsset(t, files, filepath.Join(binDir, platformEngineBinary()), engineContent)
	for name, content := range lua {
		writeFakeAsset(t, files, filepath.Join(luaDir, filepath.FromSlash(name)), content)
	}
	writeFakeAsset(t, files, filepath.Join(listDir, "default.lst"), listContent)
	return &AssetPaths{
		RootDir:        root,
		BinDir:         binDir,
		LuaDir:         luaDir,
		ListDir:        listDir,
		EngineSHA256:   files[filepath.Join(binDir, platformEngineBinary())],
		extractedFiles: files,
	}
}

func digestOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// expectedRuntimeIdentity reproduces the documented recipe — sorted logical
// names, one "name=hash" line each — so the test pins the format itself rather
// than the function that produces it.
func expectedRuntimeIdentity(hashes map[string]string) string {
	names := make([]string, 0, len(hashes))
	for name := range hashes {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name + "=" + hashes[name] + "\n")
	}
	return digestOf(b.String())
}

func mustRuntimeIdentity(t *testing.T, paths *AssetPaths) string {
	t.Helper()
	identity, err := VerifiedRuntimeAssetIdentity(paths)
	if err != nil {
		t.Fatalf("VerifiedRuntimeAssetIdentity returned error: %v", err)
	}
	if !runtimeIdentityFormat.MatchString(identity) {
		t.Fatalf("identity = %q, want 64 lowercase hex characters", identity)
	}
	return identity
}

func TestVerifiedRuntimeAssetIdentityKeysEntriesByLogicalName(t *testing.T) {
	engineContent := "engine-binary-bytes"
	lua := map[string]string{
		"zapret-lib.lua":       "lib",
		"init_vars.lua":        "vars",
		"unbound_adaptive.lua": "adaptive",
	}
	paths := fakeExtractedAssets(t, engineContent, lua, "0.0.0.0/0\n")

	want := expectedRuntimeIdentity(map[string]string{
		"engine":                        digestOf(engineContent),
		"bin:" + platformEngineBinary(): digestOf(engineContent),
		"lua:init_vars.lua":             digestOf("vars"),
		"lua:unbound_adaptive.lua":      digestOf("adaptive"),
		"lua:zapret-lib.lua":            digestOf("lib"),
	})
	if got := mustRuntimeIdentity(t, paths); got != want {
		t.Fatalf("identity = %q, want %q", got, want)
	}
}

func TestVerifiedRuntimeAssetIdentityIgnoresTheExtractionRoot(t *testing.T) {
	lua := map[string]string{"zapret-lib.lua": "lib"}
	first := fakeExtractedAssets(t, "engine", lua, "list")
	second := fakeExtractedAssets(t, "engine", lua, "list")
	if first.RootDir == second.RootDir {
		t.Fatal("the two synthetic extractions share a root, the test proves nothing")
	}
	if got, want := mustRuntimeIdentity(t, first), mustRuntimeIdentity(t, second); got != want {
		t.Fatalf("extraction root changed the identity: %q != %q", got, want)
	}
}

func TestVerifiedRuntimeAssetIdentityIgnoresHostLists(t *testing.T) {
	lua := map[string]string{"zapret-lib.lua": "lib"}
	baseline := mustRuntimeIdentity(t, fakeExtractedAssets(t, "engine", lua, "0.0.0.0/0\n"))
	// Host lists are data, not executed semantics. Changing one must not change
	// the runtime identity, even though the recorded hash is updated with it.
	changed := fakeExtractedAssets(t, "engine", lua, "10.0.0.0/8\n")
	if got := mustRuntimeIdentity(t, changed); got != baseline {
		t.Fatalf("host list change altered the identity: %q != %q", got, baseline)
	}
}

func TestVerifiedRuntimeAssetIdentityChangesWithEngineBinary(t *testing.T) {
	lua := map[string]string{"zapret-lib.lua": "lib"}
	baseline := mustRuntimeIdentity(t, fakeExtractedAssets(t, "engine-v1", lua, "list"))
	changed := mustRuntimeIdentity(t, fakeExtractedAssets(t, "engine-v2", lua, "list"))
	if baseline == changed {
		t.Fatalf("engine binary change did not alter the identity %q", baseline)
	}
}

func TestVerifiedRuntimeAssetIdentityChangesWithASingleLuaScript(t *testing.T) {
	baseline := mustRuntimeIdentity(t, fakeExtractedAssets(t, "engine", map[string]string{
		"a.lua": "one",
		"b.lua": "two",
	}, "list"))
	changed := mustRuntimeIdentity(t, fakeExtractedAssets(t, "engine", map[string]string{
		"a.lua": "one",
		"b.lua": "two-modified",
	}, "list"))
	if baseline == changed {
		t.Fatalf("lua script change did not alter the identity %q", baseline)
	}
}

func TestVerifiedRuntimeAssetIdentityIncludesNestedLuaScripts(t *testing.T) {
	flat := mustRuntimeIdentity(t, fakeExtractedAssets(t, "engine", map[string]string{"a.lua": "one"}, "list"))
	nested := mustRuntimeIdentity(t, fakeExtractedAssets(t, "engine", map[string]string{
		"a.lua":        "one",
		"nested/b.lua": "two",
	}, "list"))
	if flat == nested {
		t.Fatalf("a nested lua script was ignored, identity stayed %q", flat)
	}
	want := expectedRuntimeIdentity(map[string]string{
		"engine":                        digestOf("engine"),
		"bin:" + platformEngineBinary(): digestOf("engine"),
		"lua:a.lua":                     digestOf("one"),
		"lua:nested/b.lua":              digestOf("two"),
	})
	if nested != want {
		t.Fatalf("nested identity = %q, want %q", nested, want)
	}
}

func TestVerifiedRuntimeAssetIdentityRejectsUnverifiedContent(t *testing.T) {
	paths := fakeExtractedAssets(t, "engine", map[string]string{"a.lua": "one"}, "list")
	// Tamper with the extracted file without updating the recorded hash: the
	// identity must not describe bytes that were never verified.
	tampered := filepath.Join(paths.LuaDir, "a.lua")
	if err := os.WriteFile(tampered, []byte("tampered"), 0o600); err != nil {
		t.Fatalf("tamper with %s: %v", tampered, err)
	}
	identity, err := VerifiedRuntimeAssetIdentity(paths)
	if err == nil {
		t.Fatalf("unverified content produced identity %q", identity)
	}
	if identity != "" {
		t.Fatalf("error returned identity %q", identity)
	}
}

func TestVerifiedRuntimeAssetIdentityRejectsIncompleteExtraction(t *testing.T) {
	paths := fakeExtractedAssets(t, "engine", map[string]string{"a.lua": "one"}, "list")
	enginePath := filepath.Join(paths.BinDir, platformEngineBinary())
	delete(paths.extractedFiles, enginePath)
	if identity, err := VerifiedRuntimeAssetIdentity(paths); err == nil {
		t.Fatalf("a missing engine binary produced identity %q", identity)
	}

	noLuaDir := fakeExtractedAssets(t, "engine", map[string]string{"a.lua": "one"}, "list")
	noLuaDir.LuaDir = ""
	if identity, err := VerifiedRuntimeAssetIdentity(noLuaDir); err == nil {
		t.Fatalf("an unknown lua directory produced identity %q", identity)
	}

	if identity, err := VerifiedRuntimeAssetIdentity(nil); err == nil {
		t.Fatalf("nil asset paths produced identity %q", identity)
	}
}
