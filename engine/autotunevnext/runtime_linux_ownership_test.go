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
	return &linuxCandidate{done: done, cancel: func() {}, spec: spec, mode: "iptables", pid: 4242}
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
	queues := queuesInRuleListing("queue num 40000 --queue-num 40001 unrelated-40002")
	if _, ok := queues[40000]; !ok {
		t.Fatal("nft queue reference was not parsed")
	}
	if _, ok := queues[40001]; !ok {
		t.Fatal("iptables queue reference was not parsed")
	}
	if _, ok := queues[40002]; ok {
		t.Fatal("unrelated numeric substring became a queue identity")
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
