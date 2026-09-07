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
	// Directional capture flags present
	if !strings.Contains(joinedArgs, "--wf-tcp-in=80,443") || !strings.Contains(joinedArgs, "--wf-tcp-out=80,443") {
		t.Error("Missing directional capture flags --wf-tcp-in/out in adaptive profile args")
	}

	// Structured event bridge script present
	if !strings.Contains(joinedArgs, "unbound_adaptive_events.lua") {
		t.Error("Missing unbound_adaptive_events.lua in adaptive profile args")
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

func TestParseAdaptiveStructuredEventBridge(t *testing.T) {
	tracker := GetAdaptiveStateTracker()
	tracker.ResetState()

	// 1. Initial state
	ParseAdaptiveLogEvent("[UNBOUND_EVENT] adaptive host=youtube.com event=init strategy=1")
	states := tracker.GetHostStates()
	if len(states) != 1 || states[0].StrategyIndex != 1 {
		t.Fatalf("Expected strategy 1 on init, got: %+v", states)
	}

	// 2. Rotations to 2, then 3
	ParseAdaptiveLogEvent("[UNBOUND_EVENT] adaptive host=youtube.com event=rotate strategy=2")
	ParseAdaptiveLogEvent("[UNBOUND_EVENT] adaptive host=youtube.com event=rotate strategy=3")
	states = tracker.GetHostStates()
	if states[0].StrategyIndex != 3 || states[0].StrategyName != "Fake TLS" {
		t.Fatalf("Expected strategy 3 (Fake TLS) after rotation, got: %+v", states[0])
	}

	// 3. Success on strategy 3 - MUST NOT reset to strategy 1!
	ParseAdaptiveLogEvent("[UNBOUND_EVENT] adaptive host=youtube.com event=success strategy=3")
	states = tracker.GetHostStates()
	if states[0].StrategyIndex != 3 {
		t.Errorf("BUG REGRESSION: Success on strategy 3 reset to %d", states[0].StrategyIndex)
	}
	if states[0].Confidence != "high" {
		t.Errorf("Expected high confidence after success, got %s", states[0].Confidence)
	}
	if states[0].FailureCount != 0 {
		t.Errorf("Expected failure count 0 after success, got %d", states[0].FailureCount)
	}
}

func TestAdaptiveConcurrentHostsNoRace(t *testing.T) {
	tracker := GetAdaptiveStateTracker()
	tracker.ResetState()

	// Interleaved events for two different hosts
	ParseAdaptiveLogEvent("[UNBOUND_EVENT] adaptive host=discord.com event=rotate strategy=2")
	ParseAdaptiveLogEvent("[UNBOUND_EVENT] adaptive host=youtube.com event=rotate strategy=4")
	ParseAdaptiveLogEvent("[UNBOUND_EVENT] adaptive host=discord.com event=success strategy=2")

	states := tracker.GetHostStates()
	if len(states) != 2 {
		t.Fatalf("Expected 2 host states, got %d", len(states))
	}

	stateMap := make(map[string]AdaptiveHostState)
	for _, s := range states {
		stateMap[s.Host] = s
	}

	if d, ok := stateMap["discord.com"]; !ok || d.StrategyIndex != 2 || d.Confidence != "high" {
		t.Errorf("discord.com state corrupted: %+v", d)
	}
	if y, ok := stateMap["youtube.com"]; !ok || y.StrategyIndex != 4 {
		t.Errorf("youtube.com state corrupted: %+v", y)
	}
}
