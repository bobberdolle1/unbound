package autotunevnext

import (
	"fmt"
	"net"
	"net/netip"
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

func nftRulesForFamily(listing, family string) []string {
	needle := family + " daddr"
	rules := make([]string, 0, 2)
	for from := 0; from < len(listing); {
		offset := strings.Index(listing[from:], needle)
		if offset < 0 {
			break
		}
		address := from + offset
		start := strings.LastIndexAny(listing[:address], ";\n") + 1
		end := len(listing)
		if offset := strings.IndexAny(listing[address:], ";\n"); offset >= 0 {
			end = address + offset
		}
		rules = append(rules, listing[start:end])
		from = address + len(needle)
	}
	return rules
}

func nftRuleAddressSet(rule, family string) (map[string]struct{}, bool) {
	needle := family + " daddr"
	start := strings.Index(rule, needle)
	if start < 0 {
		return nil, false
	}
	value := strings.TrimSpace(rule[start+len(needle):])
	if value == "" {
		return nil, false
	}
	var values []string
	if value[0] == '{' {
		end := strings.IndexByte(value, '}')
		if end < 0 {
			return nil, false
		}
		values = strings.Split(value[1:end], ",")
	} else {
		fields := strings.Fields(value)
		if len(fields) == 0 {
			return nil, false
		}
		values = []string{fields[0]}
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		address, err := netip.ParseAddr(strings.TrimSpace(value))
		if err != nil || address.Is4In6() || (family == "ip" && !address.Is4()) || (family == "ip6" && !address.Is6()) {
			return nil, false
		}
		result[address.String()] = struct{}{}
	}
	return result, len(result) == len(values)
}

func nftRuleHasExpectedAddresses(rule, family string, edges []net.IP) bool {
	got, ok := nftRuleAddressSet(rule, family)
	if !ok || len(got) != len(edges) {
		return false
	}
	for _, edge := range edges {
		address, ok := netip.AddrFromSlice(edge)
		if !ok {
			return false
		}
		if _, ok := got[address.Unmap().String()]; !ok {
			return false
		}
	}
	return true
}

func nftRuleHasExpectedPorts(rule string, spec LinuxNFQueueSpec) bool {
	normalized := strings.ReplaceAll(strings.ToLower(rule), " ", "")
	expected := strings.ReplaceAll("tcp dport "+spec.nftPorts(), " ", "")
	if strings.Contains(normalized, expected) {
		return true
	}
	return len(spec.Ports) == 1 && spec.Ports[0].Start == spec.Ports[0].End && strings.Contains(normalized, fmt.Sprintf("tcpdport%d", spec.Ports[0].Start))
}

func nftRuleHasCompleteSemantics(rule, family string, edges []net.IP, spec LinuxNFQueueSpec) bool {
	normalized := strings.ReplaceAll(strings.ToLower(rule), " ", "")
	return nftRuleHasExpectedAddresses(rule, family, edges) &&
		nftRuleHasExpectedPorts(rule, spec) &&
		strings.Contains(normalized, "metamarkand0x40000000!=0x40000000") &&
		strings.Contains(normalized, fmt.Sprintf("queuenum%dbypass", spec.Queue)) &&
		strings.Contains(rule, fmt.Sprintf("comment %q", spec.Marker))
}

func verifyNFTRuleSemantics(listing string, spec LinuxNFQueueSpec) error {
	families := []struct {
		name  string
		edges []net.IP
	}{
		{name: "ip", edges: spec.IPv4Edges},
		{name: "ip6", edges: spec.IPv6Edges},
	}
	if spec.nftFamily() != "inet" {
		families = families[:1]
		families[0].name = spec.nftFamily()
		families[0].edges = spec.Edges
		if len(families[0].edges) == 0 {
			families[0].edges = []net.IP{spec.Edge}
		}
	}
	for _, family := range families {
		matched := false
		for _, rule := range nftRulesForFamily(listing, family.name) {
			if nftRuleHasCompleteSemantics(rule, family.name, family.edges, spec) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("owned nft %s rule lacks complete exact semantics", family.name)
		}
	}
	return nil
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
