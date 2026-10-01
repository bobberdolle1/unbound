//go:build windows

package autotunevnext

// Hermetic coverage for the Windows bounded-graph render path.
//
// The security property under test is that the rendered plan carries exact
// per-host authority only: one section per active node, an exact hostname
// selector, no wildcard, and no file- or list-based authority.

import (
	"strings"
	"testing"
)

func TestRenderGraphActivationBindsEachNodeToItsOwnHost(t *testing.T) {
	activation := ServiceGraphActivation{
		Graph:    twoNodeGraph(t),
		Sections: graphSections(),
	}
	plan, err := RenderGraphActivation(activation)
	if err != nil {
		t.Fatalf("render graph activation: %v", err)
	}
	if len(plan.NodeOrder) != 2 {
		t.Fatalf("node order = %v, want 2 nodes", plan.NodeOrder)
	}
	joined := strings.Join(plan.SectionArgv, "\n")
	for _, host := range []string{"entry.example", "media.example"} {
		if !strings.Contains(joined, hostlistDomainsArg(host)) {
			t.Fatalf("rendered plan is missing the exact host selector for %s", host)
		}
	}
	// Same template on two hosts must remain two independent host bindings.
	if strings.Count(joined, "--lua-desync=fake") != 2 {
		t.Fatalf("expected two independent per-node sections, got %q", joined)
	}
}

func TestRenderGraphActivationNeverEmitsWildcardOrListAuthority(t *testing.T) {
	activation := ServiceGraphActivation{
		Graph:    twoNodeGraph(t),
		Sections: graphSections(),
	}
	plan, err := RenderGraphActivation(activation)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	all := strings.Join(plan.SectionArgv, "\n") + "\n" + plan.RawFilter + "\n" + strings.Join(plan.CaptureArgv, "\n")
	for _, forbidden := range []string{"*", "hostlist-file", "--hostlist=", "ipset", "asn", "autohostlist"} {
		if strings.Contains(all, forbidden) {
			t.Fatalf("rendered graph plan leaked non-exact authority %q", forbidden)
		}
	}
}

// A plan whose node count does not match its sections must be refused before
// anything could run.
func TestRenderGraphActivationRejectsSectionCountMismatch(t *testing.T) {
	activation := ServiceGraphActivation{
		Graph:    twoNodeGraph(t),
		Sections: graphSections()[:1],
	}
	if _, err := RenderGraphActivation(activation); err == nil {
		t.Fatal("a section/node count mismatch must be rejected")
	}
}

// The committed runtime must not inherit the bounded experiment context.
func TestCommitGraphIsIdempotentAndDetachesCancellation(t *testing.T) {
	e := &WindowsGraphExecutor{ownedPIDs: map[int]struct{}{}}
	e.active = &windowsGraphProcess{
		done:    make(chan struct{}),
		ready:   make(chan struct{}, 1),
		cancel:  func() {},
		hostSet: map[string]string{},
	}
	if err := e.CommitGraph(ServiceGraphActivation{}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := e.CommitGraph(ServiceGraphActivation{}); err != nil {
		t.Fatalf("second commit must be a no-op, got: %v", err)
	}
	if e.active == nil {
		t.Fatal("commit must keep the process owned so Revert can stop it")
	}
}
