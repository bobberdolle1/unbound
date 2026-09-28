package autotunevnext

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"

	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/strategyir"
)

type scopeResolverFixture struct {
	addresses []netip.Addr
	err       error
}

func (f scopeResolverFixture) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return f.addresses, f.err
}

func scopeTarget() Target {
	return Target{URL: "https://www.youtube.com/generate_204", Transport: observatory.TransportTCP, AddressFamily: observatory.AddressFamilyAny}
}

func TestResolveServiceScopeCanonicalDeduplicated(t *testing.T) {
	scope, err := ResolveServiceScope(context.Background(), scopeTarget(), scopeResolverFixture{addresses: []netip.Addr{netip.MustParseAddr("2001:db8::2"), netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("2001:db8::1")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(scope.Edges) != 4 || scope.Edges[0].IP.String() != "192.0.2.1" || scope.Edges[1].IP.String() != "192.0.2.2" || scope.Edges[2].IP.String() != "2001:db8::1" || scope.Edges[3].IP.String() != "2001:db8::2" {
		t.Fatalf("non-canonical scope: %#v", scope.Edges)
	}
	if scope.Fingerprint() == "" {
		t.Fatal("missing deterministic scope fingerprint")
	}
}

func TestResolveServiceScopeFailsClosed(t *testing.T) {
	tooMany := make([]netip.Addr, MaxServiceScopeEdges+1)
	for i := range tooMany {
		tooMany[i] = netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 1)})
	}
	if _, err := ResolveServiceScope(context.Background(), scopeTarget(), scopeResolverFixture{addresses: tooMany}); !errors.Is(err, ErrServiceScopeTooLarge) {
		t.Fatalf("overflow err=%v", err)
	}
	if _, err := ResolveServiceScope(context.Background(), scopeTarget(), scopeResolverFixture{}); !errors.Is(err, ErrServiceScopeEmpty) {
		t.Fatalf("empty err=%v", err)
	}
	if _, err := ResolveServiceScope(context.Background(), scopeTarget(), scopeResolverFixture{addresses: []netip.Addr{netip.IPv4Unspecified()}}); !errors.Is(err, ErrServiceScopeInvalid) {
		t.Fatalf("invalid err=%v", err)
	}
}

func TestServiceScopeSubsetNeverAdmitsNewEdge(t *testing.T) {
	validated, _ := ResolveServiceScope(context.Background(), scopeTarget(), scopeResolverFixture{addresses: []netip.Addr{netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2")}})
	current, _ := ResolveServiceScope(context.Background(), scopeTarget(), scopeResolverFixture{addresses: []netip.Addr{netip.MustParseAddr("192.0.2.1")}})
	if !current.IsSubsetOf(validated) {
		t.Fatal("removed edge rejected")
	}
	newEdge, _ := ResolveServiceScope(context.Background(), scopeTarget(), scopeResolverFixture{addresses: []netip.Addr{netip.MustParseAddr("192.0.2.3")}})
	if newEdge.IsSubsetOf(validated) {
		t.Fatal("new edge trusted")
	}
}

func TestAggregateServiceScope(t *testing.T) {
	fixed := aggregateServiceScope([]EdgeEvidence{{State: EdgeFixedByProfile}, {State: EdgeUnaffectedReachable}})
	if fixed.Status != ServiceScopeVerifiedFixed {
		t.Fatalf("fixed=%+v", fixed)
	}
	for _, edge := range []EdgeEvidence{{State: EdgeStillFailing}, {State: EdgeTargetRegression}, {State: EdgeInconclusive}} {
		got := aggregateServiceScope([]EdgeEvidence{{State: EdgeFixedByProfile}, edge})
		if got.Status == ServiceScopeVerifiedFixed {
			t.Fatalf("unsafe aggregate=%+v", got)
		}
	}
}

func TestRenderWindowsServiceScopeCaptureExactDeterministic(t *testing.T) {
	capture := exactCapture(backendcap.CaptureWinDivert, strategyir.IPFamilyAny, strategyir.DirectionOutbound, strategyir.PortRange{Start: 443, End: 443})
	edges := []ServiceScopeEdge{{IP: net.ParseIP("2001:db8::1"), Family: observatory.AddressFamilyIPv6}, {IP: net.ParseIP("192.0.2.2"), Family: observatory.AddressFamilyIPv4}, {IP: net.ParseIP("192.0.2.1"), Family: observatory.AddressFamilyIPv4}, {IP: net.ParseIP("192.0.2.2"), Family: observatory.AddressFamilyIPv4}}
	filter, err := RenderWindowsServiceScopeCapture(capture, edges)
	if err != nil || !strings.Contains(filter, "ip.DstAddr == 192.0.2.1") || !strings.Contains(filter, "ip.DstAddr == 192.0.2.2") || !strings.Contains(filter, "ipv6.DstAddr == 2001:db8::1") {
		t.Fatalf("filter=%q err=%v", filter, err)
	}
	if _, err := RenderWindowsServiceScopeCapture(capture, nil); err == nil {
		t.Fatal("accepted empty scope")
	}
}
