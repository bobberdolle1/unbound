package engine

import (
	"testing"
)

func TestParseNetshTimestamps(t *testing.T) {
	// 1. English netsh output with RFC 1323 allowed, while other lines have enabled
	englishAllowed := `
TCP Global Parameters
----------------------------------------------
Receive-Side Scaling State          : enabled
Receive Window Auto-Tuning Level    : normal
Add-On Congestion Control Provider  : default
ECN Capability                      : disabled
RFC 1323 Timestamps                 : allowed
Initial RTO                         : 1000
Receive Segment Coalescing State    : enabled
Fast Open                           : enabled
`
	enabled, matched := parseNetshTimestamps(englishAllowed)
	if !matched {
		t.Fatal("Expected matched=true for English allowed output")
	}
	if !enabled {
		t.Fatal("Expected enabled=true for RFC 1323 allowed")
	}

	// 2. English netsh output with RFC 1323 disabled, while other lines have enabled!
	// CRITICAL REGRESSION: Old code checked strings.Contains(..., "enabled") and falsely passed!
	englishDisabled := `
TCP Global Parameters
----------------------------------------------
Receive-Side Scaling State          : enabled
Receive Window Auto-Tuning Level    : normal
Add-On Congestion Control Provider  : default
ECN Capability                      : disabled
RFC 1323 Timestamps                 : disabled
Initial RTO                         : 1000
Receive Segment Coalescing State    : enabled
Fast Open                           : enabled
`
	enabled, matched = parseNetshTimestamps(englishDisabled)
	if !matched {
		t.Fatal("Expected matched=true for English disabled output")
	}
	if enabled {
		t.Fatal("Expected enabled=false when RFC 1323 is disabled, despite unrelated enabled tokens")
	}

	// 3. Russian netsh output with allowed/enabled
	russianAllowed := `
Глобальные параметры TCP
----------------------------------------------
Состояние масштабирования на стороне приема : enabled
Временные метки RFC 1323                    : разрешены
Уровень автоподстройки окна приема          : normal
`
	enabled, matched = parseNetshTimestamps(russianAllowed)
	if !matched {
		t.Fatal("Expected matched=true for Russian allowed output")
	}
	if !enabled {
		t.Fatal("Expected enabled=true for Russian разрешены")
	}

	// 4. Russian netsh output with disabled
	russianDisabled := `
Глобальные параметры TCP
----------------------------------------------
Состояние масштабирования на стороне приема : enabled
Временные метки RFC 1323                    : отключено
Уровень автоподстройки окна приема          : normal
`
	enabled, matched = parseNetshTimestamps(russianDisabled)
	if !matched {
		t.Fatal("Expected matched=true for Russian disabled output")
	}
	if enabled {
		t.Fatal("Expected enabled=false for Russian отключено")
	}

	// 5. Unrelated output without 1323
	unrelated := `
Ethernet adapter:
   Status : enabled
`
	enabled, matched = parseNetshTimestamps(unrelated)
	if matched {
		t.Fatal("Expected matched=false when 1323 line is absent")
	}
	if enabled {
		t.Fatal("Expected enabled=false for unmatched output")
	}
}

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
