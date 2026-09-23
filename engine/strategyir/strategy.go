// Package strategyir defines the safe, backend-neutral StrategyIR v1.
package strategyir

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

const SchemaVersion = 1

type Transport string

const (
	TransportTCP  Transport = "TCP"
	TransportUDP  Transport = "UDP"
	TransportQUIC Transport = "QUIC"
)

type ApplicationProtocol string

const (
	ApplicationAny  ApplicationProtocol = "ANY"
	ApplicationHTTP ApplicationProtocol = "HTTP"
	ApplicationTLS  ApplicationProtocol = "TLS"
	ApplicationQUIC ApplicationProtocol = "QUIC"
)

type IPFamily string

const (
	IPFamilyAny IPFamily = "ANY"
	IPFamilyV4  IPFamily = "IPv4"
	IPFamilyV6  IPFamily = "IPv6"
)

type Direction string

const (
	DirectionOutbound Direction = "OUTBOUND"
	DirectionInbound  Direction = "INBOUND"
	DirectionBoth     Direction = "BOTH"
)

type HostScopeMode string

const (
	HostScopeAll            HostScopeMode = "ALL"
	HostScopeExplicit       HostScopeMode = "EXPLICIT"
	HostScopeManagedList    HostScopeMode = "MANAGED_LIST"
	HostScopeAutoHostlist   HostScopeMode = "AUTO_HOSTLIST"
	HostScopeIPSetReference HostScopeMode = "IP_SET_REFERENCE"
)

// PortRange is inclusive. A single port has Start == End.
type PortRange struct {
	Start uint16 `json:"start"`
	End   uint16 `json:"end"`
}

// HostScope intentionally accepts only logical product IDs, never filesystem paths.
type HostScope struct {
	Mode  HostScopeMode `json:"mode"`
	Hosts []string      `json:"hosts,omitempty"`
	ID    string        `json:"id,omitempty"`
}

// Scope selects destinations independently from packet operations.
type Scope struct {
	Host               HostScope `json:"host"`
	ExcludeHostListIDs []string  `json:"exclude_host_list_ids,omitempty"`
	IPSetIDs           []string  `json:"ip_set_ids,omitempty"`
	ExcludeIPSetIDs    []string  `json:"exclude_ip_set_ids,omitempty"`
}

type TrafficSelector struct {
	ApplicationProtocols []ApplicationProtocol `json:"application_protocols"`
	IPFamilies           []IPFamily            `json:"ip_families"`
	Direction            Direction             `json:"direction"`
	TCPPorts             []PortRange           `json:"tcp_ports,omitempty"`
	UDPPorts             []PortRange           `json:"udp_ports,omitempty"`
	Scope                Scope                 `json:"scope"`
}

type PositionAnchor string

const (
	AnchorHost    PositionAnchor = "HOST"
	AnchorEndHost PositionAnchor = "ENDHOST"
	AnchorMidSLD  PositionAnchor = "MIDSLD"
	AnchorSNIExt  PositionAnchor = "SNIEXT"
	AnchorMethod  PositionAnchor = "METHOD"
)

// PositionExpr is either an absolute payload byte or a typed anchor plus offset.
type PositionExpr struct {
	Absolute *int           `json:"absolute,omitempty"`
	Anchor   PositionAnchor `json:"anchor,omitempty"`
	Offset   int            `json:"offset,omitempty"`
}

type OperationKind string

const (
	OperationSplit          OperationKind = "SPLIT"
	OperationMultiSplit     OperationKind = "MULTI_SPLIT"
	OperationDisorder       OperationKind = "DISORDER"
	OperationMultiDisorder  OperationKind = "MULTI_DISORDER"
	OperationTLSRecordSplit OperationKind = "TLS_RECORD_SPLIT"
	OperationHTTPHostCase   OperationKind = "HTTP_HOST_CASE"
	OperationFakeInjection  OperationKind = "FAKE_INJECTION"
	OperationHostFakeSplit  OperationKind = "HOST_FAKE_SPLIT"
	OperationWindowShaping  OperationKind = "WINDOW_SHAPING"
)

