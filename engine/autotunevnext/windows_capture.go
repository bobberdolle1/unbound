package autotunevnext

import (
	"fmt"
	"net"
	"strings"

	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/strategyir"
)

// RenderWindowsTargetCapture intersects typed compiler capture semantics with
// one factual Observatory edge. It never resolves DNS and cannot emit a
// match-all WinDivert filter.
func RenderWindowsTargetCapture(capture backendcap.CapturePlan, edge net.IP, family observatory.AddressFamily) (string, error) {
	if capture.BackendKind != backendcap.CaptureWinDivert {
		return "", fmt.Errorf("expected WINDIVERT capture, got %s", capture.BackendKind)
	}
	if edge == nil || family == "" {
		return "", fmt.Errorf("selected target edge and address family are required")
	}
	if !captureIncludesFamily(capture.IPFamilies, family) || familyForIP(edge) != family {
		return "", fmt.Errorf("selected target edge family is outside compiled capture")
	}
	if capture.Transport != backendcap.CaptureTransportTCP {
		return "", fmt.Errorf("physical Windows target guards support TCP only")
	}
	if len(capture.TCPPorts) == 0 {
		return "", fmt.Errorf("compiled capture has no ports")
	}
	portExpr := make([]string, 0, len(capture.TCPPorts))
	for _, port := range capture.TCPPorts {
		if port.Start <= 0 || port.End < port.Start || port.End > 65535 {
			return "", fmt.Errorf("invalid compiled port range %d-%d", port.Start, port.End)
		}
		if port.Start == port.End {
			portExpr = append(portExpr, fmt.Sprintf("tcp.DstPort == %d", port.Start))
		} else {
			portExpr = append(portExpr, fmt.Sprintf("(tcp.DstPort >= %d and tcp.DstPort <= %d)", port.Start, port.End))
		}
	}
	addressField := "ip.DstAddr"
	if family == observatory.AddressFamilyIPv6 {
		addressField = "ipv6.DstAddr"
	}
	// --wf-raw-filter is ANDed with the typed WinDivert constructor. Keep its
	// semantic direction there; this guard must also admit reverse SYN+ACK,
	// FIN, and RST traffic for the exact selected TCP endpoint.
	outbound := fmt.Sprintf("outbound and %s == %s and (%s)", addressField, edge.String(), strings.Join(portExpr, " or "))
	inboundPorts := strings.ReplaceAll(strings.Join(portExpr, " or "), ".DstPort", ".SrcPort")
	inboundField := strings.ReplaceAll(addressField, ".DstAddr", ".SrcAddr")
	inbound := fmt.Sprintf("inbound and %s == %s and (%s)", inboundField, edge.String(), inboundPorts)
	return "(" + outbound + ") or (" + inbound + ")", nil
}

func captureIncludesFamily(families []strategyir.IPFamily, family observatory.AddressFamily) bool {
	for _, candidate := range families {
		if candidate == strategyir.IPFamilyAny || (candidate == strategyir.IPFamilyV4 && family == observatory.AddressFamilyIPv4) || (candidate == strategyir.IPFamilyV6 && family == observatory.AddressFamilyIPv6) {
			return true
		}
	}
	return false
}
