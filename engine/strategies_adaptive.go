package engine

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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
	Threshold     int       `json:"threshold,omitempty"`
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

// RecordInit registers initial baseline tracking for a host on strategy #1.
func (t *AdaptiveStateTracker) RecordInit(host string, strategyIndex int, strategyName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.states[host]; !exists {
		t.states[host] = &AdaptiveHostState{
			Host:          host,
			StrategyIndex: strategyIndex,
			StrategyName:  strategyName,
			Confidence:    "medium",
			LastSuccess:   time.Now(),
		}
	}
}

// RecordSuccess updates the host record on successful transmission past sequence thresholds.
// Keeps the currently working strategy intact (does NOT reset to #1).
func (t *AdaptiveStateTracker) RecordSuccess(host string, strategyIndex int, strategyName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	state, ok := t.states[host]
	if !ok {
		state = &AdaptiveHostState{
			Host:          host,
			StrategyIndex: strategyIndex,
			StrategyName:  strategyName,
			Confidence:    "medium",
		}
		t.states[host] = state
	}

	state.LastSuccess = time.Now()
	state.FailureCount = 0
	if strategyIndex > 0 && strategyName != "" {
		state.StrategyIndex = strategyIndex
		state.StrategyName = strategyName
	}
	state.Confidence = "high"
}

// RecordRotation records that circular orchestration shifted to a new strategy after failure threshold.
func (t *AdaptiveStateTracker) RecordRotation(host string, newStrategyIndex int, newStrategyName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	state, ok := t.states[host]
	if !ok {
		state = &AdaptiveHostState{
			Host: host,
		}
		t.states[host] = state
	}

	state.LastFailure = time.Now()
	state.FailureCount = 0
	state.StrategyIndex = newStrategyIndex
	state.StrategyName = newStrategyName
	state.Confidence = "medium"
}

// RecordFailure increments failure counter and notes degradation.
func (t *AdaptiveStateTracker) RecordFailure(host string, strategyIndex int, strategyName string) {
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

	state.LastFailure = time.Now()
	state.FailureCount++
	if strategyIndex > 0 && strategyName != "" {
		state.StrategyIndex = strategyIndex
		state.StrategyName = strategyName
	}
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

// RecordFailureDetected updates failure counter without forcing rotation until threshold is reached.
func (t *AdaptiveStateTracker) RecordFailureDetected(host string, strategyIndex int, strategyName string, count, threshold int) {
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

	state.LastFailure = time.Now()
	state.FailureCount = count
	state.Threshold = threshold
	if strategyIndex > 0 && strategyName != "" {
		state.StrategyIndex = strategyIndex
		state.StrategyName = strategyName
	}
	state.Confidence = "low"
}

// GetAdaptiveProfile generates the command-line arguments for the Adaptive (Experimental) profile.
// Uses bundled zapret-auto.lua with circular orchestration, event bridge, and directional capture.
func GetAdaptiveProfile(luaDir, listsDir string) Profile {
	autoLua := filepath.ToSlash(filepath.Join(luaDir, "zapret-auto.lua"))
	eventsLua := filepath.ToSlash(filepath.Join(luaDir, "unbound_adaptive_events.lua"))

	// 4-stage circular orchestration chain:
	// Strategy 1: HostFakeSplit (midsld)
	// Strategy 2: MultiSplit (midsld)
	// Strategy 3: Fake TLS
	// Strategy 4: MultiDisorder (final)
	args := []string{
		// Directional capture: inbound and outbound TCP on 80,443 within initial 4KB sequence for RST/redirect detection
		"--wf-tcp-in=80,443",
		"--wf-tcp-out=80,443",
		"--filter-tcp=80,443",
		"--in-range=-s4096",
		"--lua-init=@" + autoLua,
		"--lua-init=@" + eventsLua,
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

var adaptiveStrategyNames = map[int]string{
	1: "HostFakeSplit (midsld)",
	2: "MultiSplit (midsld)",
	3: "Fake TLS",
	4: "MultiDisorder (final)",
}

// ParseAdaptiveLogEvent inspects winws2 stdout/stderr stream lines and feeds real events into AdaptiveStateTracker.
func ParseAdaptiveLogEvent(logLine string) {
	if logLine == "" {
		return
	}

	// Priority 1: Structured Machine-Readable UNBOUND Event Bridge
	// Format: [UNBOUND_EVENT] adaptive host=<host> event=<init|rotate|success|failure> strategy=<N>
	if idx := strings.Index(logLine, "[UNBOUND_EVENT] adaptive"); idx != -1 {
		part := strings.TrimSpace(logLine[idx+len("[UNBOUND_EVENT] adaptive"):])
		fields := strings.Fields(part)
		kv := make(map[string]string)
		for _, f := range fields {
			if eq := strings.Index(f, "="); eq != -1 {
				kv[f[:eq]] = f[eq+1:]
			}
		}

		host := kv["host"]
		if host == "" || host == "unknown" {
			return
		}
		event := kv["event"]
		stratNum := 1
		if s, ok := kv["strategy"]; ok && len(s) > 0 {
			if n := int(s[0] - '0'); n >= 1 && n <= 4 {
				stratNum = n
			}
		}
		stratName := adaptiveStrategyNames[stratNum]
		tracker := GetAdaptiveStateTracker()
		switch event {
		case "init":
			tracker.RecordInit(host, stratNum, stratName)
		case "rotate":
			tracker.RecordRotation(host, stratNum, stratName)
			GetLogger().Infof("Adaptive", "[ADAPTIVE] circular rotated strategy to #%d (%s) for %s", stratNum, stratName, host)
		case "success", "success_detected":
			tracker.RecordSuccess(host, stratNum, stratName)
			GetLogger().Infof("Adaptive", "[ADAPTIVE] host %s success confirmed on strategy #%d (%s)", host, stratNum, stratName)
		case "failure", "failure_detected":
			count, _ := strconv.Atoi(kv["count"])
			threshold, _ := strconv.Atoi(kv["threshold"])
			if threshold == 0 {
				threshold = 3
			}
			tracker.RecordFailureDetected(host, stratNum, stratName, count, threshold)
			GetLogger().Infof("Adaptive", "[ADAPTIVE] host %s failure detected on strategy #%d (%s) [%d/%d]",
				host, stratNum, stratName, count, threshold)
		}
		return
	}
	// Priority 2: Fallback Scraper for legacy or manual --debug=1 upstream logs
	if idx := strings.Index(logLine, "circular: rotate strategy to "); idx != -1 {
		part := strings.TrimSpace(logLine[idx+len("circular: rotate strategy to "):])
		if len(part) > 0 {
			stratNum := int(part[0] - '0')
			if stratNum >= 1 && stratNum <= 4 {
				name := adaptiveStrategyNames[stratNum]
				GetAdaptiveStateTracker().RecordRotation("active target", stratNum, name)
			}
		}
	}
}
