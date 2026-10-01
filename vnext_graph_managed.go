package main

// Bounded service-graph managed lifecycle.
//
// The temporary experiment activation and the committed managed graph runtime
// are deliberately different things:
//
//	experiment  bounded, cancellable, always restored before it returns
//	managed     committed, outlives Apply, owned until Revert/Suspend/shutdown
//
// A committed graph must not inherit the bounded Apply validation context (PR60),
// so CommitGraph detaches the activation context before Apply returns.
//
// Manual user action keeps its existing precedence: a manually started profile
// takes the runtime, and the managed graph relinquishes ownership safely rather
// than letting two packet runtimes compete.
//
// CLEANUP INVARIANT: once this service has activated a graph at least once it is
// "dirty" until a full Deactivate/Restore/VerifyRestored sequence has actually
// run. Cleanup is gated on that dirty flag, never on the reported state, so a
// FAULT or REVALIDATION_PENDING state can never leave a live capture, a stale
// nft table, a live WinDivert, or an un-cleanable machine behind it.

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

// ErrGraphIntentMismatch is returned when the durable intent does not describe
// the graph that is about to be installed.
var ErrGraphIntentMismatch = errors.New("graph managed intent does not describe the activated graph")

// AutoTuneVNextGraphManagedStatus is the product-facing graph status. It
// deliberately exposes only logical state: no edges, no argv, no filters.
type AutoTuneVNextGraphManagedStatus struct {
	State             GraphManagedState `json:"state"`
	ServiceID         string            `json:"service_id,omitempty"`
	Backend           string            `json:"backend,omitempty"`
	GraphFingerprint  string            `json:"graph_fingerprint,omitempty"`
	NodeCount         int               `json:"node_count,omitempty"`
	RevalidationState string            `json:"revalidation_state,omitempty"`
	Reason            string            `json:"reason,omitempty"`
}

// graphManagedService owns exactly one graph runtime.
type graphManagedService struct {
	mu       sync.Mutex
	executor autotunevnext.ServiceGraphExecutor
	state    GraphManagedState
	// dirty means this service has activated a graph and owes the machine a full
	// cleanup, regardless of what state currently claims.
	dirty  bool
	intent *persistedVNextGraphState
	status AutoTuneVNextGraphManagedStatus
	// direct is the pre-activation state, kept so Suspend and Revert can both
	// restore the machine's original packet path.
	direct  autotunevnext.StateSnapshot
	haveDir bool
}

func newGraphManagedService(executor autotunevnext.ServiceGraphExecutor) *graphManagedService {
	return &graphManagedService{executor: executor, state: GraphManagedDirect}
}

// graphIntentMatchesActivation verifies that the durable intent actually
// describes the graph about to be installed. Without this the persisted authority
// record and the physical capture could silently disagree.
func graphIntentMatchesActivation(intent persistedVNextGraphState, activation autotunevnext.ServiceGraphActivation) error {
	active, err := activation.Graph.ActiveCaptureGraph()
	if err != nil {
		return err
	}
	if intent.Backend != "" && string(activation.Backend) != intent.Backend {
		return errors.New("intent backend does not match the activation backend")
	}
	if intent.GraphFingerprint != activation.Graph.Fingerprint() {
		return ErrGraphIntentMismatch
	}
	if len(intent.Nodes) != len(active.Nodes) {
		return ErrGraphIntentMismatch
	}
	savedNodes := make(map[string]persistedGraphNode, len(intent.Nodes))
	for _, node := range intent.Nodes {
		savedNodes[node.NodeID] = node
	}
	for _, node := range active.Nodes {
		saved, ok := savedNodes[node.ID]
		if !ok {
			return ErrGraphIntentMismatch
		}
		if saved.StrategyID != node.Strategy {
			return ErrGraphIntentMismatch
		}
		if saved.TemplateIdentity != node.TemplateIdentity {
			return ErrGraphIntentMismatch
		}
		if saved.Required != node.Required {
			return ErrGraphIntentMismatch
		}
		savedHost, err := normalizedVNextHost(saved.Target)
		if err != nil {
			return err
		}
		if savedHost != node.Hostname() {
			return ErrGraphIntentMismatch
		}
	}
	return nil
}

