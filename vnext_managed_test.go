package main

import (
	"context"
	"os"
	"testing"
	"time"

	"unbound/engine"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
)

func TestVerifiedGrantRequiresSelectedRestoredVerifiedOutcome(t *testing.T) {
	service := newProductVNextService(nil, nil)
	target, public, err := normalizeVNextTarget("https://target.test/path?secret=value")
	if err != nil {
		t.Fatal(err)
	}
	result := autotunevnext.Result{Status: autotunevnext.StatusCompletedSelected, StateRestored: true, Backend: backendcap.Zapret2Windows, SelectedStrategyID: "strategy", SelectedFingerprint: "fingerprint", Experiments: []autotunevnext.CandidateExperiment{{StrategyID: "strategy", Fingerprint: "fingerprint", Outcome: autotunevnext.OutcomeVerifiedFixed}}}
	token := service.issueVerifiedGrant(result, target, public, nil)
	if token == "" {
		t.Fatal("verified result did not receive a grant")
	}
	grant, err := service.resolveGrant(token)
	if err != nil || grant.strategyID != "strategy" || grant.publicTarget != "https://target.test/path" {
		t.Fatalf("grant=%#v err=%v", grant, err)
	}
	result.StateRestored = false
	if token := service.issueVerifiedGrant(result, target, public, nil); token != "" {
		t.Fatal("unrestored result received a grant")
	}
	result.StateRestored = true
	result.Experiments[0].Outcome = autotunevnext.OutcomeRegressionObserved
	if token := service.issueVerifiedGrant(result, target, public, nil); token != "" {
		t.Fatal("non-verified result received a grant")
	}
}

func TestExpiredAndConsumedGrantFailClosed(t *testing.T) {
	service := newProductVNextService(nil, nil)
	service.grants["expired"] = verifiedSelectionGrant{token: "expired", expiresAt: time.Now().Add(-time.Second)}
	if _, err := service.resolveGrant("expired"); err == nil {
		t.Fatal("expired grant was accepted")
	}
	service.grants["used"] = verifiedSelectionGrant{token: "used", expiresAt: time.Now().Add(time.Minute), consumed: true}
	if _, err := service.resolveGrant("used"); err == nil {
		t.Fatal("consumed grant was accepted")
	}
}

func TestManagedStateRoundTripAndCorruptionFailClosed(t *testing.T) {
	dir := t.TempDir()
	restore := engine.SetConfigDirForTest(dir)
	defer restore()
	state := persistedVNextState{SchemaVersion: vNextManagedSchema, Enabled: true, Target: "https://target.test/path", StrategyID: "strategy", Fingerprint: "fingerprint", Backend: string(backendcap.Zapret2Windows), SavedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := saveVNextManagedState(state); err != nil {
		t.Fatal(err)
	}
	loaded, present, err := loadVNextManagedState()
	if err != nil || !present || loaded != state {
		t.Fatalf("loaded=%#v present=%t err=%v", loaded, present, err)
	}
	path, err := getVNextManagedStatePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadVNextManagedState(); err == nil {
		t.Fatal("corrupt state was accepted")
	}
	if err := clearVNextManagedState(); err != nil {
		t.Fatal(err)
	}
	if _, present, err := loadVNextManagedState(); err != nil || present {
		t.Fatalf("present=%t err=%v", present, err)
	}
}

func TestStartupRejectsCatalogFingerprintDrift(t *testing.T) {
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	state := persistedVNextState{SchemaVersion: vNextManagedSchema, Enabled: true, Target: "https://target.test/", StrategyID: "prod-tls-multisplit-1-v1", Fingerprint: "stale-fingerprint", Backend: string(backendcap.Zapret2Windows), SavedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := saveVNextManagedState(state); err != nil {
		t.Fatal(err)
	}
	if got := newProductVNextService(nil, nil).RevalidateSaved(context.Background()); got.State != "SAVED_STRATEGY_STALE" {
		t.Fatalf("state=%#v", got)
	}
}
