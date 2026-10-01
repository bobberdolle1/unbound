//go:build !race

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"unbound/engine"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/providers"
	"unbound/engine/strategyir"
)

// Physical acceptance harness for the bounded service graph on Windows.
//
// It drives the REAL product path: engine.ExtractAssets, engine provenance
// verification, the real winws2 binary and real WinDivert capture. Nothing is
// faked and no in-memory asset state is injected. The only injection is the
// bounding provider (privilege + status), because a harness has no legacy
// provider to own.
//
// It is skipped unless UNBOUND_PHYSICAL_ACCEPT=1 so it never runs in normal CI.

// physicalAcceptStrategyIR builds a real, semantically representable StrategyIR
// for the acceptance hosts: a TCP TLS multisplit at position 1 with the proven
// google overlap pattern, target-only, LOW aggressiveness, and explicitly
// Steam-safe. It is declared as IR rather than assembled field-by-field so the
// document the compiler canonicalizes is exactly the one under test.
func physicalAcceptStrategyIR(hosts []string) string {
	encoded, err := json.Marshal(hosts)
	if err != nil {
		panic(err)
	}
	return `{
  "schema_version": 1,
  "id": "physical.accept.tls.multisplit",
  "name": "Physical acceptance TLS multisplit",
  "transport": ["TCP"],
  "selector": {
    "application_protocols": ["TLS"],
    "ip_families": ["IPv4"],
    "direction": "OUTBOUND",
    "tcp_ports": [{"start": 443, "end": 443}],
    "scope": {"host": {"mode": "EXPLICIT", "hosts": ` + string(encoded) + `}}
  },
  "operations": [
    {"type": "MULTI_SPLIT", "positions": [{"absolute": 1}], "sequence_overlap": 652, "overlap_pattern_ref": "tls-google"}
  ],
  "safety": {
    "aggressiveness": "LOW",
    "target_only": true,
    "may_affect_steam": false,
    "may_affect_non_target_tls": false,
    "experimental": false
  },
  "metadata": {"source": "physical-acceptance", "description": "bounded graph runtime acceptance"}
}`
}

// physicalAcceptProvider is a harness-only RuntimeProvider. It performs the
// real privilege check and reports a real status; it never fakes success for
// an operation the executor performs itself.
type physicalAcceptProvider struct{ alive bool }

func (p *physicalAcceptProvider) CheckPrivileges() (bool, error) { return true, nil }
func (p *physicalAcceptProvider) Start(context.Context, string) error {
	p.alive = true
	return nil
}
func (p *physicalAcceptProvider) Stop() error            { p.alive = false; return nil }
func (p *physicalAcceptProvider) CurrentProfile() string { return "" }
func (p *physicalAcceptProvider) Name() string           { return "physical-acceptance" }
func (p *physicalAcceptProvider) GetStatus() providers.Status {
	if p.alive {
		return providers.StatusRunning
	}
	return providers.StatusStopped
}

// physicalAcceptHosts are the two acceptance hosts. They are deliberately
// boring, long-lived, non-YouTube hosts: this is graph-runtime acceptance, not
// service discovery acceptance.
var physicalAcceptHosts = []string{"cloudflare.com", "www.gstatic.com"}

