package historyid

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"unbound/engine"
	"unbound/engine/backendcap"
)

var backendFormat = regexp.MustCompile(`^backend-v1-[0-9a-f]{64}$`)

const (
	testEngineSHA256    = "3f1c0b5d2a9e6477c1b0a9d8e7f60514233445566778899aabbccddeeff00112"
	testRuntimeIdentity = "9c8b7a6554433221100ffeeddccbbaa99887766554433221100ffeeddccbbaa9"
)

// stubRuntimeAssets stands in for engine.VerifiedRuntimeAssetIdentity so the
// backend identity can be exercised without extracting assets to disk. The
// recorded argument lets a test observe exactly which AssetPaths reached the
// resolver.
func stubRuntimeAssets(identity string, seen **engine.AssetPaths) func(*engine.AssetPaths) (string, error) {
	return func(assets *engine.AssetPaths) (string, error) {
		if seen != nil {
			*seen = assets
		}
		return identity, nil
	}
}

// fakeAssets builds an AssetPaths rooted at a unique temporary directory. The
// files are never created: nothing in the backend identity reads them.
func fakeAssets(t *testing.T, engineSHA256 string) *engine.AssetPaths {
	t.Helper()
	root := t.TempDir()
	return &engine.AssetPaths{
		RootDir:      root,
		BinDir:       filepath.Join(root, "core_bin"),
		LuaDir:       filepath.Join(root, "lua_scripts"),
		ListDir:      filepath.Join(root, "lists"),
		EngineSHA256: engineSHA256,
	}
}

// mustBackendFingerprint derives an identity with a stubbed runtime asset
// resolver, so no assets have to exist on disk.
func mustBackendFingerprint(t *testing.T, assets *engine.AssetPaths, contractRevision string) BackendFingerprint {
	t.Helper()
	fingerprint, err := backendFingerprintWith(
		backendcap.Get(backendcap.Zapret2Windows),
		assets,
		contractRevision,
		stubRuntimeAssets(testRuntimeIdentity, nil),
	)
	if err != nil {
		t.Fatalf("backendFingerprintWith returned error: %v", err)
	}
	return fingerprint
}

func TestBackendFingerprintRequiresExtractedAssets(t *testing.T) {
	fingerprint, err := BackendFingerprintFor(backendcap.Zapret2Windows, nil)
	if err == nil {
		t.Fatalf("nil assets produced fingerprint %q, want an error", fingerprint)
	}
	if fingerprint != "" {
		t.Fatalf("error returned fingerprint %q", fingerprint)
	}
	var unavailable *ErrUnavailable
	if !errors.As(err, &unavailable) {
		t.Fatalf("error %v is not an *ErrUnavailable", err)
	}
}

func TestBackendFingerprintRequiresEngineHash(t *testing.T) {
	for name, assets := range map[string]*engine.AssetPaths{
		"empty_hash":  fakeAssets(t, ""),
		"blank_hash":  fakeAssets(t, "   "),
		"unpopulated": {},
	} {
		fingerprint, err := BackendFingerprintFor(backendcap.Zapret2Windows, assets)
		if err == nil {
			t.Fatalf("%s: produced fingerprint %q, want an error", name, fingerprint)
		}
		var unavailable *ErrUnavailable
		if !errors.As(err, &unavailable) {
			t.Fatalf("%s: error %v is not an *ErrUnavailable", name, err)
		}
	}
}

func TestBackendFingerprintReportsUnverifiedRuntimeAssets(t *testing.T) {
	assets := fakeAssets(t, testEngineSHA256)
	failing := func(*engine.AssetPaths) (string, error) {
		return "", errors.New("verify extracted assets: hash mismatch")
	}
	fingerprint, err := backendFingerprintWith(backendcap.Get(backendcap.Zapret2Windows), assets, ExecutionContractRevision, failing)
	if err == nil {
		t.Fatalf("unverified assets produced fingerprint %q, want an error", fingerprint)
	}
	if fingerprint != "" {
		t.Fatalf("error returned fingerprint %q", fingerprint)
	}
	empty := func(*engine.AssetPaths) (string, error) { return "  ", nil }
	if _, err := backendFingerprintWith(backendcap.Get(backendcap.Zapret2Windows), assets, ExecutionContractRevision, empty); err == nil {
		t.Fatal("an empty runtime asset identity was accepted")
	}
}

