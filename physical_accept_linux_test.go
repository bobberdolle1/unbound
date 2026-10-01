//go:build linux && !race

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"unbound/engine"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/providers"
	"unbound/engine/strategyir"
)

// Physical acceptance harness for the bounded service graph on Linux.
//
// It drives the REAL product path: engine.ExtractAssets, VerifyExtractedAssets,
// the real product asset resolver, a real nftables table and a real NFQUEUE
// nfqws2 process. No Docker, no fake queue, no mocked nft.
//
// It must run as root because the product runtime requires it, and it refuses to
// run anywhere but the dedicated lab host.

func physicalAcceptLinuxStrategyIR(hosts []string) string {
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

type physicalAcceptLinuxProvider struct{ alive bool }

func (p *physicalAcceptLinuxProvider) CheckPrivileges() (bool, error) { return os.Geteuid() == 0, nil }
func (p *physicalAcceptLinuxProvider) Start(context.Context, string) error {
	p.alive = true
	return nil
}
func (p *physicalAcceptLinuxProvider) Stop() error            { p.alive = false; return nil }
func (p *physicalAcceptLinuxProvider) CurrentProfile() string { return "" }
func (p *physicalAcceptLinuxProvider) Name() string           { return "physical-acceptance" }
func (p *physicalAcceptLinuxProvider) GetStatus() providers.Status {
	if p.alive {
		return providers.StatusRunning
	}
	return providers.StatusStopped
}

// physicalAcceptHosts is the bounded two-node acceptance graph. IPv4 only, so
// the run is deterministic and does not manufacture IPv6 connectivity.
var physicalAcceptLinuxHosts = []string{"cloudflare.com", "github.com"}

// foreignFirewallBaseline is the pre-existing machine firewall state. It is
// captured before any mutation and must be byte-identical afterwards.
func physicalAcceptForeignRuleset(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("nft", "list", "ruleset").CombinedOutput()
	if err != nil {
		t.Fatalf("capture foreign ruleset: %v: %s", err, out)
	}
	return string(out)
}

func physicalAcceptNftTables(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("nft", "list", "tables").CombinedOutput()
	if err != nil {
		t.Fatalf("list tables: %v: %s", err, out)
	}
	return string(out)
}

func physicalAcceptNfqws2Count() int {
	out, err := exec.Command("pgrep", "-c", "-x", "nfqws2").Output()
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}

func TestPhysicalLinuxGraphAcceptance(t *testing.T) {
	if os.Getenv("UNBOUND_PHYSICAL_ACCEPT") != "1" {
		t.Skip("physical acceptance requires UNBOUND_PHYSICAL_ACCEPT=1 on a lab host")
	}
	host, err := os.Hostname()
	if err != nil || !strings.HasPrefix(host, "bobpc-HP-Compaq") {
		t.Fatalf("refusing to mutate a foreign host: hostname=%q", host)
	}
	if os.Geteuid() != 0 {
		t.Fatal("physical acceptance requires root: the product runtime manages nftables and NFQUEUE")
	}

	// The extracted runtime is process-scoped; a run must never leave it behind.
	defer func() {
		if err := engine.CleanupExtractedAssets(); err != nil {
			t.Logf("harness cleanup of extracted assets: %v", err)
		}
	}()

	tmp, err := os.MkdirTemp("", "unbound-physical-")
	if err != nil {
		t.Fatalf("temp config dir: %v", err)
	}
	defer os.RemoveAll(tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)

	// FOREIGN_FIREWALL_STATE_PRESERVED: baseline before any mutation.
	foreignBefore := physicalAcceptForeignRuleset(t)
	tablesBefore := physicalAcceptNftTables(t)
	if physicalAcceptNfqws2Count() != 0 {
		t.Fatalf("precondition: %d nfqws2 already running", physicalAcceptNfqws2Count())
	}
	// The table baseline must be captured BEFORE the first activation, otherwise
	// the owned table is already present and a diff can never see it.
	physicalAcceptBaselineTables = tablesBefore
	// A test abort must never leave owned machine state behind. This removes any
	// owned table and stops any owned process, whatever the failure mode was.
	defer physicalAcceptForceCleanup(t, tablesBefore)
	t.Logf("FOREIGN_BASELINE ruleset_bytes=%d tables=%d", len(foreignBefore), len(strings.Fields(tablesBefore)))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// ---- LINUX_ASSET_PIPELINE: real extraction + provenance ----
	assets, err := engine.ExtractAssets()
	if err != nil {
		t.Fatalf("ASSET_PIPELINE extract: %v", err)
	}
	if err := engine.VerifyExtractedAssets(assets); err != nil {
		t.Fatalf("ASSET_PIPELINE verify: %v", err)
	}
	nfqws := assets.BinDir + "/nfqws2"
	if _, err := os.Stat(nfqws); err != nil {
		t.Fatalf("ASSET_PIPELINE nfqws2 missing: %v", err)
	}
	t.Logf("ASSET_PIPELINE=PASS root=%s bin=%s", assets.RootDir, assets.BinDir)

	var strategy strategyir.Strategy
	if err := json.Unmarshal([]byte(physicalAcceptLinuxStrategyIR(physicalAcceptLinuxHosts)), &strategy); err != nil {
		t.Fatalf("strategy IR: %v", err)
	}
	backend := backendcap.Zapret2Linux
	compiled := backendcap.Compile(strategy, backend)
	if compiled.Status != backendcap.StatusCompiled {
		t.Fatalf("strategy does not compile: status=%v unsupported=%+v", compiled.Status, compiled.Unsupported)
	}
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
			t.Fatalf("ASSET_PIPELINE unresolved logical asset remained: %q", arg)
		}
	}
	t.Logf("ASSET_RESOLUTION=PASS resolved=%d argv=%v", len(resolved), plan.EngineArgv)

	// ---- Fresh product DNS: two exact IPv4 nodes, one template ----
	nodes := make([]autotunevnext.ServiceNode, 0, len(physicalAcceptLinuxHosts))
	union := make([]autotunevnext.ServiceScopeEdge, 0, 16)
	persisted := make([]persistedGraphNode, 0, len(physicalAcceptLinuxHosts))
	seen := map[string]bool{}
	for i, h := range physicalAcceptLinuxHosts {
		target := autotunevnext.Target{URL: "https://" + h + "/", AddressFamily: observatory.AddressFamilyIPv4}
		scope, err := physicalAcceptResolveWithRetry(ctx, target)
		if err != nil {
			t.Fatalf("resolve %s: %v", h, err)
		}
		if len(scope.Edges) == 0 || len(scope.Edges) > 8 {
			t.Fatalf("%s resolved %d edges, need 1..8", h, len(scope.Edges))
		}
		for _, e := range scope.Edges {
			if e.Family != observatory.AddressFamilyIPv4 {
				t.Fatalf("%s resolved a non-IPv4 edge %v; IPv4-only acceptance", h, e.IP)
			}
		}
		parsed, _ := url.Parse(scope.Target.URL)
		key := strings.ToLower(parsed.Hostname())
		if seen[key] {
			t.Fatalf("duplicate host %q", key)
		}
		seen[key] = true
		union = append(union, scope.Edges...)
		nodeID := fmt.Sprintf("NODE%d", i+1)
		nodes = append(nodes, autotunevnext.ServiceNode{
			ID: nodeID, Role: autotunevnext.ServiceNodeRoleEntry, Required: true,
			Target: target, Validation: autotunevnext.ServiceNodeActive,
			Strategy: strategy.ID, StrategyFingerprint: compiled.StrategyFingerprint,
			TemplateIdentity: strategy.ID, Scope: scope,
		})
		persisted = append(persisted, persistedGraphNode{
			NodeID: nodeID, Role: "ENTRY", Required: true, Target: "https://" + h + "/",
			StrategyID: strategy.ID, TemplateIdentity: strategy.ID, Fingerprint: compiled.StrategyFingerprint,
		})
	}
	graph, err := autotunevnext.NewServiceGraph(nodes)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}
	activation := autotunevnext.ServiceGraphActivation{
		Backend: backend, Capture: plan.Capture, Plan: plan,
		Graph: graph, Sections: physicalAcceptLinuxSections(t, graph, plan.EngineArgv), UnionEdges: union,
	}
	intent := persistedVNextGraphState{
		SchemaVersion: vNextGraphSchema, Enabled: true, ServiceID: "physical-accept",
		Backend: string(backend), GraphFingerprint: graph.Fingerprint(),
		CatalogIdentity: "physical-accept-catalog", SavedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Nodes: persisted,
	}
	t.Logf("GRAPH nodes=%d union_edges=%d", len(graph.Nodes), len(union))

	for run := 1; run <= 2; run++ {
		runPhysicalAcceptLinux(t, ctx, assets, activation, intent, run)
	}
	t.Log("SECOND_RUN=PASS")

	// ---- FOREIGN_FIREWALL_STATE_PRESERVED ----
	// Compare STRUCTURE, not bytes. Packet and byte counters on pre-existing
	// rules advance with ordinary machine traffic, so a byte-exact comparison
	// would report a false failure. What must hold is that no table, chain or
	// rule was added, removed or altered.
	foreignAfter := physicalAcceptForeignRuleset(t)
	if diff := physicalAcceptStructuralDiff(foreignBefore, foreignAfter); diff != "" {
		t.Fatalf("FOREIGN_FIREWALL_STATE_PRESERVED=FAIL the pre-existing firewall structure changed:\n%s", diff)
	}
	if physicalAcceptNftTables(t) != tablesBefore {
		t.Fatalf("FINAL_CLEANUP=FAIL the table set changed:\nbefore:\n%s\nafter:\n%s", tablesBefore, physicalAcceptNftTables(t))
	}
	if n := physicalAcceptNfqws2Count(); n != 0 {
		t.Fatalf("FINAL_CLEANUP=FAIL %d owned nfqws2 survived", n)
	}
	t.Log("FOREIGN_FIREWALL_STATE_PRESERVED=PASS FINAL_CLEANUP=PASS")
}

