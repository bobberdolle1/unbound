package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// StrategyAggressiveness defines the risk and blast radius of a desync strategy.
type StrategyAggressiveness int

const (
	AggressivenessLow          StrategyAggressiveness = 1 // Standard splitting (e.g. hostfakesplit, multisplit)
	AggressivenessMedium       StrategyAggressiveness = 2 // Fake packets with safe payloads or wssize
	AggressivenessHigh         StrategyAggressiveness = 3 // Multi-disorder, complex fragmentation, heavy fooling
	AggressivenessExperimental StrategyAggressiveness = 4 // Out-of-spec syndata, raw sequence overlaps
)

func (a StrategyAggressiveness) String() string {
	switch a {
	case AggressivenessLow:
		return "LOW"
	case AggressivenessMedium:
		return "MEDIUM"
	case AggressivenessHigh:
		return "HIGH"
	case AggressivenessExperimental:
		return "EXPERIMENTAL"
	default:
		return "UNKNOWN"
	}
}

// TCPTimestampsRequirement specifies TCP timestamps prerequisites.
type TCPTimestampsRequirement string

const (
	TimestampsNone        TCPTimestampsRequirement = "none"
	TimestampsRecommended TCPTimestampsRequirement = "recommended"
	TimestampsRequired    TCPTimestampsRequirement = "required"
)

// StrategyRequirements encapsulates OS and engine prerequisites for a strategy.
type StrategyRequirements struct {
	TCPTimestamps    TCPTimestampsRequirement `json:"tcpTimestamps"`
	InboundTCP       bool                     `json:"inboundTcp"`
	InboundUDP       bool                     `json:"inboundUdp"`
	QUIC             bool                     `json:"quic"`
	IPv6             bool                     `json:"ipv6"`
	LuaModules       []string                 `json:"luaModules,omitempty"`
	EngineMinVersion string                   `json:"engineMinVersion,omitempty"`
	FakePayloads     []string                 `json:"fakePayloads,omitempty"`
}

// StrategyCandidate represents an experimental or catalog bypass candidate evaluated in Strategy Lab.
type StrategyCandidate struct {
	ID                   string                 `json:"id"`
	Name                 string                 `json:"name"`
	Protocol             string                 `json:"protocol"` // "HTTP", "TLS1.2", "TLS1.3", "QUIC", "ANY"
	Zapret2Args          []string               `json:"zapret2Args"`
	RequiredLuaFunctions []string               `json:"requiredLuaFunctions,omitempty"`
	Requirements         StrategyRequirements   `json:"requirements"`
	Aggressiveness       StrategyAggressiveness `json:"aggressiveness"`
	Source               string                 `json:"source"` // "blockcheck2 v1.0.5", "unbound", "custom"
	Experimental         bool                   `json:"experimental"`
	ExpectedTrafficScope string                 `json:"expectedTrafficScope"` // "target_only", "global"
}

// SummaryDescription returns a human-readable summary of the candidate strategy.
func (sc StrategyCandidate) SummaryDescription() string {
	return fmt.Sprintf("%s [%s] (Aggressiveness: %s, Source: %s)",
		sc.Name, sc.Protocol, sc.Aggressiveness.String(), sc.Source)
}

// ValidProtocols lists all officially recognized protocols for Strategy Lab.
var ValidProtocols = map[string]bool{
	"HTTP":   true,
	"TLS1.2": true,
	"TLS1.3": true,
	"QUIC":   true,
	"ANY":    true,
}

// IsValidProtocol checks whether a protocol identifier is recognized and supported.
func IsValidProtocol(protocol string) bool {
	return ValidProtocols[strings.ToUpper(strings.TrimSpace(protocol))]
}

// CheckCandidateCapabilities evaluates whether system environment supports candidate requirements.
func CheckCandidateCapabilities(cand StrategyCandidate, assets *AssetPaths) (bool, string) {
	req := cand.Requirements

	// 1. TCP Timestamps requirement
	if req.TCPTimestamps == TimestampsRequired {
		if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
			return false, "SKIPPED_UNSUPPORTED: TCP timestamps required but not supported on platform"
		}
	}

	// 2. QUIC requirements
	if req.QUIC {
		if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
			return false, "SKIPPED_UNSUPPORTED: QUIC interception requires Windows WinDivert or Linux NFQUEUE"
		}
	}

	// 3. Fake payload files
	if len(req.FakePayloads) > 0 && assets != nil {
		for _, fp := range req.FakePayloads {
			target := filepath.Join(assets.ListDir, fp)
			if _, err := os.Stat(target); os.IsNotExist(err) {
				return false, fmt.Sprintf("SKIPPED_CONFIG_ERROR: missing required fake payload %s", fp)
			}
		}
	}

	// 4. Lua modules
	if len(req.LuaModules) > 0 && assets != nil {
		for _, mod := range req.LuaModules {
			target := filepath.Join(assets.LuaDir, mod)
			if _, err := os.Stat(target); os.IsNotExist(err) {
				return false, fmt.Sprintf("SKIPPED_CONFIG_ERROR: missing required Lua module %s", mod)
			}
		}
	}

	return true, ""
}