type FakeModifiers struct {
	Repeat               int  `json:"repeat,omitempty"`
	TTL                  *int `json:"ttl,omitempty"`
	SequenceOffset       *int `json:"sequence_offset,omitempty"`
	AcknowledgmentOffset *int `json:"acknowledgment_offset,omitempty"`
	TCPMD5               bool `json:"tcp_md5,omitempty"`
	TCPTimestamp         bool `json:"tcp_timestamp,omitempty"`
}

// RangeDirection selects the traffic direction whose range counter is bounded.
type RangeDirection string

const (
	RangeDirectionIn  RangeDirection = "IN"
	RangeDirectionOut RangeDirection = "OUT"
)

// RangeCounter names the upstream range counter without exposing its expression syntax.
type RangeCounter string

const (
	RangeCounterPacketNumber     RangeCounter = "PACKET_NUMBER"
	RangeCounterDataPacketNumber RangeCounter = "DATA_PACKET_NUMBER"
	RangeCounterRelativeSequence RangeCounter = "RELATIVE_SEQUENCE"
	RangeCounterDataPosition     RangeCounter = "DATA_POSITION"
)

// Cutoff bounds one typed upstream range counter. Limit is the inclusive terminal
// counter value; Zapret2 currently spells DATA_PACKET_NUMBER as -d<limit> and
// RELATIVE_SEQUENCE as -s<limit>.
type Cutoff struct {
	Direction RangeDirection `json:"direction"`
	Counter   RangeCounter   `json:"counter"`
	Limit     int            `json:"limit"`
}

type WindowShaping struct {
	Window int `json:"window"`
	Scale  int `json:"scale,omitempty"`
}

// Operation contains no backend-language escape hatch. PayloadRef is a trusted logical asset ID.
type Operation struct {
	Type              OperationKind  `json:"type"`
	Positions         []PositionExpr `json:"positions,omitempty"`
	PayloadRef        string         `json:"payload_ref,omitempty"`
	SequenceOverlap   *int           `json:"sequence_overlap,omitempty"`
	OverlapPatternRef string         `json:"overlap_pattern_ref,omitempty"`
	Fake              *FakeModifiers `json:"fake,omitempty"`
	HostTemplate      string         `json:"host_template,omitempty"`
	Window            *WindowShaping `json:"window,omitempty"`
	Cutoff            *Cutoff        `json:"cutoff,omitempty"`
}

type SafetyPolicy struct {
	Aggressiveness string `json:"aggressiveness"`
	TargetOnly     bool   `json:"target_only"`
	MayAffectSteam bool   `json:"may_affect_steam,omitempty"`
	MayAffectTLS   bool   `json:"may_affect_non_target_tls,omitempty"`
	Experimental   bool   `json:"experimental,omitempty"`
}

