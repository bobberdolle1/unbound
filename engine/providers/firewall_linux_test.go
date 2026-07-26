//go:build linux

package providers

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The rest of the firewall tests compare generated arguments as strings, which
// cannot catch a spec the kernel rejects — a misplaced '=' in --connbytes-dir,
// a match that needs a module the running kernel lacks, or an ordering the
// parser refuses. These tests hand the generated rules to the real iptables and
// nft.
//
// They are opt-in because they touch the host firewall:
//
//	sudo UNBOUND_FIREWALL_TEST=1 go test ./engine/providers/ -run Live -v
//
// Even then they never modify a chain that carries traffic. Rules go into a
// dedicated user-defined chain with nothing jumping to it, so the kernel
// validates and stores them while no packet is ever matched.

const testChain = "UNBOUND_RULETEST"

func requireLiveFirewall(t *testing.T, tool string) {
	t.Helper()

	if os.Getenv("UNBOUND_FIREWALL_TEST") == "" {
		t.Skip("set UNBOUND_FIREWALL_TEST=1 to run tests that touch the host firewall")
	}
	if os.Geteuid() != 0 {
		t.Skip("needs root to modify firewall rules")
	}
	if _, err := exec.LookPath(tool); err != nil {
		t.Skipf("%s not installed", tool)
	}
}

// TestLiveIPTablesAcceptsEveryBuiltinRule checks that the kernel accepts, and
// then cleanly deletes, the exact rule spec each built-in profile generates.
func TestLiveIPTablesAcceptsEveryBuiltinRule(t *testing.T) {
	requireLiveFirewall(t, "iptables")

	if out, err := exec.Command("iptables", "-t", "mangle", "-N", testChain).CombinedOutput(); err != nil {
		t.Fatalf("could not create test chain: %s", strings.TrimSpace(string(out)))
	}
	t.Cleanup(func() {
		_ = exec.Command("iptables", "-t", "mangle", "-F", testChain).Run()
		_ = exec.Command("iptables", "-t", "mangle", "-X", testChain).Run()
	})

	for name, profile := range builtinProfiles {
		for _, filter := range profile.Filters {
			spec := ruleSpec(filter)

			add := append([]string{"-t", "mangle", "-A", testChain}, spec...)
			if out, err := exec.Command("iptables", add...).CombinedOutput(); err != nil {
				t.Errorf("%s: kernel rejected %s/%s rule: %s",
					name, filter.Proto, filter.Ports, strings.TrimSpace(string(out)))
				continue
			}

			// Flush() deletes by exact spec rather than by index. If iptables
			// normalises any argument on the way in, the delete silently fails
			// and the rule is stranded, so verify the round trip.
			del := append([]string{"-t", "mangle", "-D", testChain}, spec...)
			if out, err := exec.Command("iptables", del...).CombinedOutput(); err != nil {
				t.Errorf("%s: rule installed but could not be deleted by the same spec: %s",
					name, strings.TrimSpace(string(out)))
			}
		}
	}

	// Everything added must have been removed again.
	out, err := exec.Command("iptables", "-t", "mangle", "-S", testChain).Output()
	if err != nil {
		t.Fatalf("could not list the test chain: %v", err)
	}
	if lines := strings.Count(strings.TrimSpace(string(out)), "\n"); lines != 0 {
		t.Errorf("test chain still holds rules after cleanup:\n%s", out)
	}
}

// TestLiveNftAcceptsGeneratedRuleset feeds the nftables backend's ruleset to
// nft. The chain deliberately has no hook, so it is never traversed.
func TestLiveNftAcceptsGeneratedRuleset(t *testing.T) {
	requireLiveFirewall(t, "nft")

	const table = "unbound_ruletest"
	t.Cleanup(func() {
		_ = exec.Command("nft", "delete", "table", "inet", table).Run()
	})

	var b strings.Builder
	b.WriteString("add table inet " + table + "\n")
	b.WriteString("add chain inet " + table + " test\n")
	for _, pf := range builtinProfiles["Ultimate Bypass (Multi-Strategy)"].Filters {
		b.WriteString(nftRule(table, "test", pf) + "\n")
	}

	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(b.String())
	out, err := cmd.CombinedOutput()
	if err == nil {
		return
	}

	msg := strings.TrimSpace(string(out))

	// nft reports a missing kernel expression as ENOENT at commit time, which
	// is an environment limitation rather than a bad ruleset: containers
	// commonly lack nft_queue even where the iptables compat target works.
	// A genuine syntax error looks different ("syntax error, unexpected ...").
	if strings.Contains(msg, "No such file or directory") {
		t.Skipf("kernel lacks nft queue support, ruleset parsed but could not be committed:\n%s", msg)
	}
	t.Fatalf("nft rejected the generated ruleset:\n%s", msg)
}