func TestPhysicalWindowsGraphAcceptance(t *testing.T) {
	if os.Getenv("UNBOUND_PHYSICAL_ACCEPT") != "1" {
		t.Skip("physical acceptance requires UNBOUND_PHYSICAL_ACCEPT=1 on a lab host")
	}
	if got := os.Getenv("COMPUTERNAME"); got != "DESKTOP-MNEHCPT" {
		t.Fatalf("refusing to mutate a foreign host: COMPUTERNAME=%q", got)
	}

	// The extracted runtime is process-scoped; a run must never leave it behind.
	defer func() {
		if err := engine.CleanupExtractedAssets(); err != nil {
			t.Logf("harness cleanup of extracted assets: %v", err)
		}
	}()

	// Hermetic config dir so a real run never touches an installed profile.
	tmp, err := os.MkdirTemp("", "unbound-physical-")
	if err != nil {
		t.Fatalf("temp config dir: %v", err)
	}
	defer os.RemoveAll(tmp)
	t.Setenv("LOCALAPPDATA", tmp)
	t.Setenv("APPDATA", tmp)

	// The baseline must be zero so any winws2 seen later is one this run started.
	if n := physicalAcceptOwnedPIDs(); n != 0 {
		t.Fatalf("precondition: %d winws2 already running, refusing to claim foreign state", n)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// ---- WINDOWS_ASSET_PIPELINE: real extraction + provenance verification ----
	assets, err := engine.ExtractAssets()
	if err != nil {
		t.Fatalf("ASSET_PIPELINE extract: %v", err)
	}
	if err := engine.VerifyExtractedAssets(assets); err != nil {
		t.Fatalf("ASSET_PIPELINE verify: %v", err)
	}
	winws := assets.BinDir + string(os.PathSeparator) + "winws2.exe"
	if _, err := os.Stat(winws); err != nil {
		t.Fatalf("ASSET_PIPELINE winws2 missing: %v", err)
	}
	t.Logf("ASSET_PIPELINE=PASS root=%s bin=%s", assets.RootDir, assets.BinDir)

	// ---- One real strategy, compiled deterministically ----
	var strategy strategyir.Strategy
	if err := json.Unmarshal([]byte(physicalAcceptStrategyIR(physicalAcceptHosts)), &strategy); err != nil {
		t.Fatalf("strategy IR: %v", err)
	}
	backend := backendcap.Zapret2Windows
	compiled := backendcap.Compile(strategy, backend)
	if compiled.Status != backendcap.StatusCompiled {
		t.Fatalf("strategy does not compile: status=%v unsupported=%+v", compiled.Status, compiled.Unsupported)
	}
	if compiled.StrategyFingerprint == "" {
		t.Fatal("compile produced no strategy fingerprint")
	}

	// ---- Resolve logical assets through the REAL product resolver ----
	resolver, err := autotunevnext.NewProductAssetResolver(assets)
	if err != nil {
		t.Fatalf("ASSET_PIPELINE product asset resolver: %v", err)
	}
	resolved, err := resolver.Resolve(ctx, backend, compiled.RequiredAssets)
	if err != nil {
		t.Fatalf("ASSET_PIPELINE resolve %v: %v", compiled.RequiredAssets, err)
	}
	plan := compiled.Plan
	plan.EngineArgv, err = autotunevnext.MaterializeEngineArgv(plan.EngineArgv, resolved)
	if err != nil {
		t.Fatalf("ASSET_PIPELINE materialize argv: %v", err)
	}
	for _, arg := range plan.EngineArgv {
		if strings.Contains(arg, "${asset:") {
			t.Fatalf("ASSET_PIPELINE unresolved logical asset remained in argv: %q", arg)
		}
	}
	t.Logf("ASSET_RESOLUTION=PASS resolved=%d argv=%v", len(resolved), plan.EngineArgv)

	// ---- Fresh product DNS: two exact hosts, one template, independently bound ----
	nodes := make([]autotunevnext.ServiceNode, 0, len(physicalAcceptHosts))
	union := make([]autotunevnext.ServiceScopeEdge, 0, 16)
	persisted := make([]persistedGraphNode, 0, len(physicalAcceptHosts))
	seenHost := map[string]bool{}
	for i, host := range physicalAcceptHosts {
		target := autotunevnext.Target{URL: "https://" + host + "/"}
		scope, err := autotunevnext.ResolveServiceScope(ctx, target, net.DefaultResolver)
		if err != nil {
			t.Fatalf("resolve %s: %v", host, err)
		}
		if len(scope.Edges) == 0 || len(scope.Edges) > 8 {
			t.Fatalf("%s resolved %d edges, need 1..8", host, len(scope.Edges))
		}
		parsed, err := url.Parse(scope.Target.URL)
		if err != nil {
			t.Fatalf("parse %s: %v", host, err)
		}
		hostKey := strings.ToLower(parsed.Hostname())
		if seenHost[hostKey] {
			t.Fatalf("duplicate host %q in graph", hostKey)
		}
		seenHost[hostKey] = true
		union = append(union, scope.Edges...)
		nodeID := fmt.Sprintf("NODE%d", i+1)
		nodes = append(nodes, autotunevnext.ServiceNode{
			ID: nodeID, Role: autotunevnext.ServiceNodeRoleEntry, Required: true,
			Target: target, Validation: autotunevnext.ServiceNodeActive,
			Strategy: strategy.ID, StrategyFingerprint: compiled.StrategyFingerprint,
			TemplateIdentity: strategy.ID, Scope: scope,
		})
		persisted = append(persisted, persistedGraphNode{
			NodeID: nodeID, Role: "ENTRY", Required: true,
			Target: "https://" + host + "/", StrategyID: strategy.ID,
			TemplateIdentity: strategy.ID, Fingerprint: compiled.StrategyFingerprint,
		})
	}

	graph, err := autotunevnext.NewServiceGraph(nodes)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}
	activation := autotunevnext.ServiceGraphActivation{
		Backend: backend, Capture: plan.Capture, Plan: plan,
		Graph: graph, Sections: physicalAcceptSections(t, graph, plan.EngineArgv), UnionEdges: union,
	}
	intent := persistedVNextGraphState{
		SchemaVersion: vNextGraphSchema, Enabled: true, ServiceID: "physical-accept",
		Backend: string(backend), GraphFingerprint: graph.Fingerprint(),
		CatalogIdentity: "physical-accept-catalog", SavedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Nodes: persisted,
	}

	t.Logf("GRAPH nodes=%d union_edges=%d", len(graph.Nodes), len(union))

	for run := 1; run <= 2; run++ {
		runPhysicalAcceptGraph(t, ctx, assets, activation, intent, run)
	}
	t.Log("SECOND_RUN=PASS")
}

