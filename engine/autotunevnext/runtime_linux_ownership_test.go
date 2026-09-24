//go:build linux

package autotunevnext

import (
	"context"
	"errors"
	"net"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"unbound/engine"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/providers"
	"unbound/engine/strategyir"
)

type linuxRunnerStub struct {
	runFn func(string, []string) (string, error)
	paths map[string]bool
	calls []string
}

func (r *linuxRunnerStub) run(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	if r.runFn == nil {
		return "", nil
	}
	return r.runFn(name, args)
}
func (r *linuxRunnerStub) lookPath(name string) (string, error) {
	if r.paths[name] {
		return "/usr/sbin/" + name, nil
	}
	return "", errors.New("unavailable")
}

type linuxTestExitError int

func (e linuxTestExitError) Error() string { return "iptables exit" }
func (e linuxTestExitError) ExitCode() int { return int(e) }

func ownedLinuxSpec() LinuxNFQueueSpec {
	return LinuxNFQueueSpec{Table: "unbound_autotune_test", Queue: 40000, Marker: linuxOwnershipPrefix + ":test", Edge: []byte{192, 0, 2, 7}, Family: observatory.AddressFamilyIPv4, Ports: []strategyir.PortRange{{Start: 443, End: 443}}}
}

func testLinuxRuntime(t *testing.T, runner *linuxRunnerStub) *LinuxRuntime {
	t.Helper()
	runtime, err := NewLinuxRuntime(RuntimeOptions{Provider: &linuxRuntimeProvider{status: providers.StatusRunning, profile: "original"}, Assets: &engine.AssetPaths{}})
	if err != nil {
		t.Fatal(err)
	}
	runtime.runner = runner
	runtime.processAlive = func(int) (bool, error) { return false, nil }
	runtime.killGroup = func(int, syscall.Signal) error { return nil }
	runtime.readQueueState = func() ([]byte, error) { return nil, nil }
	runtime.randomBytes = func(data []byte) (int, error) { clear(data); return len(data), nil }
	runtime.cleanupWait, runtime.forceWait = time.Millisecond, time.Millisecond
	return runtime
}

func closedLinuxCandidate(spec LinuxNFQueueSpec) *linuxCandidate {
	done := make(chan struct{})
	close(done)
	return &linuxCandidate{
		done:           done,
		cancel:         func() {},
		spec:           spec,
		mode:           "iptables",
		pid:            4242,
		processStarted: true,
		ruleInstalled:  true,
	}
}

