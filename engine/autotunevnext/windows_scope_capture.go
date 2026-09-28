package autotunevnext

import (
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"

	"unbound/engine/backendcap"
	"unbound/engine/strategyir"
)

// RenderWindowsServiceScopeCapture renders only the current validated exact
// edges. It has no DNS input and rejects partial or malformed scope data.
func RenderWindowsServiceScopeCapture(capture backendcap.CapturePlan, edges []ServiceScopeEdge) (string, error) {
	canonical, err := canonicalCaptureEdges(edges)
	if err != nil {
		return "", err
	}
	if capture.BackendKind != backendcap.CaptureWinDivert || capture.Transport != backendcap.CaptureTransportTCP || len(capture.TCPPorts) == 0 {
		return "", fmt.Errorf("physical Windows service scope supports only TCP WinDivert capture")
	}
	ports := make([]string, 0, len(capture.TCPPorts))
	for _, port := range capture.TCPPorts {
		if port.Start <= 0 || port.End < port.Start || port.End > 65535 {
			return "", fmt.Errorf("invalid compiled port range %d-%d", port.Start, port.End)
		}
		if port.Start == port.End {
			ports = append(ports, fmt.Sprintf("tcp.DstPort == %d", port.Start))
		} else {
			ports = append(ports, fmt.Sprintf("(tcp.DstPort >= %d and tcp.DstPort <= %d)", port.Start, port.End))
		}
	}
	outbound, inbound := make([]string, 0, len(canonical)), make([]string, 0, len(canonical))
	for _, edge := range canonical {
		if !captureIncludesFamily(capture.IPFamilies, edge.Family) {
			return "", fmt.Errorf("scope edge family is outside compiled capture")
		}
		field, reverse := "ip.DstAddr", "ip.SrcAddr"
		if edge.Family == "ipv6" {
			field, reverse = "ipv6.DstAddr", "ipv6.SrcAddr"
		}
		outbound = append(outbound, fmt.Sprintf("%s == %s", field, edge.IP.String()))
		inbound = append(inbound, fmt.Sprintf("%s == %s", reverse, edge.IP.String()))
	}
	portExpr := strings.Join(ports, " or ")
	inboundPortExpr := strings.ReplaceAll(portExpr, ".DstPort", ".SrcPort")
	return "(outbound and (" + strings.Join(outbound, " or ") + ") and (" + portExpr + ")) or (inbound and (" + strings.Join(inbound, " or ") + ") and (" + inboundPortExpr + "))", nil
}

func canonicalCaptureEdges(edges []ServiceScopeEdge) ([]ServiceScopeEdge, error) {
	if len(edges) == 0 {
		return nil, fmt.Errorf("service scope has no edges")
	}
	if len(edges) > MaxServiceScopeEdges {
		return nil, fmt.Errorf("%w: %d > %d", ErrServiceScopeTooLarge, len(edges), MaxServiceScopeEdges)
	}
	seen := make(map[string]struct{}, len(edges))
	result := make([]ServiceScopeEdge, 0, len(edges))
	for _, edge := range edges {
		if edge.IP == nil || edge.Family == "" || familyForIP(edge.IP) != edge.Family {
			return nil, fmt.Errorf("invalid service scope edge")
		}
		ip := append(net.IP(nil), edge.IP...)
		if edge.Family == "ipv4" {
			ip = ip.To4()
		}
		address, ok := netip.AddrFromSlice(ip)
		if !ok || address.IsUnspecified() || address.Is4In6() {
			return nil, fmt.Errorf("invalid service scope address")
		}
		key := string(edge.Family) + "|" + address.String()
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, ServiceScopeEdge{IP: ip, Family: edge.Family})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("service scope has no usable edges")
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Family != result[j].Family {
			return result[i].Family < result[j].Family
		}
		return netip.MustParseAddr(result[i].IP.String()).Less(netip.MustParseAddr(result[j].IP.String()))
	})
	return result, nil
}

var _ = strategyir.DirectionOutbound
