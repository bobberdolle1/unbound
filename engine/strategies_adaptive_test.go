package engine

import (
	"strings"
	"testing"
)

func TestGetAdaptiveProfile(t *testing.T) {
	tempLua := t.TempDir()
	tempLists := t.TempDir()

	prof := GetAdaptiveProfile(tempLua, tempLists)
	if prof.Name != "Adaptive (Experimental)" {
		t.Errorf("Expected profile name 'Adaptive (Experimental)', got %s", prof.Name)
	}

	joinedArgs := strings.Join(prof.Args, " ")

	// Circular flag present
	if !strings.Contains(joinedArgs, "circular:fails=3:time=60") {
		t.Error("Missing circular orchestrator flag in adaptive profile args")
	}

	// Scoped capture flag present
	if !strings.Contains(joinedArgs, "--in-range=-s4096") {
		t.Error("Missing scoped capture flag --in-range=-s4096")
	}

	// Steam safe exclusions present
	if !strings.Contains(joinedArgs, "steam-web-exclude.txt") {
		t.Error("Missing steam-web-exclude.txt in adaptive profile args")
	}
}

func TestAdaptiveStateTracker(t *testing.T) {
	tracker := GetAdaptiveStateTracker()
	tracker.ResetState()

	// Initial state empty
	if states := tracker.GetHostStates(); len(states) != 0 {
		t.Fatalf("Expected 0 states after reset, got %d", len(states))
	}

	// Record success
	tracker.RecordSuccess("youtube.com", 1, "HostFakeSplit")
	states := tracker.GetHostStates()
	if len(states) != 1 {
		t.Fatalf("Expected 1 state, got %d", len(states))
	}
	if states[0].Host != "youtube.com" || states[0].StrategyName != "HostFakeSplit" {
		t.Errorf("Unexpected state: %+v", states[0])
	}

	// Record failure
	tracker.RecordFailure("youtube.com", 2, "MultiSplit")
	states = tracker.GetHostStates()
	if states[0].StrategyIndex != 2 || states[0].FailureCount != 1 || states[0].Confidence != "low" {
		t.Errorf("Unexpected state after failure: %+v", states[0])
	}

	// Reset
	tracker.ResetState()
	if states := tracker.GetHostStates(); len(states) != 0 {
		t.Errorf("Expected 0 states after reset, got %d", len(states))
	}
}

func TestParseAdaptiveLogEventRealStream(t *testing.T) {
	tracker := GetAdaptiveStateTracker()
	tracker.ResetState()

	// 1. Host key line from winws2 output
	ParseAdaptiveLogEvent("DLOG: automate: host record key 'autostate.circular.discord.com'")

	// 2. Rotation line
	ParseAdaptiveLogEvent("DLOG: circular: rotate strategy to 2")

	states := tracker.GetHostStates()
	if len(states) != 1 {
		t.Fatalf("Expected 1 host state, got %d", len(states))
	}
	if states[0].Host != "discord.com" {
		t.Errorf("Host = %s; want discord.com", states[0].Host)
	}
	if states[0].StrategyIndex != 2 || states[0].StrategyName != "MultiSplit (midsld)" {
		t.Errorf("Strategy = (%d, %s); want (2, MultiSplit (midsld))", states[0].StrategyIndex, states[0].StrategyName)
	}

	// 3. Success line
	ParseAdaptiveLogEvent("DLOG: automate: success detected")
	states = tracker.GetHostStates()
	if states[0].FailureCount != 0 {
		t.Errorf("Expected failure count 0 after success, got %d", states[0].FailureCount)
	}
}