func TestLinuxDeactivateRetainsOwnershipUntilProcessAndRuleAreGone(t *testing.T) {
	runner := &linuxRunnerStub{paths: map[string]bool{}}
	runner.runFn = func(_ string, args []string) (string, error) {
		if strings.Contains(strings.Join(args, " "), " -C ") {
			return "", linuxTestExitError(1)
		}
		return "", nil
	}
	runtime := testLinuxRuntime(t, runner)
	snapshot, err := runtime.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	runtime.active = closedLinuxCandidate(ownedLinuxSpec())
	runtime.killGroup = func(int, syscall.Signal) error { return errors.New("cannot signal") }
	if err := runtime.Deactivate(context.Background()); err == nil {
		t.Fatal("termination failure passed")
	}
	if runtime.active == nil {
		t.Fatal("ownership was cleared after termination failure")
	}
	runtime.killGroup = func(int, syscall.Signal) error { return nil }
	firstDelete := true
	runner.runFn = func(_ string, args []string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, " -D ") && firstDelete {
			firstDelete = false
			return "delete failed", errors.New("failure")
		}
		if strings.Contains(joined, " -C ") {
			return "", linuxTestExitError(1)
		}
		return "", nil
	}
	if err := runtime.Restore(context.Background(), snapshot); err == nil {
		t.Fatal("rule deletion failure passed")
	}
	if runtime.active == nil {
		t.Fatal("ownership was cleared after rule deletion failure")
	}
	if err := runtime.Restore(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	if runtime.active != nil {
		t.Fatal("ownership remained after factual cleanup")
	}
	if err := runtime.VerifyRestored(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxVerifyRestoredFailsForResidualOwnedRule(t *testing.T) {
	runner := &linuxRunnerStub{paths: map[string]bool{}}
	runner.runFn = func(_ string, args []string) (string, error) {
		if strings.Contains(strings.Join(args, " "), " -C ") {
			return "rule remains", nil
		}
		return "", nil
	}
	runtime := testLinuxRuntime(t, runner)
	snapshot, err := runtime.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	runtime.active = closedLinuxCandidate(ownedLinuxSpec())
	if err := runtime.VerifyRestored(context.Background(), snapshot); err == nil {
		t.Fatal("restoration passed with residual rule")
	}
}

func TestLinuxFirewallListingCoversIPTablesAndIP6Tables(t *testing.T) {
	runner := &linuxRunnerStub{paths: map[string]bool{"ip6tables": true}}
	runner.runFn = func(name string, _ []string) (string, error) {
		if name == "ip6tables" {
			return "-A OUTPUT --comment " + linuxOwnershipPrefix + ":stale", nil
		}
		return "-A OUTPUT", nil
	}
	runtime := testLinuxRuntime(t, runner)
	listing, err := runtime.firewallListing(context.Background())
	if err != nil || !strings.Contains(listing, linuxOwnershipPrefix+":stale") {
		t.Fatalf("listing=%q err=%v", listing, err)
	}
}

func TestLinuxQueueOwnershipUsesExactReferencesAndNFNetlink(t *testing.T) {
	queues := queuesInRuleListing(strings.Join([]string{
		"queue num 40000",
		"queue flags bypass to 40001",
		"--queue-num 40002",
		"unrelated-40003",
		"tcp dport 40004",
	}, "\n"))
	for _, queue := range []uint16{40000, 40001, 40002} {
		if _, ok := queues[queue]; !ok {
			t.Fatalf("queue %d was not parsed", queue)
		}
	}
	for _, number := range []uint16{40003, 40004} {
		if _, ok := queues[number]; ok {
			t.Fatalf("unrelated number %d became a queue identity", number)
		}
	}
	if !nfNetlinkQueueBound([]byte("40000 99 1"), 40000) {
		t.Fatal("bound nfnetlink queue was not detected")
	}

	runner := &linuxRunnerStub{paths: map[string]bool{}}
	runner.runFn = func(_ string, _ []string) (string, error) { return "", nil }
	runtime := testLinuxRuntime(t, runner)
	runtime.readQueueState = func() ([]byte, error) { return []byte("40000"), nil }
	if _, _, _, err := runtime.allocateOwnership(context.Background(), "iptables"); err == nil {
		t.Fatal("occupied nfnetlink queue was selected")
	}
}

func TestLinuxAllocateOwnershipRejectsCanonicalNFTQueueCollision(t *testing.T) {
	runner := &linuxRunnerStub{paths: map[string]bool{"nft": true}}
	runner.runFn = func(_ string, args []string) (string, error) {
		if strings.Join(args, " ") != "list ruleset" {
			t.Fatalf("unexpected nft command: %q", args)
		}
		return "table ip existing { chain output { queue flags bypass to 41370 } }", nil
	}
	runtime := testLinuxRuntime(t, runner)
	proposals := [][]byte{{5, 90, 1}, {5, 91, 2}}
	runtime.randomBytes = func(data []byte) (int, error) {
		copy(data, proposals[0])
		proposals = proposals[1:]
		return len(data), nil
	}
	queue, _, _, err := runtime.allocateOwnership(context.Background(), "nft")
	if err != nil {
		t.Fatal(err)
	}
	if queue != 41371 {
		t.Fatalf("queue=%d, want canonical collision to reject 41370", queue)
	}
}

func TestLinuxRuleDeletionRequiresFactualAbsence(t *testing.T) {
	spec := ownedLinuxSpec()
	for _, tc := range []struct {
		name, mode string
		runFn      func(string, []string) (string, error)
		wantErr    bool
	}{
		{"iptables residual", "iptables", func(_ string, args []string) (string, error) {
			if strings.Contains(strings.Join(args, " "), " -C ") {
				return "still here", nil
			}
			return "", nil
		}, true},
		{"iptables absent", "iptables", func(_ string, args []string) (string, error) {
			if strings.Contains(strings.Join(args, " "), " -C ") {
				return "", linuxTestExitError(1)
			}
			return "", nil
		}, false},
		{"iptables operational failure", "iptables", func(_ string, args []string) (string, error) {
			if strings.Contains(strings.Join(args, " "), " -C ") {
				return "", errors.New("xtables lock")
			}
			return "", nil
		}, true},
		{"nft listing failure", "nft", func(_ string, args []string) (string, error) {
			if strings.Contains(strings.Join(args, " "), "list ruleset") {
				return "", errors.New("audit unavailable")
			}
			return "", nil
		}, true},
		{"nft exact absence", "nft", func(_ string, _ []string) (string, error) { return "table ip unrelated { }", nil }, false},
		{"nft unrelated canonical queue", "nft", func(_ string, _ []string) (string, error) {
			return "table ip unrelated { chain output { queue flags bypass to 40000 } }", nil
		}, false},
		{"nft residual owned table", "nft", func(_ string, _ []string) (string, error) {
			return "table ip unbound_autotune_test { }", nil
		}, true},
		{"nft residual owned marker", "nft", func(_ string, _ []string) (string, error) {
			return `comment "unbound-autotune-vnext:test"`, nil
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &linuxRunnerStub{paths: map[string]bool{}, runFn: tc.runFn}
			runtime := testLinuxRuntime(t, runner)
			err := runtime.deleteRule(context.Background(), tc.mode, spec)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
func canonicalOwnedNFTTable(spec LinuxNFQueueSpec) string {
	return strings.Join([]string{
		nftTableHeader(spec),
		"chain output {",
		"type filter hook output priority mangle; policy accept;",
		`ip daddr 192.0.2.7 tcp dport 443 queue flags bypass to 40000 comment "unbound-autotune-vnext:test"`,
		"}",
		"}",
	}, "\n")
}

func TestLinuxVerifyRuleScopesExactOwnedNFTTable(t *testing.T) {
	spec := ownedLinuxSpec()
	canonical := canonicalOwnedNFTTable(spec)
	for _, tc := range []struct {
		name    string
		listing string
		wantErr bool
	}{
		{"canonical owned table", canonical, false},
		{"wrong edge", strings.Replace(canonical, "192.0.2.7", "192.0.2.8", 1), true},
		{"wrong port", strings.Replace(canonical, "tcp dport 443", "tcp dport 444", 1), true},
		{"wrong queue", strings.Replace(canonical, "to 40000", "to 40001", 1), true},
		{"wrong marker", strings.Replace(canonical, linuxOwnershipPrefix+":test", linuxOwnershipPrefix+":other", 1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &linuxRunnerStub{paths: map[string]bool{}}
			runner.runFn = func(name string, args []string) (string, error) {
				if name != "nft" || strings.Join(args, " ") != "list table ip unbound_autotune_test" {
					t.Fatalf("verify queried %s %q, not the exact owned table", name, args)
				}
				return tc.listing, nil
			}
			runtime := testLinuxRuntime(t, runner)
			err := runtime.verifyRule(context.Background(), "nft", spec)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestLinuxVerifyRuleCannotCombineSeparateNFTTables(t *testing.T) {
	spec := ownedLinuxSpec()
	ownedTable := strings.Join([]string{
		nftTableHeader(spec),
		"chain output { type filter hook output priority mangle; policy accept;",
		`comment "unbound-autotune-vnext:test"`,
		"}",
		"}",
	}, "\n")
	unrelatedTable := strings.Join([]string{
		"table ip unrelated {",
		"chain output {",
		"ip daddr 192.0.2.7 tcp dport 443 queue flags bypass to 40000",
		"}",
		"}",
	}, "\n")
	runner := &linuxRunnerStub{paths: map[string]bool{}}
	runner.runFn = func(name string, args []string) (string, error) {
		if name != "nft" {
			t.Fatalf("unexpected command: %s %q", name, args)
		}
		switch strings.Join(args, " ") {
		case "list table ip unbound_autotune_test":
			return ownedTable, nil
		case "list ruleset":
			return ownedTable + "\n" + unrelatedTable, nil
		default:
			t.Fatalf("unexpected nft arguments: %q", args)
			return "", nil
		}
	}
	runtime := testLinuxRuntime(t, runner)
	if err := runtime.verifyRule(context.Background(), "nft", spec); err == nil {
		t.Fatal("verification combined fragments from separate nft tables")
	}
	if strings.Contains(strings.Join(runner.calls, "\n"), "list ruleset") {
		t.Fatal("verification consulted unrelated nft tables")
	}
}

func TestLinuxStartFailureRollsBackTrackedRule(t *testing.T) {
	runner := &linuxRunnerStub{paths: map[string]bool{"iptables": true}}
	deleteFails := false
	runner.runFn = func(_ string, args []string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, " -D ") && deleteFails {
			return "", errors.New("delete failed")
		}
		if strings.Contains(joined, " -C ") {
			return "", linuxTestExitError(1)
		}
		return "", nil
	}
	runtime := testLinuxRuntime(t, runner)
	snapshot, err := runtime.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	runtime.startProcess = func(context.Context, string, []string, string, *syscall.SysProcAttr) (*exec.Cmd, error) {
		if runtime.active == nil || !runtime.active.ruleInstalled || runtime.active.processStarted {
			t.Fatal("installed rule was not tracked before process start")
		}
		return nil, errors.New("nfqws start failed")
	}
	candidate := ExecutableCandidate{Plan: backendcap.Plan{Capture: exactCapture(backendcap.CaptureNFQUEUE, strategyir.IPFamilyV4, strategyir.DirectionOutbound, strategyir.PortRange{Start: 443, End: 443})}, TargetEdge: net.ParseIP("192.0.2.7"), TargetFamily: observatory.AddressFamilyIPv4}
	if err := runtime.Activate(context.Background(), candidate); err == nil {
		t.Fatal("start failure passed")
	}
	if runtime.active != nil {
		t.Fatal("successful startup rollback retained ownership")
	}

	deleteFails = true
	if err := runtime.Activate(context.Background(), candidate); err == nil {
		t.Fatal("rollback failure passed")
	}
	if runtime.active == nil || !runtime.active.ruleInstalled || runtime.active.ruleRemoved {
		t.Fatal("failed startup rollback lost installed-rule ownership")
	}
	deleteFails = false
	if err := runtime.Restore(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	if runtime.active != nil {
		t.Fatal("restore did not retry retained startup rule cleanup")
	}
	if err := runtime.VerifyRestored(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxParentDeathAndQueueBypassAreMandatory(t *testing.T) {
	attrs := linuxCandidateSysProcAttr()
	if !attrs.Setpgid || attrs.Pdeathsig != syscall.SIGTERM {
		t.Fatalf("unsafe child process attributes: %#v", attrs)
	}
	args := strings.Join(ownedLinuxSpec().iptablesArgs("-I"), " ")
	if !strings.Contains(args, "--queue-bypass") {
		t.Fatal("NFQUEUE fail-open bypass was removed")
	}
}