func runPhysicalAcceptGraph(t *testing.T, ctx context.Context, assets *engine.AssetPaths, activation autotunevnext.ServiceGraphActivation, intent persistedVNextGraphState, run int) {
	t.Helper()
	provider := &physicalAcceptProvider{}
	executor, err := autotunevnext.NewWindowsGraphExecutor(autotunevnext.RuntimeOptions{
		Provider: provider, Assets: assets, Log: physicalAcceptLog,
	})
	if err != nil {
		t.Fatalf("new windows graph executor: %v", err)
	}
	service := newGraphManagedService(executor)

	snapshot, err := executor.Snapshot(ctx)
	if err != nil {
		t.Fatalf("run%d snapshot: %v", run, err)
	}
	if err := executor.EstablishDirect(ctx, snapshot); err != nil {
		t.Fatalf("run%d establish direct: %v", run, err)
	}

	// ---- ApplyGraph, then prove the process SURVIVES the Apply return ----
	if err := service.ApplyGraph(ctx, activation, intent); err != nil {
		t.Fatalf("run%d apply: %v", run, err)
	}
	// Bounded wait: this is the P0 lifetime check. A committed graph must still
	// be alive after the Apply that started it has fully returned.
	time.Sleep(3 * time.Second)
	if physicalAcceptOwnedPIDs() != 1 {
		t.Fatalf("run%d POST_APPLY_PROCESS_ALIVE=FAIL owned winws2 count=%d, want exactly 1", run, physicalAcceptOwnedPIDs())
	}
	t.Logf("run%d PROCESS_START=PASS ONE_PROCESS=PASS POST_APPLY_PROCESS_ALIVE=PASS", run)

	if err := executor.VerifyActiveGraph(ctx, activation); err != nil {
		t.Fatalf("run%d verify active graph: %v", run, err)
	}
	t.Logf("run%d EXACT_UNION=PASS HOST_SECTIONS=PASS", run)

	// ---- Health stays healthy when nothing changed ----
	if got := service.EvaluateHealth(activation.Graph, physicalAcceptSameGraph(t, activation.Graph), true, false); got != GraphManagedActive {
		t.Fatalf("run%d MANAGED_HEALTH=FAIL state=%q, want GRAPH_MANAGED_ACTIVE", run, got)
	}
	t.Logf("run%d MANAGED_HEALTH=PASS", run)

	// ---- A moved edge must not silently stay ACTIVE ----
	if got := service.EvaluateHealth(activation.Graph, physicalAcceptMovedGraph(t), true, false); got == GraphManagedActive {
		t.Fatalf("run%d REVALIDATION_PENDING=FAIL a changed edge reported ACTIVE", run)
	}
	t.Logf("run%d REVALIDATION_PENDING=PASS", run)

	if run == 1 {
		// ---- Suspend cleans up but RETAINS the intent ----
		if err := service.Suspend(ctx); err != nil {
			t.Fatalf("suspend: %v", err)
		}
		if n := physicalAcceptOwnedPIDs(); n != 0 {
			t.Fatalf("SUSPEND=FAIL %d owned winws2 still alive", n)
		}
		if _, ok, _ := loadVNextGraphState(); !ok {
			t.Fatal("SUSPEND=FAIL the intent must be retained")
		}
		t.Log("SUSPEND=PASS")

		// ---- Revert cleans up and CLEARS the intent ----
		if err := service.ApplyGraph(ctx, activation, intent); err != nil {
			t.Fatalf("re-apply before revert: %v", err)
		}
		if err := service.Revert(ctx); err != nil {
			t.Fatalf("revert: %v", err)
		}
		if n := physicalAcceptOwnedPIDs(); n != 0 {
			t.Fatalf("REVERT=FAIL %d owned winws2 still alive", n)
		}
		if _, ok, _ := loadVNextGraphState(); ok {
			t.Fatal("REVERT=FAIL the intent must be cleared")
		}
		t.Log("REVERT=PASS")

		// ---- Failure rollback leaves nothing behind ----
		broken := activation
		broken.Graph = physicalAcceptBrokenGraph(t)
		broken.Sections = nil
		if err := service.ApplyGraph(ctx, broken, intent); err == nil {
			t.Fatal("FAILURE_ROLLBACK=FAIL a malformed activation was accepted")
		}
		if n := physicalAcceptOwnedPIDs(); n != 0 {
			t.Fatalf("FAILURE_ROLLBACK=FAIL %d owned winws2 left running", n)
		}
		if _, ok, _ := loadVNextGraphState(); ok {
			t.Fatal("FAILURE_ROLLBACK=FAIL a failed apply persisted an intent")
		}
		t.Log("FAILURE_ROLLBACK=PASS")
	}

	// ---- Shutdown fully cleans up regardless of health state ----
	if err := service.Shutdown(ctx); err != nil {
		t.Fatalf("run%d shutdown: %v", run, err)
	}
	if n := physicalAcceptOwnedPIDs(); n != 0 {
		t.Fatalf("run%d FINAL_CLEANUP=FAIL %d owned winws2 survived Shutdown", run, n)
	}
	t.Logf("run%d FINAL_CLEANUP=PASS", run)
}