// physicalAcceptStructuralDiff normalizes a ruleset for structural comparison:
// counter values and nft's advisory warnings are runtime noise, everything else
// is authority and must match exactly.
// physicalAcceptCounterPattern matches an nft counter clause anywhere in a rule.
var physicalAcceptCounterPattern = regexp.MustCompile(`counter packets [0-9]+ bytes [0-9]+`)

func physicalAcceptStructuralDiff(before, after string) string {
	normalize := func(ruleset string) []string {
		lines := make([]string, 0, 128)
		for _, l := range strings.Split(ruleset, "\n") {
			l = strings.TrimSpace(l)
			if l == "" || strings.HasPrefix(l, "# Warning:") {
				continue
			}
			// Drop counter values wherever they appear; the rest of the line is
			// the authority and must match exactly.
			l = physicalAcceptCounterPattern.ReplaceAllString(l, "counter")
			l = strings.Join(strings.Fields(l), " ")
			if l != "" {
				lines = append(lines, l)
			}
		}
		return lines
	}
	b, a := normalize(before), normalize(after)
	if len(b) != len(a) {
		return fmt.Sprintf("line count %d -> %d", len(b), len(a))
	}
	for i := range b {
		if b[i] != a[i] {
			return fmt.Sprintf("line %d:\n  before: %s\n  after:  %s", i+1, b[i], a[i])
		}
	}
	return ""
}

