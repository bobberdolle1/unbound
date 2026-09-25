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
	Table  string
	Queue  uint16
	Marker string
	Edge   net.IP
	Family observatory.AddressFamily
	Ports  []strategyir.PortRange
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
	if s.Family == observatory.AddressFamilyIPv6 {
		return "ip6"
	}
	return "ip"
}

func (s LinuxNFQueueSpec) nftScript() string {
	family := s.nftFamily()
	address := "ip daddr"
	if family == "ip6" {
		address = "ip6 daddr"
	}
	return fmt.Sprintf("add table %s %s\nadd chain %s %s output { type filter hook output priority mangle; policy accept; }\nadd rule %s %s output %s %s tcp dport %s meta mark and 0x40000000 != 0x40000000 queue num %d bypass comment \"%s\"\n", family, s.Table, family, s.Table, family, s.Table, address, s.Edge.String(), s.nftPorts(), s.Queue, s.Marker)
}

func (s LinuxNFQueueSpec) iptablesArgs(operation string) []string {
	binaryFamily := "iptables"
	if s.Family == observatory.AddressFamilyIPv6 {
		binaryFamily = "ip6tables"
	}
	return []string{binaryFamily, "-t", "mangle", operation, "OUTPUT", "-d", s.Edge.String(), "-p", "tcp", "-m", "multiport", "--dports", s.iptablesPorts(), "-m", "mark", "!", "--mark", "0x40000000/0x40000000", "-m", "comment", "--comment", s.Marker, "-j", "NFQUEUE", "--queue-num", fmt.Sprint(s.Queue), "--queue-bypass"}
}