// GetBlockCheck2Candidates returns the canonical suite of BlockCheck2-inspired candidates
// safe for isolated testing via WinDivert raw filter.
// Rejects invalid or unknown protocols with fail-closed nil return.
func GetBlockCheck2Candidates(protocol string) []StrategyCandidate {
	proto := strings.ToUpper(strings.TrimSpace(protocol))
	if proto == "" {
		proto = "TLS1.3"
	}
	if !IsValidProtocol(proto) {
		return nil
	}
	all := []StrategyCandidate{
		// 1. HostFakeSplit (Low Aggressiveness - Canonical baseline)
		{
			ID:       "cand_hostfakesplit_midhost",
			Name:     "HostFakeSplit (midsld)",
			Protocol: "TLS1.3",
			Zapret2Args: []string{
				"--payload=tls_client_hello",
				"--lua-desync=hostfakesplit:midhost=midsld:repeats=2",
			},
			Requirements:   StrategyRequirements{TCPTimestamps: TimestampsNone},
			Aggressiveness: AggressivenessLow,
			Source:         "blockcheck2 v1.0.5 (35-hostfake.sh)",
		},
		{
			ID:       "cand_hostfakesplit_default",
			Name:     "HostFakeSplit (default)",
			Protocol: "TLS1.3",
			Zapret2Args: []string{
				"--payload=tls_client_hello",
				"--lua-desync=hostfakesplit:repeats=2",
			},
			Requirements:   StrategyRequirements{TCPTimestamps: TimestampsNone},
			Aggressiveness: AggressivenessLow,
			Source:         "blockcheck2 v1.0.5 (35-hostfake.sh)",
		},

		// 2. MultiSplit (Low Aggressiveness)
		{
			ID:       "cand_multisplit_midsld",
			Name:     "MultiSplit (midsld)",
			Protocol: "TLS1.3",
			Zapret2Args: []string{
				"--payload=tls_client_hello",
				"--lua-desync=multisplit:pos=midsld",
			},
			Requirements:   StrategyRequirements{TCPTimestamps: TimestampsNone},
			Aggressiveness: AggressivenessLow,
			Source:         "blockcheck2 v1.0.5 (20-multi.sh)",
		},
		{
			ID:       "cand_multisplit_sniext",
			Name:     "MultiSplit (sniext+1)",
			Protocol: "TLS1.3",
			Zapret2Args: []string{
				"--payload=tls_client_hello",
				"--lua-desync=multisplit:pos=sniext+1",
			},
			Requirements:   StrategyRequirements{TCPTimestamps: TimestampsNone},
			Aggressiveness: AggressivenessLow,
			Source:         "blockcheck2 v1.0.5 (20-multi.sh)",
		},
		{
			ID:       "cand_multisplit_dual",
			Name:     "MultiSplit (1,midsld)",
			Protocol: "TLS1.3",
			Zapret2Args: []string{
				"--payload=tls_client_hello",
				"--lua-desync=multisplit:pos=1,midsld",
			},
			Requirements:   StrategyRequirements{TCPTimestamps: TimestampsNone},
			Aggressiveness: AggressivenessMedium,
			Source:         "blockcheck2 v1.0.5 (20-multi.sh)",
		},

		// 3. WSSize + MultiSplit (Medium Aggressiveness - window scaling)
		{
			ID:       "cand_wssize_multisplit",
			Name:     "WSSize + MultiSplit (wsize=1, sniext+1)",
			Protocol: "TLS1.3",
			Zapret2Args: []string{
				"--payload=tls_client_hello",
				"--lua-desync=wssize:wsize=1:scale=6",
				"--lua-desync=multisplit:pos=sniext+1",
			},
			Requirements: StrategyRequirements{
				TCPTimestamps: TimestampsRecommended,
			},
			Aggressiveness: AggressivenessMedium,
			Source:         "blockcheck2 v1.0.5 (20-multi.sh)",
		},

		// 4. Fake TLS (Medium Aggressiveness)
		{
			ID:       "cand_fake_tls_default",
			Name:     "Fake TLS (tls_client_hello)",
			Protocol: "TLS1.3",
			Zapret2Args: []string{
				"--payload=tls_client_hello",
				"--lua-desync=fake:repeats=2",
			},
			Requirements:   StrategyRequirements{TCPTimestamps: TimestampsNone},
			Aggressiveness: AggressivenessMedium,
			Source:         "blockcheck2 v1.0.5 (25-fake.sh)",
		},
		{
			ID:       "cand_fake_tls_multisplit",
			Name:     "Fake TLS + MultiSplit (sniext+1)",
			Protocol: "TLS1.3",
			Zapret2Args: []string{
				"--payload=tls_client_hello",
				"--lua-desync=fake:repeats=2",
				"--lua-desync=multisplit:pos=sniext+1",
			},
			Requirements:   StrategyRequirements{TCPTimestamps: TimestampsNone},
			Aggressiveness: AggressivenessHigh,
			Source:         "blockcheck2 v1.0.5 (50-fake-multi.sh)",
		},

		// 5. MultiDisorder (High Aggressiveness)
		{
			ID:       "cand_multidisorder_midsld",
			Name:     "MultiDisorder (midsld)",
			Protocol: "TLS1.3",
			Zapret2Args: []string{
				"--payload=tls_client_hello",
				"--lua-desync=multidisorder:pos=midsld",
			},
			Requirements:   StrategyRequirements{TCPTimestamps: TimestampsNone},
			Aggressiveness: AggressivenessHigh,
			Source:         "blockcheck2 v1.0.5 (20-multi.sh)",
		},

		// 6. FakeDSplit (High Aggressiveness)
		{
			ID:       "cand_fakedsplit_default",
			Name:     "FakeDSplit (disorder)",
			Protocol: "TLS1.3",
			Zapret2Args: []string{
				"--payload=tls_client_hello",
				"--lua-desync=fakedsplit:repeats=2",
			},
			Requirements:   StrategyRequirements{TCPTimestamps: TimestampsNone},
			Aggressiveness: AggressivenessHigh,
			Source:         "blockcheck2 v1.0.5 (30-faked.sh)",
		},

		// 7. SynData (Experimental)
		{
			ID:       "cand_syndata_experimental",
			Name:     "SynData (TCP SYN payload)",
			Protocol: "TLS1.3",
			Zapret2Args: []string{
				"--payload=tls_client_hello",
				"--lua-desync=syndata",
			},
			Requirements: StrategyRequirements{
				TCPTimestamps: TimestampsRequired,
			},
			Aggressiveness: AggressivenessExperimental,
			Source:         "blockcheck2 v1.0.5 (24-syndata.sh)",
			Experimental:   true,
		},

		// 8. HTTP Candidates
		{
			ID:       "cand_http_multisplit_midsld",
			Name:     "HTTP MultiSplit (midsld)",
			Protocol: "HTTP",
			Zapret2Args: []string{
				"--payload=http_req",
				"--lua-desync=multisplit:pos=midsld",
			},
			Requirements:   StrategyRequirements{TCPTimestamps: TimestampsNone},
			Aggressiveness: AggressivenessLow,
			Source:         "blockcheck2 v1.0.5 (10-http-basic.sh)",
		},
		{
			ID:       "cand_http_hostfakesplit",
			Name:     "HTTP HostFakeSplit",
			Protocol: "HTTP",
			Zapret2Args: []string{
				"--payload=http_req",
				"--lua-desync=hostfakesplit",
			},
			Requirements:   StrategyRequirements{TCPTimestamps: TimestampsNone},
			Aggressiveness: AggressivenessLow,
			Source:         "blockcheck2 v1.0.5 (35-hostfake.sh)",
		},

		// 9. QUIC Candidate
		{
			ID:       "cand_quic_fake",
			Name:     "QUIC Fake Packet",
			Protocol: "QUIC",
			Zapret2Args: []string{
				"--payload=quic_initial",
				"--lua-desync=fake:repeats=2",
			},
			Requirements: StrategyRequirements{
				QUIC: true,
			},
			Aggressiveness: AggressivenessMedium,
			Source:         "blockcheck2 v1.0.5 (90-quic.sh)",
		},
	}

	var matched []StrategyCandidate
	for _, c := range all {
		if proto == "ANY" || c.Protocol == proto || (strings.HasPrefix(proto, "TLS") && strings.HasPrefix(c.Protocol, "TLS")) {
			matched = append(matched, c)
		}
	}
	return matched
}
