package autotunevnext

import (
	"fmt"
	"net"
	"strings"

	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/strategyir"
)

const linuxOwnershipPrefix = "unbound-autotune-vnext"

// LinuxNFQueueSpec is an owned, exact rule description. It is deliberately
// smaller than CapturePlan: physical v1 supports only factual outbound TCP.
type LinuxNFQueueSpec struct {
	Table     string
	Queue     uint16
	Marker    string
	Edge      net.IP
	Edges     []net.IP
	Family    observatory.AddressFamily
	NFTFamily string
	IPv4Edges []net.IP
	IPv6Edges []net.IP
	Ports     []strategyir.PortRange
}

func NewLinuxNFQueueSpec(capture backendcap.CapturePlan, edge net.IP, family observatory.AddressFamily, table string, queue uint16, marker string) (LinuxNFQueueSpec, error) {
	if capture.BackendKind != backendcap.CaptureNFQUEUE || capture.Transport != backendcap.CaptureTransportTCP || capture.Direction != strategyir.DirectionOutbound {
		return LinuxNFQueueSpec{}, fmt.Errorf("physical Linux v1 supports only outbound TCP NFQUEUE capture")
	}
	if edge == nil || family == "" || familyForIP(edge) != family || !captureIncludesFamily(capture.IPFamilies, family) {
		return LinuxNFQueueSpec{}, fmt.Errorf("selected target edge is not representable by compiled capture")
	}
	if table == "" || queue == 0 || marker == "" || len(capture.TCPPorts) == 0 {
		return LinuxNFQueueSpec{}, fmt.Errorf("incomplete owned NFQUEUE rule specification")
	}
	ports := append([]strategyir.PortRange(nil), capture.TCPPorts...)
	for _, port := range ports {
		if port.Start <= 0 || port.End < port.Start || port.End > 65535 {
			return LinuxNFQueueSpec{}, fmt.Errorf("invalid compiled port range %d-%d", port.Start, port.End)
		}
	}
	return LinuxNFQueueSpec{Table: table, Queue: queue, Marker: marker, Edge: append(net.IP(nil), edge...), Family: family, Ports: ports}, nil
}

// NewLinuxServiceScopeNFQueueSpec owns one exact nft rule over a bounded
// same-family edge set, or two exact rules in one inet table for a mixed
// family scope. It never creates or reuses a persistent system set.
func NewLinuxServiceScopeNFQueueSpec(capture backendcap.CapturePlan, edges []ServiceScopeEdge, table string, queue uint16, marker string) (LinuxNFQueueSpec, error) {
	canonical, err := canonicalCaptureEdges(edges)
	if err != nil {
		return LinuxNFQueueSpec{}, err
	}
	var ipv4, ipv6 []net.IP
	for _, edge := range canonical {
		if !captureIncludesFamily(capture.IPFamilies, edge.Family) {
			return LinuxNFQueueSpec{}, fmt.Errorf("unsupported Linux service scope address family")
		}
		if edge.Family == observatory.AddressFamilyIPv4 {
			ipv4 = append(ipv4, append(net.IP(nil), edge.IP...))
			continue
		}
		if edge.Family == observatory.AddressFamilyIPv6 {
			ipv6 = append(ipv6, append(net.IP(nil), edge.IP...))
			continue
		}
		return LinuxNFQueueSpec{}, fmt.Errorf("invalid Linux service scope address family")
	}
	first := canonical[0]
	spec, err := NewLinuxNFQueueSpec(capture, first.IP, first.Family, table, queue, marker)
	if err != nil {
		return LinuxNFQueueSpec{}, err
	}
	if len(ipv4) > 0 && len(ipv6) > 0 {
		spec.NFTFamily = "inet"
		spec.IPv4Edges = ipv4
		spec.IPv6Edges = ipv6
		return spec, nil
	}
	if len(ipv4) > 0 {
		spec.Edges = ipv4
		return spec, nil
	}
	spec.Edges = ipv6
	return spec, nil
}

func (s LinuxNFQueueSpec) iptablesPorts() string {
	out := make([]string, 0, len(s.Ports))
	for _, port := range s.Ports {
		if port.Start == port.End {
			out = append(out, fmt.Sprint(port.Start))
		} else {
			out = append(out, fmt.Sprintf("%d:%d", port.Start, port.End))
		}
	}
	return strings.Join(out, ",")
}

func (s LinuxNFQueueSpec) nftPorts() string {
	out := make([]string, 0, len(s.Ports))
	for _, port := range s.Ports {
		if port.Start == port.End {
			out = append(out, fmt.Sprint(port.Start))
		} else {
			out = append(out, fmt.Sprintf("%d-%d", port.Start, port.End))
		}
	}
	return "{ " + strings.Join(out, ", ") + " }"
}

func (s LinuxNFQueueSpec) nftFamily() string {
	if s.NFTFamily != "" {
		return s.NFTFamily
	}
	if s.Family == observatory.AddressFamilyIPv6 {
		return "ip6"
	}
	return "ip"
}

func nftAddressExpression(family string, edges []net.IP) string {
	address := "ip daddr"
	if family == "ip6" {
		address = "ip6 daddr"
	}
	values := make([]string, 0, len(edges))
	for _, edge := range edges {
		values = append(values, edge.String())
	}
	addressExpr := values[0]
	if len(values) > 1 {
		addressExpr = "{ " + strings.Join(values, ", ") + " }"
	}
	return address + " " + addressExpr
}

func (s LinuxNFQueueSpec) nftAddressExpressions() []string {
	if s.nftFamily() == "inet" {
		return []string{
			nftAddressExpression("ip", s.IPv4Edges),
			nftAddressExpression("ip6", s.IPv6Edges),
		}
	}
	edges := s.Edges
	if len(edges) == 0 {
		edges = []net.IP{s.Edge}
	}
	return []string{nftAddressExpression(s.nftFamily(), edges)}
}

func (s LinuxNFQueueSpec) nftScript() string {
	family := s.nftFamily()
	var rules strings.Builder
	for _, address := range s.nftAddressExpressions() {
		fmt.Fprintf(&rules, "add rule %s %s output %s tcp dport %s meta mark and 0x40000000 != 0x40000000 queue num %d bypass comment %q\n", family, s.Table, address, s.nftPorts(), s.Queue, s.Marker)
	}
	return fmt.Sprintf("add table %s %s\nadd chain %s %s output { type filter hook output priority mangle; policy accept; }\n%s", family, s.Table, family, s.Table, rules.String())
}

func (s LinuxNFQueueSpec) iptablesArgs(operation string) []string {
	binaryFamily := "iptables"
	if s.Family == observatory.AddressFamilyIPv6 {
		binaryFamily = "ip6tables"
	}
	return []string{binaryFamily, "-t", "mangle", operation, "OUTPUT", "-d", s.Edge.String(), "-p", "tcp", "-m", "multiport", "--dports", s.iptablesPorts(), "-m", "mark", "!", "--mark", "0x40000000/0x40000000", "-m", "comment", "--comment", s.Marker, "-j", "NFQUEUE", "--queue-num", fmt.Sprint(s.Queue), "--queue-bypass"}
}
