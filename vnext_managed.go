package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"unbound/engine"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/strategyir"
)

const (
	vNextManagedStateFile = "autotune_vnext_state.json"
	vNextManagedSchema    = 1
	vNextGrantTTL         = 10 * time.Minute
)

type verifiedSelectionGrant struct {
	token        string
	target       autotunevnext.Target
	publicTarget string
	controls     []autotunevnext.Target
	strategyID   string
	fingerprint  string
	backend      backendcap.Backend
	createdAt    time.Time
	expiresAt    time.Time
	consumed     bool
}

type managedVNextActivation struct {
	activation *autotunevnext.ManagedActivation
	grant      verifiedSelectionGrant
}

// AutoTuneVNextManagedStatus exposes only logical/redacted managed state.
type AutoTuneVNextManagedStatus struct {
	State             string `json:"state"`
	Active            bool   `json:"active"`
	NeedsRevalidation bool   `json:"needs_revalidation"`
	Target            string `json:"target,omitempty"`
	StrategyID        string `json:"strategy_id,omitempty"`
	Fingerprint       string `json:"fingerprint,omitempty"`
	Backend           string `json:"backend,omitempty"`
}

type persistedVNextState struct {
	SchemaVersion int    `json:"schema_version"`
	Enabled       bool   `json:"enabled"`
	Target        string `json:"target"`
	StrategyID    string `json:"strategy_id"`
	Fingerprint   string `json:"fingerprint"`
	Backend       string `json:"backend"`
	SavedAt       string `json:"saved_at"`
}

func getVNextManagedStatePath() (string, error) {
	dir, err := engine.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, vNextManagedStateFile), nil
}

func loadVNextManagedState() (persistedVNextState, bool, error) {
	path, err := getVNextManagedStatePath()
	if err != nil {
		return persistedVNextState{}, false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return persistedVNextState{}, false, nil
	}
	if err != nil {
		return persistedVNextState{}, false, err
	}
	var state persistedVNextState
	if err := json.Unmarshal(data, &state); err != nil {
		return persistedVNextState{}, false, fmt.Errorf("corrupt managed vNext state: %w", err)
	}
	if state.SchemaVersion != vNextManagedSchema || !state.Enabled || state.Target == "" || state.StrategyID == "" || state.Fingerprint == "" || state.Backend == "" {
		return persistedVNextState{}, false, errors.New("invalid managed vNext state")
	}
	if _, _, err := normalizeVNextTarget(state.Target); err != nil {
		return persistedVNextState{}, false, fmt.Errorf("invalid managed target: %w", err)
	}
	if _, err := time.Parse(time.RFC3339Nano, state.SavedAt); err != nil {
		return persistedVNextState{}, false, fmt.Errorf("invalid managed saved_at: %w", err)
	}
	return state, true, nil
}

func saveVNextManagedState(state persistedVNextState) error {
	path, err := getVNextManagedStatePath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".autotune-vnext-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}

func clearVNextManagedState() error {
	path, err := getVNextManagedStatePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func secureGrantToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *productVNextService) invalidateGrantsLocked() {
	for token := range s.grants {
		delete(s.grants, token)
	}
}

func (s *productVNextService) issueVerifiedGrant(result autotunevnext.Result, target autotunevnext.Target, public string, controls []autotunevnext.Target) string {
	if result.Status != autotunevnext.StatusCompletedSelected || !result.StateRestored || result.SelectedStrategyID == "" || result.SelectedFingerprint == "" {
		return ""
	}
	verified := false
	for _, experiment := range result.Experiments {
		if experiment.StrategyID == result.SelectedStrategyID && experiment.Fingerprint == result.SelectedFingerprint && experiment.Outcome == autotunevnext.OutcomeVerifiedFixed {
			verified = true
			break
		}
	}
	if !verified {
		return ""
	}
	token, err := secureGrantToken()
	if err != nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invalidateGrantsLocked()
	now := time.Now()
	s.grants[token] = verifiedSelectionGrant{token: token, target: target, publicTarget: public, controls: append([]autotunevnext.Target(nil), controls...), strategyID: result.SelectedStrategyID, fingerprint: result.SelectedFingerprint, backend: result.Backend, createdAt: now, expiresAt: now.Add(vNextGrantTTL)}
	return token
}

func (s *productVNextService) InvalidateGrants() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invalidateGrantsLocked()
}

func (s *productVNextService) resolveGrant(token string) (verifiedSelectionGrant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	grant, ok := s.grants[token]
	if !ok || token == "" || grant.consumed || time.Now().After(grant.expiresAt) {
		if ok {
			delete(s.grants, token)
		}
		return verifiedSelectionGrant{}, errors.New("APPLY_TOKEN_INVALID")
	}
	if s.active != nil {
		return verifiedSelectionGrant{}, errors.New("OPERATION_CONFLICT")
	}
	return grant, nil
}