func TestBackendFingerprintFormatIsVersionedLowercaseHex(t *testing.T) {
	fingerprint := mustBackendFingerprint(t, fakeAssets(t, testEngineSHA256), ExecutionContractRevision)
	if !backendFormat.MatchString(string(fingerprint)) {
		t.Fatalf("fingerprint = %q, want backend-v1- followed by 64 lowercase hex", fingerprint)
	}
	if !fingerprint.Available() {
		t.Fatalf("fingerprint %q reports unavailable", fingerprint)
	}
}

func TestBackendFingerprintIsDeterministic(t *testing.T) {
	first := mustBackendFingerprint(t, fakeAssets(t, testEngineSHA256), ExecutionContractRevision)
	second := mustBackendFingerprint(t, fakeAssets(t, testEngineSHA256), ExecutionContractRevision)
	if first != second {
		t.Fatalf("same inputs produced different fingerprints: %q != %q", first, second)
	}
}

func TestBackendFingerprintExcludesEveryFilesystemPath(t *testing.T) {
	capabilities := backendcap.Get(backendcap.Zapret2Windows)
	canonical, err := backendCanonical(capabilities, testEngineSHA256, testRuntimeIdentity, ExecutionContractRevision)
	if err != nil {
		t.Fatalf("backendCanonical returned error: %v", err)
	}

	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	first := &engine.AssetPaths{
		RootDir:      firstRoot,
		BinDir:       filepath.Join(firstRoot, "core_bin"),
		LuaDir:       filepath.Join(firstRoot, "lua_scripts"),
		ListDir:      filepath.Join(firstRoot, "lists"),
		EngineSHA256: testEngineSHA256,
	}
	second := &engine.AssetPaths{
		RootDir:      secondRoot,
		BinDir:       filepath.Join(secondRoot, "core_bin"),
		LuaDir:       filepath.Join(secondRoot, "lua_scripts"),
		ListDir:      filepath.Join(secondRoot, "lists"),
		EngineSHA256: testEngineSHA256,
	}

	firstFingerprint, err := backendFingerprintWith(capabilities, first, ExecutionContractRevision, stubRuntimeAssets(testRuntimeIdentity, nil))
	if err != nil {
		t.Fatalf("first fingerprint: %v", err)
	}
	secondFingerprint, err := backendFingerprintWith(capabilities, second, ExecutionContractRevision, stubRuntimeAssets(testRuntimeIdentity, nil))
	if err != nil {
		t.Fatalf("second fingerprint: %v", err)
	}
	if firstFingerprint != secondFingerprint {
		t.Fatalf("extraction root changed the fingerprint: %q != %q", firstFingerprint, secondFingerprint)
	}
	for _, path := range []string{firstRoot, secondRoot, first.BinDir, second.LuaDir, filepath.ToSlash(firstRoot)} {
		if strings.Contains(string(canonical), path) {
			t.Fatalf("canonical form leaks the filesystem path %q:\n%s", path, canonical)
		}
	}
	if strings.Contains(string(firstFingerprint), filepath.Base(firstRoot)) {
		t.Fatalf("fingerprint %q leaks the extraction root name", firstFingerprint)
	}
}

func TestBackendFingerprintResolvesRuntimeAssetsFromTheGivenPaths(t *testing.T) {
	assets := fakeAssets(t, testEngineSHA256)
	var observed *engine.AssetPaths
	if _, err := backendFingerprintWith(
		backendcap.Get(backendcap.Zapret2Windows),
		assets,
		ExecutionContractRevision,
		stubRuntimeAssets(testRuntimeIdentity, &observed),
	); err != nil {
		t.Fatalf("backendFingerprintWith returned error: %v", err)
	}
	if observed != assets {
		t.Fatal("the runtime asset resolver did not receive the supplied asset paths")
	}
}

func TestBackendFingerprintChangesWithTheEngineHash(t *testing.T) {
	baseline := mustBackendFingerprint(t, fakeAssets(t, testEngineSHA256), ExecutionContractRevision)
	changed := mustBackendFingerprint(t, fakeAssets(t, strings.Repeat("a", 64)), ExecutionContractRevision)
	if baseline == changed {
		t.Fatalf("changing the engine hash did not change %q", baseline)
	}
}