// cleanupOwnedRuntime runs the full Deactivate/Restore/VerifyRestored sequence.
// It never stops early: every step is attempted and the first error wins, so a
// failed Deactivate still gets a restore attempt.
func (s *graphManagedService) cleanupOwnedRuntime(ctx context.Context) error {
	cleanup := context.WithoutCancel(ctx)
	var firstErr error
	if err := s.executor.Deactivate(cleanup); err != nil {
		firstErr = err
	}
	if s.haveDir {
		if err := s.executor.Restore(cleanup, s.direct); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := s.executor.VerifyRestored(cleanup, s.direct); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	s.dirty = false
	return firstErr
}

// ApplyGraph commits a validated graph as the managed runtime. Unlike a bounded
// experiment Apply, the graph stays alive after this returns.
func (s *graphManagedService) ApplyGraph(ctx context.Context, activation autotunevnext.ServiceGraphActivation, intent persistedVNextGraphState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == GraphManagedActive || s.dirty {
		return errors.New("a managed graph is already active")
	}
	// The durable intent must describe the graph we are about to install.
	if err := graphIntentMatchesActivation(intent, activation); err != nil {
		return err
	}
	// Persist BEFORE the machine is touched, so a crash between activation and
	// persistence cannot leave an invisible committed runtime.
	if err := saveVNextGraphState(intent); err != nil {
		return err
	}
	snapshot, err := s.executor.Snapshot(ctx)
	if err != nil {
		_ = clearVNextGraphState()
		return err
	}
	s.direct, s.haveDir = snapshot, true
	s.dirty = true
	if err := s.executor.EstablishDirect(ctx, snapshot); err != nil {
		s.dirty = false
		_ = clearVNextGraphState()
		return err
	}
	if err := s.executor.ActivateGraph(ctx, activation); err != nil {
		return s.failApply(ctx, err)
	}
	if err := s.executor.VerifyActiveGraph(ctx, activation); err != nil {
		return s.failApply(ctx, err)
	}
	// Detach the bounded activation context so the committed runtime outlives
	// this call. Without it a caller-side defer cancel() would kill the process
	// the instant Apply returned.
	if err := s.executor.CommitGraph(activation); err != nil {
		return s.failApply(ctx, err)
	}
	s.state = GraphManagedActive
	stored := intent
	s.intent = &stored
	s.status = graphStatusFor(GraphManagedActive, &stored, "", "")
	return nil
}

// failApply performs full cleanup after a failed activation. The persisted intent
// is deliberately KEPT when cleanup itself fails, so a later retry can still
// find the authority record; dropping it would leave an untracked live capture.
func (s *graphManagedService) failApply(ctx context.Context, cause error) error {
	cleanupErr := s.cleanupOwnedRuntime(ctx)
	if cleanupErr == nil {
		_ = clearVNextGraphState()
		s.state = GraphManagedDirect
		s.status = graphStatusFor(GraphManagedDirect, nil, "", "")
		return cause
	}
	s.state = GraphManagedFault
	s.status = graphStatusFor(GraphManagedFault, s.intent, "", cleanupErr.Error())
	return errors.Join(cause, autotunevnext.ErrManagedStateRestoreFailed)
}

// Suspend restores the direct packet path but keeps the logical graph intent, so
// the graph can be rebuilt later from fresh authority. This is what shutdown and
// health-triggered recovery use.
func (s *graphManagedService) Suspend(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dirty {
		return nil
	}
	if err := s.cleanupOwnedRuntime(ctx); err != nil {
		s.state = GraphManagedFault
		s.status = graphStatusFor(GraphManagedFault, s.intent, "", err.Error())
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
	if !s.dirty {
		s.state = GraphManagedDirect
		return clearVNextGraphState()
	}
	firstErr := s.cleanupOwnedRuntime(ctx)
	if clearErr := clearVNextGraphState(); clearErr != nil && firstErr == nil {
		firstErr = clearErr
	}
	s.intent = nil
	if firstErr != nil {
		s.state = GraphManagedFault
		s.status = graphStatusFor(GraphManagedFault, nil, "", firstErr.Error())
		return firstErr
	}
	s.state = GraphManagedDirect
	s.status = graphStatusFor(GraphManagedDirect, nil, "", "")
	return nil
}

// YieldToManualAction relinquishes runtime ownership when the user manually
// starts a profile. The managed graph steps aside completely; it never competes
// with the user's chosen runtime.
func (s *graphManagedService) YieldToManualAction(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dirty {
		return nil
	}
	return s.revertLocked(ctx)
}

// EvaluateHealth maps the physical fact of the owned runtime onto product state.
func (s *graphManagedService) EvaluateHealth(graph autotunevnext.ServiceGraph, current autotunevnext.ServiceGraph, ownedProcessAlive, resolverFailed bool) GraphManagedState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != GraphManagedActive {
		return s.state
	}
	health, err := autotunevnext.EvaluateServiceGraphHealth(autotunevnext.ServiceGraphHealthInput{
		ActiveCapture:     graph,
		Current:           current,
		OwnedProcessAlive: ownedProcessAlive,
		ResolverFailed:    resolverFailed,
	})
	// The error is a reason, not a precondition: a FAULT must never fall through
	// to the healthy default just because it was returned without one.
	switch {
	case health == autotunevnext.ServiceGraphFault:
		s.state = GraphManagedFault
		s.status = graphStatusFor(GraphManagedFault, s.intent, "", faultReason(err, "OWNED_GRAPH_PROCESS_DEAD"))
	case health == autotunevnext.ServiceGraphNeedsRevalidation:
		s.state = GraphManagedRevalidation
		s.status = graphStatusFor(GraphManagedRevalidation, s.intent, string(autotunevnext.GraphRevalidationNone), faultReason(err, "GRAPH_NEEDS_REVALIDATION"))
	default:
		s.status = graphStatusFor(GraphManagedActive, s.intent, "", "")
	}
	return s.state
}

func faultReason(err error, fallback string) string {
	if err != nil {
		return err.Error()
	}
	return fallback
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

// Shutdown suspends rather than reverts: the user's intent survives a restart and
// is rebuilt from fresh authority, never replayed. It always runs a full cleanup
// whenever the service is dirty, regardless of the reported state.
func (s *graphManagedService) Shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, graphShutdownTimeout)
	defer cancel()
	return s.Suspend(ctx)
}
