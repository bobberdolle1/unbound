//go:build linux

package providers

import (
	"fmt"
	"os/exec"
	"strings"
)

// linuxFirewall installs and removes the netfilter rules that divert traffic
// into Unbound's NFQUEUE.
//
// Two backends exist because distributions differ: most still ship the
// iptables command (usually as an iptables-nft shim), but minimal and
// container-oriented images increasingly ship only nft. The previous
// implementation assumed iptables was present and, when it was not, failed
// with a bare "exec: iptables: executable file not found" that gave the user
// nothing to act on.
type linuxFirewall interface {
	Name() string
	Apply(filters []packetFilter) error
	Flush() error
}

func newLinuxFirewall() (linuxFirewall, error) {
	if _, err := exec.LookPath("iptables"); err == nil {
		return &iptablesFirewall{}, nil
	}
	if _, err := exec.LookPath("nft"); err == nil {
		return &nftablesFirewall{}, nil
	}
	return nil, fmt.Errorf(
		"не найдены ни iptables, ни nft; установите iptables (пакет iptables) " +
			"или nftables, чтобы Unbound мог направлять трафик в NFQUEUE")
}

// run executes a firewall command and folds its output into the error, which
// is where iptables reports the actual reason (missing kernel module, no
// permission, unknown match).
func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
	}
	return nil
}

// ─── iptables ───────────────────────────────────────────────────────────────

type iptablesFirewall struct {
	// applied records the exact rule specs installed, so Flush deletes
	// precisely what Apply added. Reconstructing the delete command by
	// string-replacing "-I" with "-D" (as the previous code did) silently
	// failed whenever a rule was formatted even slightly differently.
	applied []appliedRule
}

type appliedRule struct {
	cmd  string // "iptables" or "ip6tables"
	spec []string
}

func (f *iptablesFirewall) Name() string { return "iptables" }

// ruleSpec renders a packetFilter as iptables arguments after the chain.
func ruleSpec(pf packetFilter) []string {
	spec := []string{
		"-p", pf.Proto,
		"-m", "multiport", "--dports", pf.Ports,
	}
	if pf.HandshakeOnly && pf.Proto == "tcp" {
		spec = append(spec,
			"-m", "connbytes",
			"--connbytes-dir", "original",
			"--connbytes-mode", "packets",
			"--connbytes", "1:6")
	}
	spec = append(spec,
		"-m", "mark", "!", "--mark", fwmarkMatch,
		"-j", "NFQUEUE",
		"--queue-num", nfqueueNum,
		// Without --queue-bypass, an nfqws crash turns every matched packet
		// into a black hole and takes the user's connection down with it.
		"--queue-bypass")
	return spec
}

func (f *iptablesFirewall) Apply(filters []packetFilter) error {
	// IPv6 is best-effort: plenty of hosts have it disabled, and failing the
	// whole start because ip6tables is unavailable would be wrong. But when
	// IPv6 *is* up, skipping it lets traffic bypass the bypass.
	ip6Available := false
	if _, err := exec.LookPath("ip6tables"); err == nil {
		if err := exec.Command("ip6tables", "-t", "mangle", "-S").Run(); err == nil {
			ip6Available = true
		}
	}

	for _, pf := range filters {
		spec := ruleSpec(pf)

		args := append([]string{"-t", "mangle", "-I", "POSTROUTING"}, spec...)
		if err := run("iptables", args...); err != nil {
			// Roll back whatever we managed to install; leaving half a
			// ruleset behind would queue traffic to a process that is never
			// going to start.
			_ = f.Flush()
			return err
		}
		f.applied = append(f.applied, appliedRule{cmd: "iptables", spec: spec})

		if ip6Available {
			args6 := append([]string{"-t", "mangle", "-I", "POSTROUTING"}, spec...)
			if err := run("ip6tables", args6...); err == nil {
				f.applied = append(f.applied, appliedRule{cmd: "ip6tables", spec: spec})
			}
		}
	}

	if len(f.applied) == 0 {
		return fmt.Errorf("не установлено ни одного правила iptables")
	}
	return nil
}

func (f *iptablesFirewall) Flush() error {
	var errs []string
	// Delete in reverse insertion order so indices stay valid.
	for i := len(f.applied) - 1; i >= 0; i-- {
		r := f.applied[i]
		args := append([]string{"-t", "mangle", "-D", "POSTROUTING"}, r.spec...)
		if err := run(r.cmd, args...); err != nil {
			errs = append(errs, err.Error())
		}
	}
	f.applied = nil

	if len(errs) > 0 {
		return fmt.Errorf("не удалось удалить правила: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ─── nftables ───────────────────────────────────────────────────────────────

// nftTable is a dedicated table so Flush can drop everything Unbound added in
// a single atomic operation without touching the user's own ruleset.
const nftTable = "unbound"

type nftablesFirewall struct {
	created bool
}

func (f *nftablesFirewall) Name() string { return "nftables" }

// nftPortSet converts an iptables multiport list ("443,3478,50000:65535") into
// an nft set literal ("{ 443, 3478, 50000-65535 }").
func nftPortSet(ports string) string {
	parts := strings.Split(ports, ",")
	for i, p := range parts {
		parts[i] = strings.ReplaceAll(strings.TrimSpace(p), ":", "-")
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

func (f *nftablesFirewall) Apply(filters []packetFilter) error {
	var b strings.Builder
	// inet covers IPv4 and IPv6 from one table, so IPv6 traffic cannot slip
	// past the bypass.
	fmt.Fprintf(&b, "add table inet %s\n", nftTable)
	fmt.Fprintf(&b, "add chain inet %s postrouting { type filter hook postrouting priority mangle; policy accept; }\n", nftTable)

	for _, pf := range filters {
		rule := fmt.Sprintf("add rule inet %s postrouting %s dport %s",
			nftTable, pf.Proto, nftPortSet(pf.Ports))
		if pf.HandshakeOnly && pf.Proto == "tcp" {
			rule += " ct original packets 1-6"
		}
		rule += fmt.Sprintf(" meta mark and 0x40000000 != 0x40000000 queue num %s bypass", nfqueueNum)
		b.WriteString(rule + "\n")
	}

	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(b.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = f.Flush()
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("nft: %w", err)
		}
		return fmt.Errorf("nft: %s", msg)
	}

	f.created = true
	return nil
}

func (f *nftablesFirewall) Flush() error {
	if !f.created {
		return nil
	}
	f.created = false
	// Deleting our own table removes every rule at once and cannot disturb
	// rules the user or their firewall manager installed elsewhere.
	if err := run("nft", "delete", "table", "inet", nftTable); err != nil {
		if strings.Contains(err.Error(), "No such file or directory") {
			return nil // already gone
		}
		return err
	}
	return nil
}