func (s *productVNextService) Apply(ctx context.Context, token string) AutoTuneVNextManagedStatus {
	grant, err := s.resolveGrant(token)
	if err != nil {
		return AutoTuneVNextManagedStatus{State: "NOT_APPLIED"}
	}
	catalog, err := productionVNextStrategyCatalog(hostnameForVNextTarget(grant.target))
	if err != nil {
		return AutoTuneVNextManagedStatus{State: "NOT_APPLIED"}
	}
	var strategy strategyir.Strategy
	for _, candidate := range catalog {
		if candidate.ID == grant.strategyID {
			strategy = candidate
			break
		}
	}
	if strategy.ID == "" {
		return AutoTuneVNextManagedStatus{State: "SAVED_STRATEGY_STALE"}
	}
	fingerprint, err := strategyir.Fingerprint(strategy)
	if err != nil || fingerprint != grant.fingerprint {
		return AutoTuneVNextManagedStatus{State: "SAVED_STRATEGY_STALE"}
	}
	adapter := newProductRuntimeProvider(s.manager)
	if err := adapter.validateRestorableState(); err != nil {
		return AutoTuneVNextManagedStatus{State: "NOT_APPLIED"}
	}
	runtimeBinding, err := s.deps.newRuntime(adapter, s.assets, logProductVNextPhysicalEvent)
	if err != nil {
		return AutoTuneVNextManagedStatus{State: managedRuntimeFailureState(err)}
	}
	if runtimeBinding.backend != grant.backend {
		return AutoTuneVNextManagedStatus{State: "SAVED_STRATEGY_STALE"}
	}
	resolver, err := s.deps.newResolver(s.assets)
	if err != nil {
		return AutoTuneVNextManagedStatus{State: "NOT_APPLIED"}
	}
	activation, err := autotunevnext.ApplyVerified(ctx, autotunevnext.ManagedRequest{Target: grant.target, Controls: grant.controls, Strategy: strategy, Fingerprint: grant.fingerprint, Backend: runtimeBinding.backend, NetworkLabel: "product-autotune-vnext"}, s.deps.observer, runtimeBinding.executor, runtimeBinding.preflight, resolver)
	if err != nil {
		if errors.Is(err, autotunevnext.ErrManagedStateRestoreFailed) {
			status := managedStatusForGrant("STATE_RESTORE_FAILED", false, grant)
			s.mu.Lock()
			if activation != nil {
				s.active = &managedVNextActivation{activation: activation, grant: grant}
			}
			s.fault = status
			s.dormant = AutoTuneVNextManagedStatus{}
			s.mu.Unlock()
			return status
		}
		return AutoTuneVNextManagedStatus{State: "NOT_APPLIED"}
	}
	state := persistedVNextState{SchemaVersion: vNextManagedSchema, Enabled: true, Target: grant.publicTarget, StrategyID: grant.strategyID, Fingerprint: grant.fingerprint, Backend: string(runtimeBinding.backend), SavedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := saveVNextManagedState(state); err != nil {
		if revertErr := activation.Revert(context.Background()); revertErr != nil {
			status := managedStatusForGrant("STATE_RESTORE_FAILED", false, grant)
			s.mu.Lock()
			s.active = &managedVNextActivation{activation: activation, grant: grant}
			s.fault = status
			s.dormant = AutoTuneVNextManagedStatus{}
			s.mu.Unlock()
			return status
		}
		return managedStatusForGrant("PERSISTENCE_UPDATE_FAILED", false, grant)
	}
	s.mu.Lock()
	if current, ok := s.grants[token]; ok {
		current.consumed = true
		s.grants[token] = current
	}
	s.active = &managedVNextActivation{activation: activation, grant: grant}
	s.fault = AutoTuneVNextManagedStatus{}
	s.dormant = AutoTuneVNextManagedStatus{}
	s.mu.Unlock()
	return managedStatusForGrant("APPLIED", true, grant)
}

// DeactivateManaged distinguishes network restoration from persisted intent.
// preserveIntent is used by shutdown and bounded health revalidation; explicit
// user and legacy takeover reverts pass false.
func (s *productVNextService) DeactivateManaged(ctx context.Context, preserveIntent bool) AutoTuneVNextManagedStatus {
	s.mu.Lock()
	active := s.active
	s.mu.Unlock()
	if active != nil {
		if err := active.activation.Revert(ctx); err != nil {
			status := managedStatusForGrant("STATE_RESTORE_FAILED", false, active.grant)
			s.mu.Lock()
			if s.active == active {
				s.fault = status
			}
			s.mu.Unlock()
			return status
		}
		s.mu.Lock()
		if s.active == active {
			s.active = nil
		}
		s.fault = AutoTuneVNextManagedStatus{}
		s.invalidateGrantsLocked()
		s.mu.Unlock()
	}
	if preserveIntent {
		if active != nil {
			status := managedStatusForGrant("SAVED_REVALIDATION_PENDING", false, active.grant)
			status.NeedsRevalidation = true
			s.setDormant(status)
			return status
		}
		return s.Status()
	}
	if err := clearVNextManagedState(); err != nil {
		if active != nil {
			status := managedStatusForGrant("PERSISTENCE_UPDATE_FAILED", false, active.grant)
			s.setDormant(status)
			return status
		}
		status := AutoTuneVNextManagedStatus{State: "PERSISTENCE_UPDATE_FAILED"}
		s.setDormant(status)
		return status
	}
	s.mu.Lock()
	s.dormant = AutoTuneVNextManagedStatus{}
	s.invalidateGrantsLocked()
	s.mu.Unlock()
	if active == nil {
		return AutoTuneVNextManagedStatus{State: "DIRECT"}
	}
	return managedStatusForGrant("REVERTED", false, active.grant)
}

// Suspend preserves logical intent after direct restoration for shutdown and
// health recovery. It never clears the atomically stored selection.
func (s *productVNextService) Suspend(ctx context.Context) AutoTuneVNextManagedStatus {
	return s.DeactivateManaged(ctx, true)
}

// Revert is the external explicit user/takeover semantic: restore direct state
// and clear saved managed intent. Persistence removal failure is not reported
// as a network restore failure.
func (s *productVNextService) Revert(ctx context.Context) AutoTuneVNextManagedStatus {
	return s.DeactivateManaged(ctx, false)
}

func (s *productVNextService) setDormant(status AutoTuneVNextManagedStatus) {
	s.mu.Lock()
	s.dormant = status
	s.mu.Unlock()
}

func (s *productVNextService) rememberDormant(status AutoTuneVNextManagedStatus) AutoTuneVNextManagedStatus {
	s.setDormant(status)
	return status
}

func (s *productVNextService) Status() AutoTuneVNextManagedStatus {
	s.mu.Lock()
	active := s.active
	fault := s.fault
	dormant := s.dormant
	s.mu.Unlock()
	if fault.State != "" {
		return fault
	}
	if active != nil {
		return managedStatusForGrant("VNEXT_MANAGED_ACTIVE", true, active.grant)
	}
	if dormant.State != "" {
		return dormant
	}
	state, present, err := loadVNextManagedState()
	if err != nil {
		return AutoTuneVNextManagedStatus{State: "SAVED_STRATEGY_STALE", NeedsRevalidation: true}
	}
	if !present {
		return AutoTuneVNextManagedStatus{State: "DIRECT"}
	}
	return AutoTuneVNextManagedStatus{State: "SAVED_REVALIDATION_PENDING", NeedsRevalidation: true, Target: state.Target, StrategyID: state.StrategyID, Fingerprint: shortManagedFingerprint(state.Fingerprint), Backend: state.Backend}
}

// ManagedHealthy resolves the target's current edge without pinning it to the
// retained capture edge. Any edge or family drift requires revalidation; the
// active capture rule remains exact-edge scoped until Suspend restores it.
func (s *productVNextService) ManagedHealthy(ctx context.Context) bool {
	s.mu.Lock()
	active := s.active
	s.mu.Unlock()
	if active == nil {
		return false
	}
	candidate := active.activation.Candidate()
	if len(candidate.TargetEdge) == 0 || candidate.TargetFamily == "" {
		return false
	}
	healthCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	observation, err := s.deps.observer.Observe(healthCtx, active.grant.target.URL, observatory.Options{
		AddressFamily: active.grant.target.AddressFamily,
		Transport:     active.grant.target.Transport,
		NetworkLabel:  "product-autotune-vnext",
	})
	if err != nil || observation.Classification != observatory.ClassSuccess {
		return false
	}
	currentEdge, currentFamily, ok := concreteManagedObservationEdge(observation)
	return ok && currentEdge.Equal(candidate.TargetEdge) && currentFamily == candidate.TargetFamily
}

func concreteManagedObservationEdge(observation observatory.ObservationResult) (net.IP, observatory.AddressFamily, bool) {
	if observation.PrimaryAttemptIndex == nil {
		return nil, "", false
	}
	index := *observation.PrimaryAttemptIndex
	if index < 0 || index >= len(observation.Attempts) {
		return nil, "", false
	}
	attempt := observation.Attempts[index]
	edge := net.ParseIP(attempt.ResolvedIP)
	if edge == nil || attempt.AddressFamily == "" {
		return nil, "", false
	}
	return edge, attempt.AddressFamily, true
}

// RevalidateActive is one bounded recovery attempt. It restores direct state
// while retaining intent before rerunning the exact saved selection.
func (s *productVNextService) RevalidateActive(ctx context.Context) AutoTuneVNextManagedStatus {
	s.mu.Lock()
	active := s.active
	s.mu.Unlock()
	if active == nil {
		return s.Status()
	}
	if state := s.Suspend(ctx); state.State == "STATE_RESTORE_FAILED" {
		return state
	}
	return s.RevalidateSaved(ctx)
}

// RevalidateSaved rebuilds the current catalog and repeats the bounded
// experiment before activation. Persisted intent contains no old edge, argv,
// process, or capture state.
func (s *productVNextService) RevalidateSaved(ctx context.Context) AutoTuneVNextManagedStatus {
	state, present, err := loadVNextManagedState()
	if err != nil {
		return s.rememberDormant(AutoTuneVNextManagedStatus{State: "SAVED_STRATEGY_STALE", NeedsRevalidation: true})
	}
	if !present {
		return s.rememberDormant(AutoTuneVNextManagedStatus{State: "DIRECT"})
	}
	target, publicTarget, err := normalizeVNextTarget(state.Target)
	if err != nil {
		return s.rememberDormant(AutoTuneVNextManagedStatus{State: "SAVED_STRATEGY_STALE", NeedsRevalidation: true})
	}
	catalog, err := productionVNextStrategyCatalog(hostnameForVNextTarget(target))
	if err != nil {
		return s.rememberDormant(AutoTuneVNextManagedStatus{State: "SAVED_STRATEGY_STALE", NeedsRevalidation: true})
	}
	matched := false
	for _, strategy := range catalog {
		fingerprint, fingerprintErr := strategyir.Fingerprint(strategy)
		if strategy.ID == state.StrategyID && fingerprintErr == nil && fingerprint == state.Fingerprint {
			matched = true
			break
		}
	}
	if !matched {
		return s.rememberDormant(AutoTuneVNextManagedStatus{State: "SAVED_STRATEGY_STALE", Target: publicTarget, StrategyID: state.StrategyID, Fingerprint: shortManagedFingerprint(state.Fingerprint), Backend: state.Backend, NeedsRevalidation: true})
	}
	control, _, controlErr := normalizeVNextTarget(productVNextDefaultControl.Target)
	if controlErr != nil {
		return s.rememberDormant(AutoTuneVNextManagedStatus{State: "SAVED_STRATEGY_STALE", Target: publicTarget, StrategyID: state.StrategyID, Fingerprint: shortManagedFingerprint(state.Fingerprint), Backend: state.Backend, NeedsRevalidation: true})
	}
	result := s.Run(ctx, AutoTuneVNextRequest{Target: target.URL, Controls: []string{control.URL}})
	switch result.Status {
	case string(autotunevnext.StatusCompletedNoActionNeeded):
		return s.rememberDormant(AutoTuneVNextManagedStatus{State: "SAVED_NOT_CURRENTLY_NEEDED", Target: publicTarget, StrategyID: state.StrategyID, Fingerprint: shortManagedFingerprint(state.Fingerprint), Backend: state.Backend, NeedsRevalidation: true})
	case string(autotunevnext.StatusCompletedSelected):
		if result.ApplyAvailable && result.SelectedStrategyID == state.StrategyID && result.SelectedFingerprint == state.Fingerprint {
			return s.Apply(ctx, result.ApplyToken)
		}
	}
	return s.rememberDormant(AutoTuneVNextManagedStatus{State: "NO_VERIFIED_STRATEGY", Target: publicTarget, StrategyID: state.StrategyID, Fingerprint: shortManagedFingerprint(state.Fingerprint), Backend: state.Backend, NeedsRevalidation: true})
}

func shortManagedFingerprint(fingerprint string) string {
	if len(fingerprint) > 12 {
		return fingerprint[:12]
	}
	return fingerprint
}

func managedStatusForGrant(state string, active bool, grant verifiedSelectionGrant) AutoTuneVNextManagedStatus {
	fingerprint := grant.fingerprint
	if len(fingerprint) > 12 {
		fingerprint = fingerprint[:12]
	}
	return AutoTuneVNextManagedStatus{State: state, Active: active, Target: grant.publicTarget, StrategyID: grant.strategyID, Fingerprint: fingerprint, Backend: string(grant.backend)}
}

func hostnameForVNextTarget(target autotunevnext.Target) string {
	parsed, err := url.Parse(target.URL)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
}

func managedRuntimeFailureState(err error) string {
	if errors.Is(err, errVNextMeasurementPathUnsupported) {
		return "MEASUREMENT_PATH_UNSUPPORTED"
	}
	return "NOT_APPLIED"
}
