package main

// Bounded service-graph managed lifecycle.
//
// The temporary experiment activation and the committed managed graph runtime
// are deliberately different things:
//
//	experiment  bounded, cancellable, always restored before it returns
//	managed     committed, outlives Apply, owned until Revert/Suspend/shutdown
//
// A committed graph must not inherit the bounded Apply validation context (PR60).
// Manual user action keeps its existing precedence: a manually started profile
// takes the runtime, and the managed graph relinquishes ownership safely rather
// than letting two packet runtimes compete.

import (
	"context"
	"errors"
	"sync"
	"time"

	"unbound/engine/autotunevnext"
)

// GraphManagedState is the product-facing graph managed state.
type GraphManagedState string

const (
	GraphManagedDirect       GraphManagedState = "DIRECT"
	GraphManagedActive       GraphManagedState = "GRAPH_MANAGED"
	GraphManagedSuspended    GraphManagedState = "GRAPH_SUSPENDED"
	GraphManagedRevalidation GraphManagedState = "GRAPH_REVALIDATION_PENDING"
	GraphManagedFault        GraphManagedState = "GRAPH_FAULT"
)

// AutoTuneVNextGraphManagedStatus is the product-facing graph status. It
// deliberately exposes only logical state: no edges, no argv, no filters.
type AutoTuneVNextGraphManagedStatus struct {
	State             GraphManagedState `json:"state"`
	ServiceID         string            `json:"service_id,omitempty"`
	Backend           string            `json:"backend,omitempty"`
	GraphFingerprint  string            `json:"graph_fingerprint,omitempty"`
	NodeCount         int               `json:"node_count,omitempty"`
	EdgeCount         int               `json:"edge_count,omitempty"`
	RevalidationState string            `json:"revalidation_state,omitempty"`
	Reason            string            `json:"reason,omitempty"`
}

// graphManagedService owns exactly one graph runtime.
type graphManagedService struct {
	mu       sync.Mutex
	executor autotunevnext.ServiceGraphExecutor
	state    GraphManagedState
	intent   *persistedVNextGraphState
	status   AutoTuneVNextGraphManagedStatus
	// direct is the pre-activation state, kept so Suspend and Revert can both
	// restore the machine's original packet path.
	direct autotunevnext.StateSnapshot
}

func newGraphManagedService(executor autotunevnext.ServiceGraphExecutor) *graphManagedService {
	return &graphManagedService{executor: executor, state: GraphManagedDirect}
}

// ApplyGraph commits a validated graph as the managed runtime. Unlike a bounded
// experiment Apply, the graph stays alive after this returns.
func (s *graphManagedService) ApplyGraph(ctx context.Context, activation autotunevnext.ServiceGraphActivation, intent persistedVNextGraphState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == GraphManagedActive {
		return errors.New("a managed graph is already active")
	}
	// The intent is persisted BEFORE the machine is touched, so a crash between
	// activation and persistence cannot leave an invisible committed runtime.
	if err := saveVNextGraphState(intent); err != nil {
		return err
	}
	snapshot, err := s.executor.Snapshot(ctx)
	if err != nil {
		_ = clearVNextGraphState()
		return err
	}
	if err := s.executor.EstablishDirect(ctx, snapshot); err != nil {
		_ = clearVNextGraphState()
		return err
	}
	if err := s.executor.ActivateGraph(ctx, activation); err != nil {
		_ = s.executor.Restore(context.WithoutCancel(ctx), snapshot)
		_ = clearVNextGraphState()
		return err
	}
	if err := s.executor.VerifyActiveGraph(ctx, activation); err != nil {
		_ = s.executor.Deactivate(context.WithoutCancel(ctx))
		_ = s.executor.Restore(context.WithoutCancel(ctx), snapshot)
		_ = clearVNextGraphState()
		return err
	}
	s.direct = snapshot
	s.state = GraphManagedActive
	stored := intent
	s.intent = &stored
	s.status = graphStatusFor(GraphManagedActive, &stored, "", "")
	return nil
}

