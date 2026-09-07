package engine

import (
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// AdaptiveHostState tracks the current strategy and confidence for a domain in Adaptive mode.
type AdaptiveHostState struct {
	Host          string    `json:"host"`
	StrategyName  string    `json:"strategyName"`
	StrategyIndex int       `json:"strategyIndex"`
	Confidence    string    `json:"confidence"` // "high", "medium", "low"
	FailureCount  int       `json:"failureCount"`
	LastSuccess   time.Time `json:"lastSuccess"`
	LastFailure   time.Time `json:"lastFailure,omitempty"`
}

// AdaptiveStateTracker manages inspection and resetting of the per-host adaptive state.
type AdaptiveStateTracker struct {
	mu     sync.RWMutex
	states map[string]*AdaptiveHostState
}

var (
	globalAdaptiveTracker     *AdaptiveStateTracker
	globalAdaptiveTrackerOnce sync.Once
)

// GetAdaptiveStateTracker returns the singleton per-host adaptive state tracker.
func GetAdaptiveStateTracker() *AdaptiveStateTracker {
	globalAdaptiveTrackerOnce.Do(func() {
		globalAdaptiveTracker = &AdaptiveStateTracker{
			states: make(map[string]*AdaptiveHostState),
		}
	})
	return globalAdaptiveTracker
}

// RecordSuccess updates the host record when a connection successfully transmits past sequence thresholds.
func (t *AdaptiveStateTracker) RecordSuccess(host string, strategyIndex int, strategyName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	state, ok := t.states[host]
	if !ok {
		state = &AdaptiveHostState{
			Host:          host,
			StrategyIndex: strategyIndex,
			StrategyName:  strategyName,
			Confidence:    "low",
		}
		t.states[host] = state
	}

	state.LastSuccess = time.Now()
	state.FailureCount = 0

	// Boost confidence after repeated success
	if state.Confidence == "low" {
		state.Confidence = "medium"
	} else if state.Confidence == "medium" {
		state.Confidence = "high"
	}
}

// RecordFailure increments failure counter and notes degradation.
func (t *AdaptiveStateTracker) RecordFailure(host string, newStrategyIndex int, newStrategyName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	state, ok := t.states[host]
	if !ok {
		state = &AdaptiveHostState{
			Host:          host,
			StrategyIndex: newStrategyIndex,
			StrategyName:  newStrategyName,
			Confidence:    "low",
		}
		t.states[host] = state
	}

	state.LastFailure = time.Now()
	state.FailureCount++
	state.StrategyIndex = newStrategyIndex
	state.StrategyName = newStrategyName
	state.Confidence = "low"
}

// GetHostStates returns a sorted list of all active host states.
func (t *AdaptiveStateTracker) GetHostStates() []AdaptiveHostState {
	t.mu.RLock()
	defer t.mu.RUnlock()

	list := make([]AdaptiveHostState, 0, len(t.states))
	for _, s := range t.states {
		list = append(list, *s)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].LastSuccess.After(list[j].LastSuccess)
	})
	return list
}

// ResetState clears all transient per-host state.
func (t *AdaptiveStateTracker) ResetState() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.states = make(map[string]*AdaptiveHostState)
	GetLogger().Info("Adaptive", "[ADAPTIVE] per-host adaptive state reset")
}
// GetAdaptiveProfile generates the command-line arguments for the Adaptive (Experimental) profile.
// Uses bundled zapret-auto.lua with circular orchestration and scoped inbound capture.
func GetAdaptiveProfile(luaDir, listsDir string) Profile {
	autoLua := filepath.Join(luaDir, "zapret-auto.lua")

	// 4-stage circular orchestration chain:
	// Strategy 1: HostFakeSplit (midsld)
	// Strategy 2: MultiSplit (midsld)
	// Strategy 3: Fake TLS
	// Strategy 4: MultiDisorder (final)
	args := []string{
		// Scoped capture: inbound TCP only on 80,443 within initial 4KB sequence for RST/redirect detection
		"--wf-tcp=80,443",
		"--filter-tcp=80,443",
		"--in-range=-s4096",
		"--lua-init=@" + autoLua,
		"--payload=tls_client_hello",
		"--lua-desync=circular:fails=3:time=60",
		"--lua-desync=hostfakesplit:midhost=midsld:repeats=2:strategy=1",
		"--lua-desync=multisplit:pos=midsld:strategy=2",
		"--lua-desync=fake:repeats=2:strategy=3",
		"--lua-desync=multidisorder:pos=midsld:strategy=4:final",
	}

	// Apply Steam and Valve safety exclusions so game traffic is protected
	safeArgs := steamSafeArgs(args, listsDir)

	return Profile{
		Name: "Adaptive (Experimental)",
		Args: safeArgs,
	}
}
