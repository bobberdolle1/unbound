package engine

import (
	"testing"
)

func TestDeriveRequiredLuaFunctions(t *testing.T) {
	args := []string{
		"--payload=tls_client_hello",
		"--lua-desync=hostfakesplit:midhost=midsld:repeats=2",
		"--lua-desync=multisplit:pos=1,midsld",
		"--filter-tcp=443",
	}

	fns := deriveRequiredLuaFunctions(args)
	if len(fns) != 2 {
		t.Fatalf("Expected 2 derived functions, got %d (%v)", len(fns), fns)
	}
	if fns[0] != "hostfakesplit" || fns[1] != "multisplit" {
		t.Errorf("Unexpected derived functions: %v", fns)
	}
}

func TestCheckCandidateCapabilitiesValidation(t *testing.T) {
	// Candidate requiring unknown/fake Lua function
	badCand := StrategyCandidate{
		ID:          "cand_bad_lua",
		Name:        "Unsupported Lua Candidate",
		Zapret2Args: []string{"--lua-desync=nonexistent_desync_xyz:opt=1"},
	}

	ok, reason := CheckCandidateCapabilities(badCand, nil)
	if ok {
		t.Fatal("Expected CheckCandidateCapabilities to return false for nonexistent Lua function")
	}
	if reason == "" {
		t.Fatal("Expected non-empty reason for failure")
	}

	// Candidate with valid known function
	goodCand := StrategyCandidate{
		ID:          "cand_good_lua",
		Name:        "Supported Lua Candidate",
		Zapret2Args: []string{"--lua-desync=hostfakesplit:repeats=2"},
	}

	ok, reason = CheckCandidateCapabilities(goodCand, nil)
	if !ok {
		t.Fatalf("Expected CheckCandidateCapabilities to return true for hostfakesplit, got reason: %s", reason)
	}
}
