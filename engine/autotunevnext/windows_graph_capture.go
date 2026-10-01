package autotunevnext

import (
	"fmt"
	"sort"
	"strings"
)

// Windows bounded service-graph capture rendering.
//
// A graph needs one host-scoped Zapret2 section per node inside a single winws2
// invocation, separated by --new, over one process-global WinDivert filter that
// covers the union of validated exact edges. Phase 4 proved this exact shape is
// required: binding a proven strategy to the ENTRY host alone does not affect
// MEDIA delivery, so each active service host needs its own section.
//
// This file is a pure, deterministic renderer. It performs no DNS, spawns
// nothing and trusts nothing: every section must already be bound to its own
// node's exact hostname, and the filter is built only from validated edges.

// ServiceGraphSection is one node's compiled, asset-materialised engine argv,
// together with the exact host it is bound to.
type ServiceGraphSection struct {
	NodeID string
	Host   string
	Argv   []string
}

// WindowsServiceGraphPlan is the rendered, still-inert plan. It is not an
// execution authority: only a backend runtime that has verified ownership may
// turn it into a running capture.
type WindowsServiceGraphPlan struct {
	// SectionArgv contains every node section joined by --new.
	SectionArgv []string
	// CaptureArgv contains the single process-global WinDivert filter arguments.
	CaptureArgv []string
	// RawFilter is the exact union filter, for evidence and verification.
	RawFilter string
	// NodeOrder is the deterministic node order used for the sections.
	NodeOrder []string
}

// hostlistDomainsArg returns the exact-host selector argument for a section.
func hostlistDomainsArg(host string) string {
	return "--hostlist-domains=^" + host
}

// forbiddenSectionArgv are argv elements that would widen a section beyond the
// exact host and edge authority this graph proves. A graph section is exactly
// host-scoped capture, so any of these is a hard rejection.
var forbiddenSectionArgv = map[string]bool{
	"--wf-l3": true, "--wf-tcp-in": true, "--wf-tcp-out": true,
	"--wf-udp-in": true, "--wf-udp-out": true, "--wf-raw-filter": true,
	"--ipset": true, "--hostlist": true, "--hostlist-auto": true,
	"--debug": true, "--log": true,
}

func isForbiddenSectionArg(arg string) bool {
	if forbiddenSectionArgv[arg] {
		return true
	}
	for _, prefix := range []string{"--wf-", "--ipset=", "--ipset-", "--hostlist=", "--hostlist-auto=", "--debug", "--log"} {
		if len(arg) >= len(prefix) && arg[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// RenderWindowsServiceGraphCapture renders one exact WinDivert capture covering
// the union of validated edges of the graph's ACTIVE nodes, plus one host-scoped
// section per active node.
//
// Fail-closed properties enforced here:
//   - only ACTIVE nodes are rendered, so a validated-but-not-activated node can
//     never be smuggled into an active capture;
//   - every section must carry exactly one exact host selector for its own node;
//   - no section may carry capture-level or list-level authority;
//   - the filter is TCP-only and derived solely from validated edges.
func RenderWindowsServiceGraphCapture(g ServiceGraph, sections []ServiceGraphSection) (WindowsServiceGraphPlan, error) {
	active, err := g.ActiveCaptureGraph()
	if err != nil {
		return WindowsServiceGraphPlan{}, err
	}
	if len(sections) != len(active.Nodes) {
		return WindowsServiceGraphPlan{}, fmt.Errorf("%w: %d sections for %d active nodes", ErrServiceGraphNodeCount, len(sections), len(active.Nodes))
	}

	byNode := make(map[string]ServiceGraphSection, len(sections))
	for _, s := range sections {
		if _, dup := byNode[s.NodeID]; dup {
			return WindowsServiceGraphPlan{}, fmt.Errorf("%w: duplicate section %q", ErrServiceGraphDuplicate, s.NodeID)
		}
		byNode[s.NodeID] = s
	}

	plan := WindowsServiceGraphPlan{}
	for _, node := range active.Nodes {
		section, ok := byNode[node.ID]
		if !ok {
			return WindowsServiceGraphPlan{}, fmt.Errorf("%w: no section for node %q", ErrServiceGraphBinding, node.ID)
		}
		host := node.Hostname()
		if section.Host != host {
			return WindowsServiceGraphPlan{}, fmt.Errorf("%w: section %q bound to %q, node target is %q", ErrServiceGraphBinding, node.ID, section.Host, host)
		}
		if err := validateExactHostSection(section.Argv, host); err != nil {
			return WindowsServiceGraphPlan{}, fmt.Errorf("%w: node %q: %v", ErrServiceGraphBinding, node.ID, err)
		}
		plan.NodeOrder = append(plan.NodeOrder, node.ID)
	}
	sort.Strings(plan.NodeOrder)

	// One section per active node, deterministically ordered, joined by --new.
	for i, nodeID := range plan.NodeOrder {
		if i > 0 {
			plan.SectionArgv = append(plan.SectionArgv, "--new")
		}
		plan.SectionArgv = append(plan.SectionArgv, byNode[nodeID].Argv...)
	}

	filter, err := renderWindowsGraphRawFilter(active)
	if err != nil {
		return WindowsServiceGraphPlan{}, err
	}
	plan.RawFilter = filter
	plan.CaptureArgv = []string{"--wf-l3=ipv4,ipv6", "--wf-tcp-out=443", "--wf-raw-filter=" + filter}
	return plan, nil
}

// validateExactHostSection rejects any section that is not exactly host-scoped.
func validateExactHostSection(argv []string, host string) error {
	if len(argv) == 0 {
		return fmt.Errorf("empty section argv")
	}
	expected := hostlistDomainsArg(host)
	found := 0
	for _, arg := range argv {
		if isForbiddenSectionArg(arg) {
			return fmt.Errorf("section argv carries capture or list authority: %s", arg)
		}
		if arg == "--new" {
			return fmt.Errorf("section argv must not embed a nested --new")
		}
		if strings.HasPrefix(arg, "--hostlist-domains=") {
			if arg != expected {
				return fmt.Errorf("section host selector is %q, expected %q", arg, expected)
			}
			found++
		}
	}
	if found != 1 {
		return fmt.Errorf("section must carry exactly one exact host selector, found %d", found)
	}
	return nil
}

// renderWindowsGraphRawFilter builds the single union filter over every active
// node's validated edges. It is a flat OR over exact addresses only: no CIDR, no
// suffix-derived range, no wildcard.
func renderWindowsGraphRawFilter(g ServiceGraph) (string, error) {
	edges, err := g.UnionEdges()
	if err != nil {
		return "", err
	}
	outbound := make([]string, 0, len(edges))
	inbound := make([]string, 0, len(edges))
	for _, edge := range edges {
		field, reverse := "ip.DstAddr", "ip.SrcAddr"
		if edge.Family == "ipv6" {
			field, reverse = "ipv6.DstAddr", "ipv6.SrcAddr"
		}
		outbound = append(outbound, fmt.Sprintf("%s == %s", field, edge.IP.String()))
		inbound = append(inbound, fmt.Sprintf("%s == %s", reverse, edge.IP.String()))
	}
	return "(outbound and (" + strings.Join(outbound, " or ") + ") and tcp.DstPort == 443) or (inbound and (" + strings.Join(inbound, " or ") + ") and tcp.SrcPort == 443)", nil
}
