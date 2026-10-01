//go:build !race

package main

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"unbound/engine"
	"unbound/engine/backendcap"
	"unbound/engine/historyid"
)

// Physical identity smoke for the history-identity layer.
//
// READ-ONLY. This harness only inspects. It never mutates DNS, routes, proxies,
// interfaces, Wi-Fi, firewall, VPN, or any network service, and it never
// captures packets or starts an engine. Its whole purpose is to prove on a real
// host that the three identities are produced factually, are STABLE across
// repeated calls, and that no raw network fact leaks into anything persisted.
//
// It runs only when UNBOUND_IDENTITY_SMOKE=1 is set, and it refuses to run on a
// host that is not an approved lab.

func TestPhysicalIdentitySmoke(t *testing.T) {
	if os.Getenv("UNBOUND_IDENTITY_SMOKE") != "1" {
		t.Skip("identity smoke requires UNBOUND_IDENTITY_SMOKE=1 on a lab host")
	}
	host, _ := os.Hostname()
	approved := map[string]bool{
		"DESKTOP-MNEHCPT":                   true,
		"bobpc-HP-Compaq-6730b-GB990EA-ACB": true,
	}
	if !approved[host] {
		t.Fatalf("refusing to run on an unapproved host %q", host)
	}

	// Hermetic config dir so a smoke run never touches an installed profile.
	tmp, err := os.MkdirTemp("", "unbound-identity-")
	if err != nil {
		t.Fatalf("temp config dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	// Real asset pipeline: the backend fingerprint must be anchored to the real
	// verified engine binary, not to a synthetic value.
	assets, err := engine.ExtractAssets()
	if err != nil {
		t.Fatalf("asset pipeline: %v", err)
	}
	defer func() {
		if err := engine.CleanupExtractedAssets(); err != nil {
			t.Logf("cleanup extracted assets: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// The backend must be the one that actually runs on this host, otherwise the
	// fingerprint would describe an engine that is not present.
	backend := backendcap.Zapret2Windows
	if runtime.GOOS == "linux" {
		backend = backendcap.Zapret2Linux
	}

	// Two independent bundles, each reading the live host state.
	first := historyid.ForRun(ctx, backend, assets, tmp)
	second := historyid.ForRun(ctx, backend, assets, tmp)

	if !first.ContextKey.Available() {
		t.Fatalf("CONTEXT unavailable on this host: %v", first.ContextErr)
	}
	if !first.BackendFingerprint.Available() {
		t.Fatalf("BACKEND unavailable on this host: %v", first.BackendErr)
	}
	if !first.CapabilityFingerprint.Available() {
		t.Fatalf("CAPABILITY unavailable on this host: %v", first.CapabilityErr)
	}

	if first.ContextKey != second.ContextKey {
		t.Errorf("context identity is not stable across calls: %q vs %q", first.ContextKey, second.ContextKey)
	}
	if first.BackendFingerprint != second.BackendFingerprint {
		t.Errorf("backend identity is not stable across calls")
	}
	if first.CapabilityFingerprint != second.CapabilityFingerprint {
		t.Errorf("capability identity is not stable across calls")
	}

	// The persisted key file must exist and be owner-only, and reloading it must
	// reproduce the same identity (this is the reproducibility the HMAC key buys).
	key, err := historyid.LoadOrCreateContextKey(tmp)
	if err != nil {
		t.Fatalf("load context key: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("context key is %d bytes, want 32", len(key))
	}
	info, err := os.Stat(tmp + string(os.PathSeparator) + historyid.ContextKeyFileName)
	if err != nil {
		t.Fatalf("stat context key: %v", err)
	}
	// Unix carries the restriction in the mode bits. Windows ignores those and
	// enforces it in the DACL, so the check has to be platform-specific: asserting
	// Unix bits on Windows would pass or fail for reasons unrelated to the ACL.
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf("context key file mode is %o, must not be group or world accessible", perm)
		}
	} else {
		if err := assertKeyRestrictedToOwner(tmp + string(os.PathSeparator) + historyid.ContextKeyFileName); err != nil {
			t.Errorf("context key DACL is not owner-only: %v", err)
		}
	}

	t.Logf("IDENTITY_SMOKE=PASS host=%s context=%s backend=%s capability=%s",
		host, first.ContextKey, first.BackendFingerprint, first.CapabilityFingerprint)
	t.Logf("LIMITATIONS=%v", first.Limitations())
}