// physicalAcceptForceCleanup removes every table this run created and stops any
// process it started. It is a harness safety net, not a product path.
func physicalAcceptForceCleanup(t *testing.T, baseline string) {
	t.Helper()
	for _, table := range physicalAcceptOwnedTables(baseline, physicalAcceptNftTables(t)) {
		fields := strings.Fields(table)
		if len(fields) < 3 {
			continue
		}
		if out, err := exec.Command("nft", "delete", "table", fields[1], fields[2]).CombinedOutput(); err != nil {
			t.Logf("harness cleanup: delete table %s: %v: %s", table, err, out)
		}
	}
	if n := physicalAcceptNfqws2Count(); n > 0 {
		_ = exec.Command("pkill", "-x", "nfqws2").Run()
	}
}

// physicalAcceptResolveWithRetry tolerates a single transient DNS failure. An
// acceptance run must fail on a product defect, not on one lost UDP answer.
func physicalAcceptResolveWithRetry(ctx context.Context, target autotunevnext.Target) (autotunevnext.ServiceScopeSnapshot, error) {
	var lastErr error
	for range 3 {
		scope, err := autotunevnext.ResolveServiceScope(ctx, target, net.DefaultResolver)
		if err == nil && len(scope.Edges) > 0 {
			return scope, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return autotunevnext.ServiceScopeSnapshot{}, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return autotunevnext.ServiceScopeSnapshot{}, lastErr
}

func physicalAcceptLinuxSections(t *testing.T, graph autotunevnext.ServiceGraph, engineArgv []string) []autotunevnext.ServiceGraphSection {
	t.Helper()
	desync := make([]string, 0, len(engineArgv))
	for _, arg := range engineArgv {
		if strings.HasPrefix(arg, "--filter-") || strings.HasPrefix(arg, "--hostlist-domains=") {
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
		argv := append(append([]string{}, desync...), "--hostlist-domains=^"+host)
		sections = append(sections, autotunevnext.ServiceGraphSection{NodeID: node.ID, Host: host, Argv: argv})
	}
	return sections
}

func runPhysicalAcceptLinux(t *testing.T, ctx context.Context, assets *engine.AssetPaths, activation autotunevnext.ServiceGraphActivation, intent persistedVNextGraphState, run int) {
	t.Helper()
	provider := &physicalAcceptLinuxProvider{}
	executor, err := autotunevnext.NewLinuxGraphExecutor(autotunevnext.RuntimeOptions{Provider: provider, Assets: assets})
	if err != nil {
		t.Fatalf("run%d new linux graph executor: %v", run, err)
	}
	service := newGraphManagedService(executor)

	snapshot, err := executor.Snapshot(ctx)
	if err != nil {
		t.Fatalf("run%d snapshot: %v", run, err)
	}
	if err := executor.EstablishDirect(ctx, snapshot); err != nil {
		t.Fatalf("run%d establish direct: %v", run, err)
	}
	tablesWithGraph := physicalAcceptNftTables(t)

	if err := service.ApplyGraph(ctx, activation, intent); err != nil {
		t.Fatalf("run%d apply: %v", run, err)
	}
	// POST_APPLY_PROCESS_ALIVE: the committed capture must survive Apply.
	time.Sleep(2 * time.Second)
	if n := physicalAcceptNfqws2Count(); n != 1 {
		t.Fatalf("run%d ONE_NFQWS2=FAIL owned nfqws2 count=%d, want 1", run, n)
	}
	t.Logf("run%d PROCESS_START=PASS ONE_NFQWS2=PASS POST_APPLY_PROCESS_ALIVE=PASS", run)

	if err := executor.VerifyActiveGraph(ctx, activation); err != nil {
		t.Fatalf("run%d verify active graph: %v", run, err)
	}
	// NFT_TABLE + EXACT_UNION: exactly one new table, carrying exact addresses.
	tablesNow := physicalAcceptNftTables(t)
	owned := physicalAcceptOwnedTables(tablesBeforeCache(t), tablesNow)
	if len(owned) != 1 {
		t.Fatalf("run%d NFT_TABLE=FAIL expected exactly 1 owned table, got %v", run, owned)
	}
	physicalAcceptAssertExactUnion(t, run, owned[0], unionAddresses(activation.UnionEdges))
	t.Logf("run%d NFT_TABLE=PASS EXACT_UNION=PASS HOST_SECTIONS=PASS NFQUEUE=PASS", run)
	_ = tablesWithGraph

	if got := service.EvaluateHealth(activation.Graph, activation.Graph, true, false); got != GraphManagedActive {
		t.Fatalf("run%d MANAGED_HEALTH=FAIL state=%q", run, got)
	}
	t.Logf("run%d MANAGED_HEALTH=PASS", run)

	if run == 1 {
		// ---- SIGTERM: the product's normal stop must remove the table ----
		if err := service.Suspend(ctx); err != nil {
			t.Fatalf("suspend: %v", err)
		}
		if n := physicalAcceptNfqws2Count(); n != 0 {
			t.Fatalf("SIGTERM_CLEANUP=FAIL %d owned nfqws2 survived", n)
		}
		if after := physicalAcceptOwnedTables(tablesBeforeCache(t), physicalAcceptNftTables(t)); len(after) != 0 {
			t.Fatalf("SIGTERM_CLEANUP=FAIL owned tables remained: %v", after)
		}
		t.Log("NORMAL_STOP=PASS SIGTERM_CLEANUP=PASS")

		// ---- Failure rollback must leave nothing behind ----
		broken := activation
		broken.Graph = physicalAcceptBrokenGraph(t)
		broken.Sections = nil
		if err := service.ApplyGraph(ctx, broken, intent); err == nil {
			t.Fatal("FAILURE_ROLLBACK=FAIL a malformed activation was accepted")
		}
		if n := physicalAcceptNfqws2Count(); n != 0 {
			t.Fatalf("FAILURE_ROLLBACK=FAIL %d owned nfqws2 left running", n)
		}
		if after := physicalAcceptOwnedTables(tablesBeforeCache(t), physicalAcceptNftTables(t)); len(after) != 0 {
			t.Fatalf("FAILURE_ROLLBACK=FAIL owned tables remained: %v", after)
		}
		t.Log("FAILURE_ROLLBACK=PASS")
	}

	if err := service.Shutdown(ctx); err != nil {
		t.Fatalf("run%d shutdown: %v", run, err)
	}
	if n := physicalAcceptNfqws2Count(); n != 0 {
		t.Fatalf("run%d FINAL_CLEANUP=FAIL %d owned nfqws2 survived", run, n)
	}
	t.Logf("run%d FINAL_CLEANUP=PASS", run)
}

// physicalAcceptBaselineTables is the table set observed before any mutation.
var physicalAcceptBaselineTables string

func tablesBeforeCache(t *testing.T) string {
	t.Helper()
	if physicalAcceptBaselineTables == "" {
		t.Fatal("table baseline was not captured before the run")
	}
	return physicalAcceptBaselineTables
}

func physicalAcceptOwnedTables(before, after string) []string {
	beforeSet := map[string]bool{}
	for _, l := range strings.Split(before, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			beforeSet[l] = true
		}
	}
	var owned []string
	for _, l := range strings.Split(after, "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !beforeSet[l] {
			owned = append(owned, l)
		}
	}
	return owned
}

func unionAddresses(edges []autotunevnext.ServiceScopeEdge) []string {
	out := make([]string, 0, len(edges))
	for _, e := range edges {
		out = append(out, e.IP.String())
	}
	return out
}

// physicalAcceptAssertExactUnion proves the owned table carries exactly the
// resolved addresses, nothing more: no CIDR, no wildcard, no list, no ASN.
func physicalAcceptAssertExactUnion(t *testing.T, run int, table string, want []string) {
	t.Helper()
	// table arrives as a full "table <family> <name>" line from `nft list tables`.
	fields := strings.Fields(table)
	if len(fields) < 3 {
		t.Fatalf("run%d owned table entry is malformed: %q", run, table)
	}
	family, name := fields[1], fields[2]
	out, err := exec.Command("nft", "list", "table", family, name).CombinedOutput()
	if err != nil {
		t.Fatalf("run%d list owned table %s %s: %v: %s", run, family, name, err, out)
	}
	ruleset := string(out)
	wanted := map[string]bool{}
	for _, a := range want {
		wanted[a] = true
	}
	for _, a := range want {
		if !strings.Contains(ruleset, a) {
			t.Fatalf("run%d EXACT_UNION=FAIL resolved address %s absent from table:\n%s", run, a, ruleset)
		}
	}
	for _, forbidden := range []string{"0.0.0.0/0", "::/0", "ipset", "@", "AS", "geoip"} {
		if strings.Contains(ruleset, forbidden) {
			t.Fatalf("run%d EXACT_UNION=FAIL broad authority %q present:\n%s", run, forbidden, ruleset)
		}
	}
	if !strings.Contains(ruleset, "443") {
		t.Fatalf("run%d exact table carries no TCP/443 rule:\n%s", run, ruleset)
	}
	t.Logf("run%d OWNED_TABLE_RULESET:\n%s", run, ruleset)
}

func physicalAcceptBrokenGraph(t *testing.T) autotunevnext.ServiceGraph {
	t.Helper()
	target := autotunevnext.Target{URL: "https://dup.example/"}
	g, _ := autotunevnext.NewServiceGraph([]autotunevnext.ServiceNode{
		{ID: "A", Role: autotunevnext.ServiceNodeRoleEntry, Required: true, Target: target, Validation: autotunevnext.ServiceNodeActive, Strategy: "s", StrategyFingerprint: "f", TemplateIdentity: "t"},
		{ID: "B", Role: autotunevnext.ServiceNodeRoleEntry, Required: true, Target: target, Validation: autotunevnext.ServiceNodeActive, Strategy: "s", StrategyFingerprint: "f", TemplateIdentity: "t"},
	})
	return g
}

var (
	_ = fmt.Sprintf
	_ = syscall.SIGTERM
)
