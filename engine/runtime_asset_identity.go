package engine

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// VerifiedRuntimeAssetIdentity returns a deterministic digest over the verified
// runtime assets that can change executed semantics: the platform engine
// binary plus the Lua runtime scripts.
//
// WHY CONTENT, NOT LOCATION. Entries are keyed by a LOGICAL name ("engine",
// "lua:<script>") and never by filesystem path. The extraction root is a fresh
// per-process temporary directory, so a path-keyed identity would differ on
// every run and on every machine while describing byte-identical behaviour. The
// per-run temp location must never escape this function.
//
// WHY THESE ASSETS ONLY. The host list files under ListDir are deliberately
// excluded: they are data consumed by the engine, not executed semantics, and
// they change without any change in what the engine can do. WinDivert filters
// are excluded for the same reason.
//
// VERIFICATION FIRST. The digest is only meaningful over assets that have just
// been proven to match the embedded originals. An identity derived from an
// unverified extraction would describe bytes that were never shown to be the
// shipped ones, which is exactly the kind of unverifiable claim this package
// must never make.
func VerifiedRuntimeAssetIdentity(paths *AssetPaths) (string, error) {
	if paths == nil {
		return "", fmt.Errorf("asset paths are not initialized")
	}
	if err := verifyExtractedFiles(paths.extractedFiles); err != nil {
		return "", fmt.Errorf("verify extracted assets: %w", err)
	}
	if strings.TrimSpace(paths.LuaDir) == "" {
		return "", fmt.Errorf("asset paths do not identify the Lua script directory")
	}

	enginePath := filepath.Join(paths.BinDir, platformEngineBinary())
	engineHash, ok := paths.extractedFiles[enginePath]
	if !ok || engineHash == "" {
		return "", fmt.Errorf("verified assets do not contain the %s engine binary", platformEngineBinary())
	}

	// Coverage is deliberately broad over anything that can change what the engine
	// puts on the wire: the engine binary, the fake-payload blobs it references by
	// logical id, the capture driver and its runtime libraries, and the Lua runtime.
	// A strategy's payload ref is resolved by the engine at execution time, so
	// replacing fake_tls_8.bin or swapping the driver changes the executed bytes
	// while every logical fingerprint in the IR stays identical - that is the
	// dangerous direction, and under-invalidating history is worse than
	// over-invalidating it.
	//
	// Managed host/ips lists are excluded: they are data the user curates, not
	// engine semantics, and folding them in would invalidate history on ordinary
	// list edits.
	binDir := filepath.Clean(paths.BinDir)
	luaDir := filepath.Clean(paths.LuaDir)
	entries := map[string]string{"engine": engineHash}
	for path, hash := range paths.extractedFiles {
		if hash == "" {
			return "", fmt.Errorf("verified asset %q has no recorded hash", filepath.Base(path))
		}
		switch {
		case withinDir(path, binDir):
			entries["bin:"+filepath.Base(path)] = hash
		case withinDir(path, luaDir):
			relative, err := filepath.Rel(luaDir, path)
			if err != nil {
				continue
			}
			entries["lua:"+filepath.ToSlash(relative)] = hash
		}
	}

	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)

	// The map is only read above in a fixed key order from here on, so no map
	// iteration order can leak into the digest.
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteByte('=')
		b.WriteString(entries[name])
		b.WriteByte('\n')
	}
	return sha256Hex([]byte(b.String())), nil
}

// withinDir reports whether path lives inside dir. It exists so the digest can
// classify a verified asset by LOGICAL role without ever exposing the temporary
// extraction path it happened to land on.
func withinDir(path, dir string) bool {
	relative, err := filepath.Rel(dir, path)
	if err != nil || relative == "." || filepath.IsAbs(relative) {
		return false
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}