type Metadata struct {
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// Strategy is declarative and executable only after compilation by a trusted backend.
type Strategy struct {
	SchemaVersion int             `json:"schema_version"`
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Transport     []Transport     `json:"transport"`
	Selector      TrafficSelector `json:"selector"`
	Operations    []Operation     `json:"operations"`
	Safety        SafetyPolicy    `json:"safety"`
	Metadata      Metadata        `json:"metadata,omitempty"`
}

var (
	logicalID   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
	hostLiteral = regexp.MustCompile(`(?i)^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
)

func Validate(s Strategy) error {
	if s.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version must be %d", SchemaVersion)
	}
	if !validateID(s.ID) {
		return fmt.Errorf("id must be a logical identifier")
	}
	if len(s.Transport) == 0 {
		return fmt.Errorf("transport is required")
	}
	if len(s.Operations) == 0 {
		return fmt.Errorf("operations is required")
	}
	if len(s.Selector.ApplicationProtocols) == 0 {
		return fmt.Errorf("application_protocols is required")
	}
	if len(s.Selector.IPFamilies) == 0 {
		return fmt.Errorf("ip_families is required")
	}
	if s.Selector.Direction != DirectionOutbound && s.Selector.Direction != DirectionInbound && s.Selector.Direction != DirectionBoth {
		return fmt.Errorf("invalid direction %q", s.Selector.Direction)
	}
	for _, transport := range s.Transport {
		if transport != TransportTCP && transport != TransportUDP && transport != TransportQUIC {
			return fmt.Errorf("invalid transport %q", transport)
		}
	}
	if containsTransport(s.Transport, TransportTCP) && len(s.Selector.TCPPorts) == 0 {
		return fmt.Errorf("tcp transport requires tcp_ports")
	}
	if (containsTransport(s.Transport, TransportUDP) || containsTransport(s.Transport, TransportQUIC)) && len(s.Selector.UDPPorts) == 0 {
		return fmt.Errorf("udp or quic transport requires udp_ports")
	}
	for _, protocol := range s.Selector.ApplicationProtocols {
		if protocol != ApplicationAny && protocol != ApplicationHTTP && protocol != ApplicationTLS && protocol != ApplicationQUIC {
			return fmt.Errorf("invalid application protocol %q", protocol)
		}
	}
	if containsProtocol(s.Selector.ApplicationProtocols, ApplicationAny) && len(s.Selector.ApplicationProtocols) != 1 {
		return fmt.Errorf("application ANY cannot be combined with other protocols")
	}
	for _, family := range s.Selector.IPFamilies {
		if family != IPFamilyAny && family != IPFamilyV4 && family != IPFamilyV6 {
			return fmt.Errorf("invalid IP family %q", family)
		}
	}
	if len(s.Selector.TCPPorts) != 0 && !containsTransport(s.Transport, TransportTCP) {
		return fmt.Errorf("tcp_ports require TCP transport")
	}
	if len(s.Selector.UDPPorts) != 0 && !containsTransport(s.Transport, TransportUDP) && !containsTransport(s.Transport, TransportQUIC) {
		return fmt.Errorf("udp_ports require UDP or QUIC transport")
	}
	if err := validatePorts(s.Selector.TCPPorts); err != nil {
		return fmt.Errorf("tcp_ports: %w", err)
	}
	if err := validatePorts(s.Selector.UDPPorts); err != nil {
		return fmt.Errorf("udp_ports: %w", err)
	}
	if err := validateScope(s.Selector.Scope); err != nil {
		return err
	}
	if s.Safety.TargetOnly && s.Selector.Scope.Host.Mode == HostScopeAll {
		return fmt.Errorf("target_only safety cannot use all host scope")
	}
	switch s.Safety.Aggressiveness {
	case "LOW", "MEDIUM", "HIGH", "EXPERIMENTAL":
	default:
		return fmt.Errorf("invalid safety aggressiveness %q", s.Safety.Aggressiveness)
	}
	for i, operation := range s.Operations {
		if err := validateOperation(operation); err != nil {
			return fmt.Errorf("operations[%d]: %w", i, err)
		}
	}
	return nil
}

func validatePorts(ports []PortRange) error {
	for _, p := range ports {
		if p.Start == 0 || p.End == 0 || p.Start > p.End {
			return fmt.Errorf("invalid range %d-%d", p.Start, p.End)
		}
	}
	return nil
}
func validateID(id string) bool { return logicalID.MatchString(id) }
func validateScope(scope Scope) error {
	h := scope.Host
	switch h.Mode {
	case HostScopeAll:
		if len(h.Hosts) != 0 || h.ID != "" {
			return fmt.Errorf("all host scope cannot carry hosts or id")
		}
	case HostScopeExplicit:
		if len(h.Hosts) == 0 || h.ID != "" {
			return fmt.Errorf("explicit host scope requires hosts only")
		}
		for _, host := range h.Hosts {
			if !hostLiteral.MatchString(host) {
				return fmt.Errorf("invalid host literal %q", host)
			}
		}
	case HostScopeManagedList, HostScopeAutoHostlist, HostScopeIPSetReference:
		if !validateID(h.ID) || len(h.Hosts) != 0 {
			return fmt.Errorf("managed scope requires logical id only")
		}
	default:
		return fmt.Errorf("invalid host scope mode %q", h.Mode)
	}
	for _, id := range append(append([]string{}, scope.ExcludeHostListIDs...), append(scope.IPSetIDs, scope.ExcludeIPSetIDs...)...) {
		if !validateID(id) {
			return fmt.Errorf("invalid logical asset id %q", id)
		}
	}
	return nil
}
func validateOperation(op Operation) error {
	switch op.Type {
	case OperationSplit, OperationMultiSplit, OperationDisorder, OperationMultiDisorder, OperationTLSRecordSplit, OperationHTTPHostCase, OperationFakeInjection, OperationHostFakeSplit, OperationWindowShaping:
	default:
		return fmt.Errorf("invalid operation %q", op.Type)
	}
	for _, p := range op.Positions {
		if (p.Absolute == nil) == (p.Anchor == "") {
			return fmt.Errorf("position must have exactly one of absolute or anchor")
		}
		if p.Absolute != nil {
			if *p.Absolute < 0 {
				return fmt.Errorf("absolute position must be non-negative")
			}
			if p.Offset != 0 {
				return fmt.Errorf("absolute position cannot carry offset")
			}
		}
		if p.Anchor != "" && p.Anchor != AnchorHost && p.Anchor != AnchorEndHost && p.Anchor != AnchorMidSLD && p.Anchor != AnchorSNIExt && p.Anchor != AnchorMethod {
			return fmt.Errorf("invalid position anchor %q", p.Anchor)
		}
	}
	if op.PayloadRef != "" && !validateID(op.PayloadRef) {
		return fmt.Errorf("invalid payload_ref")
	}
	if op.SequenceOverlap != nil && *op.SequenceOverlap < 1 {
		return fmt.Errorf("sequence_overlap must be positive")
	}
	if op.OverlapPatternRef != "" && !validateID(op.OverlapPatternRef) {
		return fmt.Errorf("invalid overlap_pattern_ref")
	}
	if op.HostTemplate != "" && !hostLiteral.MatchString(op.HostTemplate) {
		return fmt.Errorf("invalid host_template")
	}
	if op.Fake != nil {
		if op.Fake.Repeat < 0 {
			return fmt.Errorf("repeat must be non-negative")
		}
		if op.Fake.TTL != nil && (*op.Fake.TTL < 1 || *op.Fake.TTL > 255) {
			return fmt.Errorf("ttl must be 1..255")
		}
		if op.Fake.Repeat == 0 && op.Fake.TTL == nil && op.Fake.SequenceOffset == nil && op.Fake.AcknowledgmentOffset == nil && !op.Fake.TCPMD5 && !op.Fake.TCPTimestamp {
			return fmt.Errorf("fake modifiers cannot be empty")
		}
	}
	if op.Window != nil && (op.Window.Window < 1 || op.Window.Scale < 0) {
		return fmt.Errorf("window values must be positive")
	}
	if op.Cutoff != nil {
		if op.Cutoff.Direction != RangeDirectionIn && op.Cutoff.Direction != RangeDirectionOut {
			return fmt.Errorf("invalid cutoff direction %q", op.Cutoff.Direction)
		}
		switch op.Cutoff.Counter {
		case RangeCounterPacketNumber, RangeCounterDataPacketNumber, RangeCounterRelativeSequence, RangeCounterDataPosition:
		default:
			return fmt.Errorf("invalid cutoff counter %q", op.Cutoff.Counter)
		}
		if op.Cutoff.Limit < 1 || op.Cutoff.Limit > 1<<31-1 {
			return fmt.Errorf("cutoff limit must be 1..2147483647")
		}
	}
	requirePositions := func(exactlyOne bool) error {
		if len(op.Positions) == 0 || (exactlyOne && len(op.Positions) != 1) {
			return fmt.Errorf("%s requires %spositions", op.Type, map[bool]string{true: "exactly one ", false: ""}[exactlyOne])
		}
		return nil
	}
	forbid := func(field, value string, present bool) error {
		if present {
			return fmt.Errorf("%s cannot carry %s", op.Type, field)
		}
		return nil
	}
	switch op.Type {
	case OperationSplit:
		if err := requirePositions(true); err != nil {
			return err
		}
	case OperationMultiSplit:
		if err := requirePositions(false); err != nil {
			return err
		}
		if op.OverlapPatternRef != "" && op.SequenceOverlap == nil {
			return fmt.Errorf("%s overlap_pattern_ref requires sequence_overlap", op.Type)
		}
	case OperationMultiDisorder:
		if err := requirePositions(false); err != nil {
			return err
		}
	case OperationTLSRecordSplit:
		if err := requirePositions(false); err != nil {
			return err
		}
	case OperationFakeInjection:
		if !validateID(op.PayloadRef) {
			return fmt.Errorf("fake injection requires logical payload_ref")
		}
	case OperationHostFakeSplit:
		if err := requirePositions(false); err != nil {
			return err
		}
	case OperationWindowShaping:
		if op.Window == nil {
			return fmt.Errorf("window shaping requires window")
		}
	}
	allowsPositions := op.Type == OperationSplit || op.Type == OperationMultiSplit || op.Type == OperationMultiDisorder || op.Type == OperationTLSRecordSplit || op.Type == OperationHostFakeSplit
	allowsPayload := op.Type == OperationFakeInjection
	allowsOverlap := op.Type == OperationMultiSplit
	allowsFake := op.Type == OperationFakeInjection || op.Type == OperationMultiDisorder || op.Type == OperationHostFakeSplit
	allowsHostTemplate := op.Type == OperationHostFakeSplit
	allowsWindow := op.Type == OperationWindowShaping
	if err := forbid("positions", "", len(op.Positions) != 0 && !allowsPositions); err != nil {
		return err
	}
	if err := forbid("payload_ref", op.PayloadRef, op.PayloadRef != "" && !allowsPayload); err != nil {
		return err
	}
	if err := forbid("sequence_overlap", "", op.SequenceOverlap != nil && !allowsOverlap); err != nil {
		return err
	}
	if err := forbid("overlap_pattern_ref", op.OverlapPatternRef, op.OverlapPatternRef != "" && !allowsOverlap); err != nil {
		return err
	}
	if err := forbid("fake", "", op.Fake != nil && !allowsFake); err != nil {
		return err
	}
	if err := forbid("host_template", op.HostTemplate, op.HostTemplate != "" && !allowsHostTemplate); err != nil {
		return err
	}
	if err := forbid("window", "", op.Window != nil && !allowsWindow); err != nil {
		return err
	}
	return nil
}

func containsTransport(transports []Transport, expected Transport) bool {
	for _, transport := range transports {
		if transport == expected {
			return true
		}
	}
	return false
}

func containsProtocol(protocols []ApplicationProtocol, expected ApplicationProtocol) bool {
	for _, protocol := range protocols {
		if protocol == expected {
			return true
		}
	}
	return false
}

// Canonicalize normalizes semantic sets without changing operation or position order.
func Canonicalize(s Strategy) (Strategy, error) {
	if err := Validate(s); err != nil {
		return Strategy{}, err
	}
	out := cloneStrategy(s)
	out.Transport = sortedUnique(out.Transport)
	out.Selector.ApplicationProtocols = sortedUnique(out.Selector.ApplicationProtocols)
	out.Selector.IPFamilies = sortedUnique(out.Selector.IPFamilies)
	out.Selector.TCPPorts = canonicalPorts(out.Selector.TCPPorts)
	out.Selector.UDPPorts = canonicalPorts(out.Selector.UDPPorts)
	out.Selector.Scope.Host.Hosts = canonicalHosts(out.Selector.Scope.Host.Hosts)
	out.Selector.Scope.ExcludeHostListIDs = sortedStrings(out.Selector.Scope.ExcludeHostListIDs)
	out.Selector.Scope.IPSetIDs = sortedStrings(out.Selector.Scope.IPSetIDs)
	out.Selector.Scope.ExcludeIPSetIDs = sortedStrings(out.Selector.Scope.ExcludeIPSetIDs)
	for index := range out.Operations {
		out.Operations[index].HostTemplate = strings.ToLower(out.Operations[index].HostTemplate)
	}
	return out, nil
}

func cloneStrategy(s Strategy) Strategy {
	out := s
	out.Transport = append([]Transport(nil), s.Transport...)
	out.Selector.ApplicationProtocols = append([]ApplicationProtocol(nil), s.Selector.ApplicationProtocols...)
	out.Selector.IPFamilies = append([]IPFamily(nil), s.Selector.IPFamilies...)
	out.Selector.TCPPorts = append([]PortRange(nil), s.Selector.TCPPorts...)
	out.Selector.UDPPorts = append([]PortRange(nil), s.Selector.UDPPorts...)
	out.Selector.Scope.Host.Hosts = append([]string(nil), s.Selector.Scope.Host.Hosts...)
	out.Selector.Scope.ExcludeHostListIDs = append([]string(nil), s.Selector.Scope.ExcludeHostListIDs...)
	out.Selector.Scope.IPSetIDs = append([]string(nil), s.Selector.Scope.IPSetIDs...)
	out.Selector.Scope.ExcludeIPSetIDs = append([]string(nil), s.Selector.Scope.ExcludeIPSetIDs...)
	out.Operations = make([]Operation, len(s.Operations))
	for i, op := range s.Operations {
		out.Operations[i] = op
		out.Operations[i].Positions = append([]PositionExpr(nil), op.Positions...)
		for j, position := range op.Positions {
			if position.Absolute != nil {
				value := *position.Absolute
				out.Operations[i].Positions[j].Absolute = &value
			}
		}
		if op.SequenceOverlap != nil {
			value := *op.SequenceOverlap
			out.Operations[i].SequenceOverlap = &value
		}
		if op.Fake != nil {
			fake := *op.Fake
			if op.Fake.TTL != nil {
				value := *op.Fake.TTL
				fake.TTL = &value
			}
			if op.Fake.SequenceOffset != nil {
				value := *op.Fake.SequenceOffset
				fake.SequenceOffset = &value
			}
			if op.Fake.AcknowledgmentOffset != nil {
				value := *op.Fake.AcknowledgmentOffset
				fake.AcknowledgmentOffset = &value
			}
			out.Operations[i].Fake = &fake
		}
		if op.Window != nil {
			window := *op.Window
			out.Operations[i].Window = &window
		}
		if op.Cutoff != nil {
			cutoff := *op.Cutoff
			out.Operations[i].Cutoff = &cutoff
		}
	}
	return out
}
func sortedUnique[T ~string](in []T) []T {
	out := append([]T(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return unique(out)
}
func unique[T comparable](in []T) []T {
	if len(in) == 0 {
		return nil
	}
	out := in[:1]
	for _, v := range in[1:] {
		if v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}
func sortedStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return unique(out)
}

func canonicalHosts(in []string) []string {
	out := make([]string, len(in))
	for index, host := range in {
		out[index] = strings.ToLower(host)
	}
	return sortedStrings(out)
}
func canonicalPorts(in []PortRange) []PortRange {
	sorted := append([]PortRange(nil), in...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Start == sorted[j].Start {
			return sorted[i].End < sorted[j].End
		}
		return sorted[i].Start < sorted[j].Start
	})
	out := make([]PortRange, 0, len(sorted))
	for _, current := range sorted {
		if len(out) == 0 || int(current.Start) > int(out[len(out)-1].End)+1 {
			out = append(out, current)
			continue
		}
		if current.End > out[len(out)-1].End {
			out[len(out)-1].End = current.End
		}
	}
	return out
}

// semanticStrategy excludes IDs and presentation metadata from the identity.
type semanticStrategy struct {
	SchemaVersion int             `json:"schema_version"`
	Transport     []Transport     `json:"transport"`
	Selector      TrafficSelector `json:"selector"`
	Operations    []Operation     `json:"operations"`
	Safety        SafetyPolicy    `json:"safety"`
}

func semanticJSON(s Strategy) ([]byte, error) {
	c, err := Canonicalize(s)
	if err != nil {
		return nil, err
	}
	return json.Marshal(semanticStrategy{c.SchemaVersion, c.Transport, c.Selector, c.Operations, c.Safety})
}
func Fingerprint(s Strategy) (string, error) {
	data, err := semanticJSON(s)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Marshal emits canonical semantic ordering, while preserving presentation fields.
func Marshal(s Strategy) ([]byte, error) {
	c, err := Canonicalize(s)
	if err != nil {
		return nil, err
	}
	return json.Marshal(c)
}

// UnmarshalStrict rejects unknown fields and trailing JSON, then validates the declarative schema.
func UnmarshalStrict(data []byte) (Strategy, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var s Strategy
	if err := dec.Decode(&s); err != nil {
		return Strategy{}, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return Strategy{}, fmt.Errorf("trailing JSON data")
	}
	if err := Validate(s); err != nil {
		return Strategy{}, err
	}
	return s, nil
}