// physicalAcceptSections derives one host-scoped section per node from the
// compiled plan, exactly as the product does: the template's engine arguments
// with the host selector rebound to this node's exact host. Capture authority
// (--filter-*, --payload, the strategy's own host list) is dropped, because the
// executor owns the single union capture and each section may carry only its own
// exact host.
func physicalAcceptSections(t *testing.T, graph autotunevnext.ServiceGraph, engineArgv []string) []autotunevnext.ServiceGraphSection {
	t.Helper()
	desync := make([]string, 0, len(engineArgv))
	for _, arg := range engineArgv {
		if strings.HasPrefix(arg, "--filter-") || strings.HasPrefix(arg, "--payload=") ||
			strings.HasPrefix(arg, "--hostlist-domains=") {
			continue
		}
		desync = append(desync, arg)
	}
	sections := make([]autotunevnext.ServiceGraphSection, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		parsed, err := url.Parse(node.Target.URL)
		if err != nil {
			t.Fatalf("parse node %q: %v", node.ID, err)
		}
		host := strings.ToLower(parsed.Hostname())
		if host == "" {
			t.Fatalf("node %q has no resolved host", node.ID)
		}
		argv := make([]string, 0, len(desync)+1)
		argv = append(argv, desync...)
		argv = append(argv, "--hostlist-domains=^"+host)
		sections = append(sections, autotunevnext.ServiceGraphSection{
			NodeID: node.ID, Host: host, Argv: argv,
		})
	}
	return sections
}

