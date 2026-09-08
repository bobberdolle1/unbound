package engine

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)
const MaxTargetIPs = 16

// TargetFilterConfig holds parameters for building a strict WinDivert raw filter.
type TargetFilterConfig struct {
	TargetHost string
	Ports      []int
	Protocol   string // "tcp", "udp", "both"
}

// BuildIsolatedWinDivertFilter resolves targetHost and generates a strict, safe WinDivert filter expression.
// It guarantees that traffic to other destinations will NOT be intercepted by the experimental candidate.
func BuildIsolatedWinDivertFilter(ctx context.Context, cfg TargetFilterConfig) (string, []net.IP, error) {
	if cfg.TargetHost == "" {
		return "", nil, errors.New("target host cannot be empty")
	}
	cleanHost := extractHost(cfg.TargetHost)
	if cleanHost == "" {
		cleanHost = strings.TrimSpace(cfg.TargetHost)
	}
	if cleanHost == "" {
		return "", nil, errors.New("invalid target host")
	}

	// Resolve IPs
	var ips []net.IP
	if parsed := net.ParseIP(cleanHost); parsed != nil {
		ips = append(ips, parsed)
	} else {
		resolved, err := net.DefaultResolver.LookupIP(ctx, "ip", cleanHost)
		if err != nil {
			return "", nil, fmt.Errorf("failed to resolve target %s: %w", cleanHost, err)
		}
		ips = resolved
	}

	if len(ips) == 0 {
		return "", nil, fmt.Errorf("no IP addresses resolved for %s", cleanHost)
	}
	if len(ips) > MaxTargetIPs {
		ips = ips[:MaxTargetIPs]
	}

	ports := append([]int(nil), cfg.Ports...)
	if _, pStr, err := net.SplitHostPort(strings.TrimSpace(cfg.TargetHost)); err == nil {
		if pNum, err := strconv.Atoi(pStr); err == nil && pNum > 0 {
			hasPort := false
			for _, p := range ports {
				if p == pNum {
					hasPort = true
					break
				}
			}
			if !hasPort {
				ports = append(ports, pNum)
			}
		}
	}
	if len(ports) == 0 {
		ports = []int{443}
	}

	filter, err := GenerateWinDivertFilterForIPs(ips, ports, cfg.Protocol)
	if err != nil {
		return "", nil, err
	}

	return filter, ips, nil
}

// GenerateWinDivertFilterForIPs constructs the exact WinDivert filter string.
// Syntax: ((tcp.DstPort == 443 or tcp.SrcPort == 443) and (ip.DstAddr == A or ip.SrcAddr == A or ...))
func GenerateWinDivertFilterForIPs(ips []net.IP, ports []int, proto string) (string, error) {
	if len(ips) == 0 {
		return "", errors.New("IP list cannot be empty (preventing match-all filter)")
	}
	if len(ports) == 0 {
		return "", errors.New("port list cannot be empty")
	}

	var portClauses []string
	protoLower := strings.ToLower(proto)
	if protoLower == "" {
		protoLower = "tcp"
	}

	for _, port := range ports {
		if port <= 0 || port > 65535 {
			return "", fmt.Errorf("invalid port number: %d", port)
		}
		if protoLower == "tcp" || protoLower == "both" {
			portClauses = append(portClauses, fmt.Sprintf("(tcp.DstPort == %d or tcp.SrcPort == %d)", port, port))
		}
		if protoLower == "udp" || protoLower == "both" {
			portClauses = append(portClauses, fmt.Sprintf("(udp.DstPort == %d or udp.SrcPort == %d)", port, port))
		}
	}

	var v4Clauses []string
	var v6Clauses []string

	for _, ip := range ips {
		if ip == nil {
			return "", errors.New("nil IP in target list")
		}
		if ip4 := ip.To4(); ip4 != nil {
			ipStr := ip4.String()
			v4Clauses = append(v4Clauses, fmt.Sprintf("ip.DstAddr == %s or ip.SrcAddr == %s", ipStr, ipStr))
		} else if ip16 := ip.To16(); ip16 != nil {
			ipStr := ip16.String()
			v6Clauses = append(v6Clauses, fmt.Sprintf("ipv6.DstAddr == %s or ipv6.SrcAddr == %s", ipStr, ipStr))
		} else {
			return "", fmt.Errorf("malformed IP address: %v", ip)
		}
	}

	var ipFilterParts []string
	if len(v4Clauses) > 0 {
		ipFilterParts = append(ipFilterParts, "("+strings.Join(v4Clauses, " or ")+")")
	}
	if len(v6Clauses) > 0 {
		ipFilterParts = append(ipFilterParts, "("+strings.Join(v6Clauses, " or ")+")")
	}

	if len(ipFilterParts) == 0 {
		return "", errors.New("no valid IPv4 or IPv6 address clauses generated")
	}

	portExpr := "(" + strings.Join(portClauses, " or ") + ")"
	ipExpr := "(" + strings.Join(ipFilterParts, " or ") + ")"

	finalFilter := fmt.Sprintf("%s and %s", portExpr, ipExpr)

	// Verification guard: never return match-all or empty filter
	if finalFilter == "" || finalFilter == "true" || strings.TrimSpace(finalFilter) == "()" {
		return "", errors.New("safety check failed: refusing to return unconstrained filter")
	}

	return finalFilter, nil
}