func TestBackendFingerprintChangesWithTheRuntimeAssetIdentity(t *testing.T) {
	capabilities := backendcap.Get(backendcap.Zapret2Windows)
	assets := fakeAssets(t, testEngineSHA256)
	baseline, err := backendFingerprintWith(capabilities, assets, ExecutionContractRevision, stubRuntimeAssets(testRuntimeIdentity, nil))
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	changed, err := backendFingerprintWith(capabilities, assets, ExecutionContractRevision, stubRuntimeAssets(strings.Repeat("b", 64), nil))
	if err != nil {
		t.Fatalf("changed: %v", err)
	}
	if baseline == changed {
		t.Fatalf("changing the runtime asset identity did not change %q", baseline)
	}
}

func TestBackendFingerprintChangesWithProvenance(t *testing.T) {
	assets := fakeAssets(t, testEngineSHA256)
	baseline := backendcap.Get(backendcap.Zapret2Windows)

	commitChanged := baseline
	commitChanged.Provenance.Commit = "0000000000000000000000000000000000000000"

	baseTagChanged := baseline
	baseTagChanged.Provenance.BaseTag = "v1.0.5"

	tagChanged := baseline
	tagChanged.Provenance.Tag = "v1.0.5.2"

	baselineFingerprint, err := backendFingerprintWith(baseline, assets, ExecutionContractRevision, stubRuntimeAssets(testRuntimeIdentity, nil))
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	for name, capabilities := range map[string]backendcap.Capabilities{
		"commit_changed":   commitChanged,
		"base_tag_changed": baseTagChanged,
		"tag_changed":      tagChanged,
	} {
		got, err := backendFingerprintWith(capabilities, assets, ExecutionContractRevision, stubRuntimeAssets(testRuntimeIdentity, nil))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got == baselineFingerprint {
			t.Fatalf("%s did not change the fingerprint %q", name, baselineFingerprint)
		}
	}
}

func TestBackendFingerprintChangesWithTheExecutionContractRevision(t *testing.T) {
	assets := fakeAssets(t, testEngineSHA256)
	baseline := mustBackendFingerprint(t, assets, ExecutionContractRevision)
	changed := mustBackendFingerprint(t, assets, ExecutionContractRevision+"-next")
	if baseline == changed {
		t.Fatalf("changing the execution contract revision did not change %q", baseline)
	}
}

func TestBackendFingerprintDistinguishesBackends(t *testing.T) {
	assets := fakeAssets(t, testEngineSHA256)
	backends := []backendcap.Backend{
		backendcap.Zapret2Windows,
		backendcap.Zapret2Linux,
		backendcap.Zapret1TPWSDarwin,
		backendcap.NativeWindows,
		backendcap.NativeDarwin,
		backendcap.Backend(""),
	}
	seen := make(map[BackendFingerprint]backendcap.Backend, len(backends))
	for _, backend := range backends {
		fingerprint, err := backendFingerprintWith(
			backendcap.Get(backend), assets, ExecutionContractRevision, stubRuntimeAssets(testRuntimeIdentity, nil),
		)
		if err != nil {
			t.Fatalf("backend %q: %v", backend, err)
		}
		if previous, clash := seen[fingerprint]; clash {
			t.Fatalf("backends %q and %q share fingerprint %q", previous, backend, fingerprint)
		}
		seen[fingerprint] = backend
	}
}

func TestBackendFingerprintRejectsUnencodableSourceMaterial(t *testing.T) {
	assets := fakeAssets(t, testEngineSHA256)
	resolver := stubRuntimeAssets(testRuntimeIdentity, nil)
	if _, err := backendFingerprintWith(
		backendcap.Get(backendcap.Zapret2Windows), assets, "strategyir-execution-v1\ninjected", resolver,
	); err == nil {
		t.Fatal("a contract revision containing a line break was accepted")
	}
	forged := backendcap.Get(backendcap.Zapret2Windows)
	forged.Provenance.Commit = "abc\nengine_sha256=" + testEngineSHA256
	fingerprint, err := backendFingerprintWith(forged, assets, ExecutionContractRevision, resolver)
	if err == nil {
		t.Fatalf("a commit containing a line break was accepted as %q", fingerprint)
	}
}