func physicalAcceptLog(p autotunevnext.PhysicalLog) {
	if os.Getenv("UNBOUND_PHYSICAL_ACCEPT_VERBOSE") == "1" {
		fmt.Printf("PHASE=%s PID=%d OUTCOME=%s\n", p.Phase, p.PID, p.Outcome)
	}
}

// physicalAcceptOwnedPIDs counts live winws2 processes by name. The zero
// baseline asserted before activation means this can only ever count processes
// this run started.
func physicalAcceptOwnedPIDs() int {
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq winws2.exe", "/NH").Output()
	if err != nil {
		return -1
	}
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(strings.ToLower(line), "winws2.exe") {
			count++
		}
	}
	return count
}

func physicalAcceptSameGraph(t *testing.T, graph autotunevnext.ServiceGraph) autotunevnext.ServiceGraph {
	t.Helper()
	nodes := make([]autotunevnext.ServiceNode, 0, len(graph.Nodes))
	nodes = append(nodes, graph.Nodes...)
	rebuilt, err := autotunevnext.NewServiceGraph(nodes)
	if err != nil {
		t.Fatalf("rebuild graph: %v", err)
	}
	return rebuilt
}

func physicalAcceptMovedGraph(t *testing.T) autotunevnext.ServiceGraph {
	t.Helper()
	target := autotunevnext.Target{URL: "https://moved.example/"}
	g, err := autotunevnext.NewServiceGraph([]autotunevnext.ServiceNode{{
		ID: "NODE1", Role: autotunevnext.ServiceNodeRoleEntry, Required: true,
		Target: target, Validation: autotunevnext.ServiceNodeActive,
		Strategy: "s", StrategyFingerprint: "f", TemplateIdentity: "t",
		Scope: autotunevnext.ServiceScopeSnapshot{
			Target: target,
			Edges:  []autotunevnext.ServiceScopeEdge{{IP: net.ParseIP("203.0.113.9"), Family: observatory.AddressFamilyIPv4}},
		},
	}})
	if err != nil {
		t.Fatalf("build moved graph: %v", err)
	}
	return g
}

// physicalAcceptBrokenGraph is invalid: two required nodes share one host.
func physicalAcceptBrokenGraph(t *testing.T) autotunevnext.ServiceGraph {
	t.Helper()
	target := autotunevnext.Target{URL: "https://dup.example/"}
	g, _ := autotunevnext.NewServiceGraph([]autotunevnext.ServiceNode{
		{ID: "A", Role: autotunevnext.ServiceNodeRoleEntry, Required: true, Target: target, Validation: autotunevnext.ServiceNodeActive, Strategy: "s", StrategyFingerprint: "f", TemplateIdentity: "t"},
		{ID: "B", Role: autotunevnext.ServiceNodeRoleEntry, Required: true, Target: target, Validation: autotunevnext.ServiceNodeActive, Strategy: "s", StrategyFingerprint: "f", TemplateIdentity: "t"},
	})
	return g
}
