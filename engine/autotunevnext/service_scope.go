package autotunevnext

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"time"

	"unbound/engine/observatory"
)

// MaxServiceScopeEdges bounds current DNS authority. Eight covers the observed
// YouTube A/AAAA snapshot without silently accepting a partial CDN result set.
const MaxServiceScopeEdges = 8

// MaxParallelEdgeProbes bounds each evidence phase. The global policy deadline
// remains authoritative; every probe also receives the policy's own timeout.
const MaxParallelEdgeProbes = MaxServiceScopeEdges

var (
	ErrServiceScopeEmpty    = errors.New("SERVICE_SCOPE_EMPTY")
	ErrServiceScopeTooLarge = errors.New("SERVICE_SCOPE_TOO_LARGE")
	ErrServiceScopeInvalid  = errors.New("SERVICE_SCOPE_INVALID")
)

// ServiceScopeEdge is a parsed, exact current DNS answer. It is execution
// evidence, never durable managed authority and never product-serialized.
type ServiceScopeEdge struct {
	IP     net.IP                    `json:"-"`
	Family observatory.AddressFamily `json:"-"`
}

// ServiceScopeSnapshot is a deterministic, bounded DNS snapshot for exactly
// one normalized target. Callers must resolve it again for every fresh run.
type ServiceScopeSnapshot struct {
	Target     Target
	ResolvedAt time.Time
	Edges      []ServiceScopeEdge
}

func (s ServiceScopeSnapshot) Fingerprint() string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(strings.ToLower(s.Target.URL)))
	for _, edge := range s.Edges {
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(edge.Family))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(edge.IP.String()))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (s ServiceScopeSnapshot) Contains(edge ServiceScopeEdge) bool {
	for _, candidate := range s.Edges {
		if candidate.Family == edge.Family && candidate.IP.Equal(edge.IP) {
			return true
		}
	}
	return false
}

// IsSubsetOf is intentionally one-way: disappearing DNS answers are harmless,
// but a new answer must never be admitted without a fresh experiment.
func (s ServiceScopeSnapshot) IsSubsetOf(validated ServiceScopeSnapshot) bool {
	if !sameTargetIdentity(s.Target, validated.Target) {
		return false
	}
	for _, edge := range s.Edges {
		if !validated.Contains(edge) {
			return false
		}
	}
	return true
}

func sameTargetIdentity(left, right Target) bool {
	return strings.EqualFold(left.URL, right.URL) && left.Transport == right.Transport && left.AddressFamily == right.AddressFamily
}

// ScopeResolver resolves only current A/AAAA answers. Implementations do not
// consult history or persisted state.
type ScopeResolver interface {
	ResolveServiceScope(context.Context, Target) (ServiceScopeSnapshot, error)
}

type DefaultScopeResolver struct{}

func (DefaultScopeResolver) ResolveServiceScope(ctx context.Context, target Target) (ServiceScopeSnapshot, error) {
	return ResolveServiceScope(ctx, target, net.DefaultResolver)
}

// ResolveServiceScope canonicalizes a fresh resolver result. DNS overflow is a
// failure, not a truncated scope: a partial result cannot prove service safety.
func ResolveServiceScope(ctx context.Context, target Target, resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}) (ServiceScopeSnapshot, error) {
	parsed, err := url.Parse(target.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return ServiceScopeSnapshot{}, fmt.Errorf("%w: normalized HTTPS target required", ErrServiceScopeInvalid)
	}
	hostname := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if hostname == "" || !strings.EqualFold(parsed.Hostname(), hostname) && strings.TrimSuffix(parsed.Hostname(), ".") != parsed.Hostname() {
		return ServiceScopeSnapshot{}, fmt.Errorf("%w: target hostname is not normalized", ErrServiceScopeInvalid)
	}
	addresses, err := resolver.LookupNetIP(ctx, "ip", hostname)
	if err != nil {
		return ServiceScopeSnapshot{}, fmt.Errorf("%w: %v", ErrServiceScopeEmpty, err)
	}
	edges := make([]ServiceScopeEdge, 0, len(addresses))
	seen := make(map[string]struct{}, len(addresses))
	for _, address := range addresses {
		if !address.IsValid() || address.IsUnspecified() || address.Is4In6() || address.Zone() != "" {
			return ServiceScopeSnapshot{}, fmt.Errorf("%w: unsupported DNS address", ErrServiceScopeInvalid)
		}
		ip := net.IP(address.AsSlice())
		family := familyForIP(ip)
		if family == "" || !scopeAcceptFamily(ip, target.AddressFamily) {
			if target.AddressFamily != "" && target.AddressFamily != observatory.AddressFamilyAny {
				continue
			}
			return ServiceScopeSnapshot{}, fmt.Errorf("%w: unsupported DNS address family", ErrServiceScopeInvalid)
		}
		key := string(family) + "|" + ip.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		edges = append(edges, ServiceScopeEdge{IP: append(net.IP(nil), ip...), Family: family})
	}
	if len(edges) == 0 {
		return ServiceScopeSnapshot{}, ErrServiceScopeEmpty
	}
	sort.Slice(edges, func(i, j int) bool {
		left, right := edges[i], edges[j]
		if left.Family != right.Family {
			return left.Family < right.Family
		}
		return netip.MustParseAddr(left.IP.String()).Less(netip.MustParseAddr(right.IP.String()))
	})
	if len(edges) > MaxServiceScopeEdges {
		return ServiceScopeSnapshot{}, fmt.Errorf("%w: %d > %d", ErrServiceScopeTooLarge, len(edges), MaxServiceScopeEdges)
	}
	canonical := target
	canonical.URL = "https://" + hostname
	if parsed.Port() != "" && parsed.Port() != "443" {
		canonical.URL += ":" + parsed.Port()
	}
	canonical.URL += parsed.EscapedPath()
	if canonical.URL == "https://"+hostname {
		canonical.URL += "/"
	}
	return ServiceScopeSnapshot{Target: canonical, ResolvedAt: time.Now().UTC(), Edges: edges}, nil
}

