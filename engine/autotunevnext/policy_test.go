package autotunevnext

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"unbound/engine/observatory"
	"unbound/engine/strategyir"
)

func verifiedSequence(prefix string) []observatory.ObservationResult {
	return []observatory.ObservationResult{
		observation(prefix+"-before", false, "192.0.2.1", "https://blocked.test/"),
		observation(prefix+"-active", true, "192.0.2.1", "https://blocked.test/"),
		observation(prefix+"-after", false, "192.0.2.1", "https://blocked.test/"),
	}
}

func TestSafetyPolicyDeterministicallyChoosesVerifiedCandidates(t *testing.T) {
	cases := []struct {
		name        string
		left, right strategyir.Strategy
		want        string
	}{
		{"target only first", func() strategyir.Strategy { s := tlsStrategy("target"); return s }(), func() strategyir.Strategy { s := tlsStrategy("broad"); s.Safety.TargetOnly = false; return s }(), "target"},
		{"nonexperimental first", func() strategyir.Strategy { s := tlsStrategy("stable"); return s }(), func() strategyir.Strategy { s := tlsStrategy("experimental"); s.Safety.Experimental = true; return s }(), "stable"},
		{"steam safe first", func() strategyir.Strategy { s := tlsStrategy("steam-safe"); return s }(), func() strategyir.Strategy { s := tlsStrategy("steam-risk"); s.Safety.MayAffectSteam = true; return s }(), "steam-safe"},
		{"lower aggressiveness last tie break", func() strategyir.Strategy { s := tlsStrategy("low"); return s }(), func() strategyir.Strategy { s := tlsStrategy("high"); s.Safety.Aggressiveness = "HIGH"; return s }(), "low"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results := []observatory.ObservationResult{observation("baseline", false, "192.0.2.1", "https://blocked.test/")}
			results = append(results, verifiedSequence("left")...)
			results = append(results, verifiedSequence("right")...)
			req := request(tc.left, tc.right)
			req.Policy.MaxCandidates = 2
			result := Run(context.Background(), req, &fakeObserver{results: results}, &fakeExecutor{}, supportedPreflight{PreflightSupported}, fakeAssets{})
			if result.Status != StatusCompletedSelected || result.SelectedStrategyID != tc.want {
				t.Fatalf("selected=%q result=%#v", result.SelectedStrategyID, result)
			}
		})
	}
}

func TestDeterminismFingerprintAndDurationBudget(t *testing.T) {
	left, right := tlsStrategy("left"), tlsStrategy("right")
	sequence := func() []observatory.ObservationResult {
		values := []observatory.ObservationResult{observation("base", false, "192.0.2.1", "https://blocked.test/")}
		values = append(values, verifiedSequence("left")...)
		values = append(values, verifiedSequence("right")...)
		return values
	}
	before, err := strategyir.Fingerprint(left)
	if err != nil {
		t.Fatal(err)
	}
	first := Run(context.Background(), request(left, right), &fakeObserver{results: sequence()}, &fakeExecutor{}, supportedPreflight{PreflightSupported}, fakeAssets{})
	second := Run(context.Background(), request(right, left), &fakeObserver{results: sequence()}, &fakeExecutor{}, supportedPreflight{PreflightSupported}, fakeAssets{})
	after, err := strategyir.Fingerprint(left)
	if err != nil || before != after {
		t.Fatalf("strategy mutated: %q %q %v", before, after, err)
	}
	if first.SelectedStrategyID != second.SelectedStrategyID || first.SelectedFingerprint != second.SelectedFingerprint {
		t.Fatalf("permutation changed selection: %#v %#v", first, second)
	}

	req := request(left, right)
	req.Policy.MaxDuration = 10 * time.Millisecond
	observer := &fakeObserver{results: []observatory.ObservationResult{observation("base", false, "192.0.2.1", "https://blocked.test/"), observation("before", false, "192.0.2.1", "https://blocked.test/")}, errAt: 2, block: true}
	result := Run(context.Background(), req, observer, &fakeExecutor{}, supportedPreflight{PreflightSupported}, fakeAssets{})
	if !result.StateRestored || len(result.Experiments) != 2 {
		t.Fatalf("duration budget was not recorded transactionally: %#v", result)
	}
	if result.Experiments[1].Outcome != OutcomeNotRunBudget {
		t.Fatalf("duration budget launched another candidate: %#v", result.Experiments)
	}
}

func TestDeactivationFailureRestoresAndPreventsSelection(t *testing.T) {
	executor := &fakeExecutor{deactivateErr: errors.New("deactivate")}
	result := Run(context.Background(), request(tlsStrategy("tls")), &fakeObserver{results: []observatory.ObservationResult{observation("base", false, "192.0.2.1", "https://blocked.test/"), observation("before", false, "192.0.2.1", "https://blocked.test/"), observation("active", true, "192.0.2.1", "https://blocked.test/")}}, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if result.Status != StatusLifecycleFailed || result.SelectedStrategyID != "" || !result.StateRestored || !slices.Contains(executor.calls, "restore") {
		t.Fatalf("deactivation lifecycle=%#v calls=%v", result, executor.calls)
	}
}

func TestPolicyDefaultsAreExplicitAndBounded(t *testing.T) {
	policy := Policy{}.normalized()
	if policy.MaxCandidates <= 0 || policy.MaxDuration <= 0 || policy.PerObservationTimeout <= 0 {
		t.Fatalf("unbounded policy: %#v", policy)
	}
	if policy.MaxDuration > 3*time.Minute {
		t.Fatalf("unreasonable default duration: %s", policy.MaxDuration)
	}
}