// Suspend restores the direct packet path but keeps the logical graph intent, so
// the graph can be rebuilt later from fresh authority. This is what shutdown and
// health-triggered recovery use.
func (s *graphManagedService) Suspend(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != GraphManagedActive {
		return nil
	}
	if err := s.executor.Deactivate(context.WithoutCancel(ctx)); err != nil {
		s.state = GraphManagedFault
		return err
	}
	if err := s.executor.Restore(context.WithoutCancel(ctx), s.direct); err != nil {
		s.state = GraphManagedFault
		return err
	}
	if err := s.executor.VerifyRestored(context.WithoutCancel(ctx), s.direct); err != nil {
		s.state = GraphManagedFault
		return err
	}
	// Intent is deliberately retained across Suspend.
	s.state = GraphManagedRevalidation
	s.status = graphStatusFor(GraphManagedRevalidation, s.intent, string(autotunevnext.GraphRevalidationNone), "SUSPENDED_BY_PRODUCT")
	return nil
}

// Revert is the explicit user or legacy-takeover semantic: restore the original
// state, stop the owned process, and clear the logical intent entirely.
func (s *graphManagedService) Revert(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revertLocked(ctx)
}

func (s *graphManagedService) revertLocked(ctx context.Context) error {
	if s.state == GraphManagedDirect {
		return clearVNextGraphState()
	}
	cleanup := context.WithoutCancel(ctx)
	var firstErr error
	if err := s.executor.Deactivate(cleanup); err != nil {
		firstErr = err
	}
	if err := s.executor.Restore(cleanup, s.direct); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := s.executor.VerifyRestored(cleanup, s.direct); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := clearVNextGraphState(); err != nil && firstErr == nil {
		firstErr = err
	}
	s.intent = nil
	s.state = GraphManagedDirect
	s.status = graphStatusFor(GraphManagedDirect, nil, "", "")
	return firstErr
}

// YieldToManualAction relinquishes runtime ownership when the user manually
// starts a profile. The managed graph steps aside completely; it never competes
// with the user's chosen runtime.
func (s *graphManagedService) YieldToManualAction(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == GraphManagedDirect {
		return nil
	}
	return s.revertLocked(ctx)
}

// EvaluateHealth maps the physical fact of the owned runtime onto product state.
// ownedProcessAlive reports whether the owned graph runtime is still running. It
// is supplied by the caller because process inspection belongs to the executor,
// not to the lifecycle owner.
func (s *graphManagedService) EvaluateHealth(graph autotunevnext.ServiceGraph, current autotunevnext.ServiceGraph, ownedProcessAlive bool) GraphManagedState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != GraphManagedActive {
		return s.state
	}
	health, err := autotunevnext.EvaluateServiceGraphHealth(autotunevnext.ServiceGraphHealthInput{
		ActiveCapture:     graph,
		Current:           current,
		OwnedProcessAlive: ownedProcessAlive,
	})
	switch {
	case err != nil && health == autotunevnext.ServiceGraphFault:
		s.state = GraphManagedFault
		s.status = graphStatusFor(GraphManagedFault, s.intent, "", "OWNED_GRAPH_PROCESS_DEAD")
	case health == autotunevnext.ServiceGraphNeedsRevalidation:
		s.state = GraphManagedRevalidation
		s.status = graphStatusFor(GraphManagedRevalidation, s.intent, string(autotunevnext.GraphRevalidationNone), err.Error())
	default:
		s.status = graphStatusFor(GraphManagedActive, s.intent, "", "")
	}
	return s.state
}

func graphStatusFor(state GraphManagedState, intent *persistedVNextGraphState, trigger, reason string) AutoTuneVNextGraphManagedStatus {
	status := AutoTuneVNextGraphManagedStatus{State: state, RevalidationState: trigger, Reason: reason}
	if intent != nil {
		status.ServiceID = intent.ServiceID
		status.Backend = intent.Backend
		status.GraphFingerprint = intent.GraphFingerprint
		status.NodeCount = len(intent.Nodes)
	}
	return status
}

// graphShutdownTimeout bounds shutdown cleanup so a stuck runtime cannot hang exit.
const graphShutdownTimeout = 30 * time.Second

func (s *graphManagedService) Shutdown(ctx context.Context) error {
	// Shutdown suspends rather than reverts: the user's intent survives a restart
	// and is rebuilt from fresh authority, never replayed.
	ctx, cancel := context.WithTimeout(ctx, graphShutdownTimeout)
	defer cancel()
	return s.Suspend(ctx)
}