func scopeAcceptFamily(ip net.IP, requested observatory.AddressFamily) bool {
	switch requested {
	case "", observatory.AddressFamilyAny:
		return true
	case observatory.AddressFamilyIPv4:
		return ip.To4() != nil
	case observatory.AddressFamilyIPv6:
		return ip.To4() == nil && ip.To16() != nil
	default:
		return false
	}
}

type ServiceScopeStatus string

const (
	ServiceScopePending          ServiceScopeStatus = "PENDING"
	ServiceScopeVerifiedFixed    ServiceScopeStatus = "SERVICE_SCOPE_VERIFIED_FIXED"
	ServiceScopeNoActionNeeded   ServiceScopeStatus = "NO_ACTION_NEEDED"
	ServiceScopeStillFailing     ServiceScopeStatus = "STILL_FAILING"
	ServiceScopeTargetRegression ServiceScopeStatus = "TARGET_REGRESSION"
	ServiceScopeInconclusive     ServiceScopeStatus = "INCONCLUSIVE"
	ServiceScopeChanged          ServiceScopeStatus = "SERVICE_SCOPE_CHANGED_REVALIDATION_REQUIRED"
)

type EdgeEvidenceState string

const (
	EdgeFixedByProfile      EdgeEvidenceState = "FIXED_BY_PROFILE"
	EdgeUnaffectedReachable EdgeEvidenceState = "UNAFFECTED_REACHABLE"
	EdgeStillFailing        EdgeEvidenceState = "STILL_FAILING"
	EdgeTargetRegression    EdgeEvidenceState = "TARGET_REGRESSION"
	EdgeInconclusive        EdgeEvidenceState = "INCONCLUSIVE"
)

// ServiceScopeValidation exposes only counts and aggregate state to products.
// Per-edge observations remain evidence records and are intentionally redacted.
type ServiceScopeValidation struct {
	EdgeCount    int                `json:"edge_count"`
	Status       ServiceScopeStatus `json:"status"`
	Fixed        int                `json:"fixed"`
	Reachable    int                `json:"reachable"`
	Failed       int                `json:"failed"`
	Inconclusive int                `json:"inconclusive"`
}

type EdgeEvidence struct {
	Edge         ServiceScopeEdge
	DirectBefore observatory.ObservationResult
	Active       observatory.ObservationResult
	DirectAfter  observatory.ObservationResult
	State        EdgeEvidenceState
}

func aggregateServiceScope(edges []EdgeEvidence) ServiceScopeValidation {
	result := ServiceScopeValidation{EdgeCount: len(edges), Status: ServiceScopeInconclusive}
	if len(edges) == 0 {
		return result
	}
	for _, edge := range edges {
		switch edge.State {
		case EdgeFixedByProfile:
			result.Fixed++
		case EdgeUnaffectedReachable:
			result.Reachable++
		case EdgeStillFailing:
			result.Failed++
		case EdgeTargetRegression:
			result.Failed++
			result.Status = ServiceScopeTargetRegression
			return result
		default:
			result.Inconclusive++
		}
	}
	if result.Failed > 0 {
		result.Status = ServiceScopeStillFailing
	} else if result.Inconclusive > 0 || result.Fixed == 0 {
		result.Status = ServiceScopeInconclusive
	} else {
		result.Status = ServiceScopeVerifiedFixed
	}
	return result
}
