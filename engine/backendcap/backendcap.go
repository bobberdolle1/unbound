// Package backendcap describes static backend support and compiles StrategyIR without execution.
package backendcap

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"unbound/engine"
	"unbound/engine/strategyir"
)

type Backend string

const (
	Zapret2Windows    Backend = "zapret2/windows"
	Zapret2Linux      Backend = "zapret2/linux"
	Zapret1TPWSDarwin Backend = "zapret1-tpws/darwin"
	NativeWindows     Backend = "unbound-native/windows"
	NativeLinux       Backend = "unbound-native/linux"
	NativeDarwin      Backend = "unbound-native/darwin"
)

type Provenance struct {
	Tag     string `json:"tag,omitempty"`
	BaseTag string `json:"base_tag,omitempty"`
	Commit  string `json:"commit"`
}

// ProfileCapabilities describes filtering and operation semantics accepted by
// the engine process itself. It intentionally excludes packet acquisition.
type ProfileCapabilities struct {
	Transports                 []strategyir.Transport           `json:"transports"`
	ApplicationProtocols       []strategyir.ApplicationProtocol `json:"application_protocols"`
	MultiProtocolPayloadFilter bool                             `json:"multi_protocol_payload_filter"`
	IPFamilies                 []strategyir.IPFamily            `json:"ip_families"`
	Operations                 []strategyir.OperationKind       `json:"operations"`
	PositionAnchors            []strategyir.PositionAnchor      `json:"position_anchors"`
	PortRanges                 bool                             `json:"port_ranges"`
	ManagedHostlists           bool                             `json:"managed_hostlists"`
	AutoHostlists              bool                             `json:"auto_hostlists"`
	IPSetReferences            bool                             `json:"ip_set_references"`
	QUIC                       bool                             `json:"quic"`
	FakePayloads               bool                             `json:"fake_payloads"`
	FakePayloadRefs            []string                         `json:"fake_payload_refs,omitempty"`
	FakeRepeat                 bool                             `json:"fake_repeat"`
	FakeTTL                    bool                             `json:"fake_ttl"`
	FakeSequenceOffset         bool                             `json:"fake_sequence_offset"`
	FakeAcknowledgmentOffset   bool                             `json:"fake_acknowledgment_offset"`
	FakeTCPMD5                 bool                             `json:"fake_tcp_md5"`
	FakeTCPTimestamp           bool                             `json:"fake_tcp_timestamp"`
	RangeDirections            []strategyir.RangeDirection      `json:"range_directions,omitempty"`
	RangeCounters              []strategyir.RangeCounter        `json:"range_counters,omitempty"`
	LuaModules                 []string                         `json:"lua_modules,omitempty"`
	LuaFunctions               []string                         `json:"lua_functions,omitempty"`
}

type CaptureBackendKind string

const (
	CaptureWinDivert CaptureBackendKind = "WINDIVERT"
	CaptureNFQUEUE   CaptureBackendKind = "NFQUEUE"
	CaptureSOCKSTCP  CaptureBackendKind = "SOCKS_TCP"
)

type CaptureTransport string

const (
	CaptureTransportTCP CaptureTransport = "TCP"
	CaptureTransportUDP CaptureTransport = "UDP"
)

// CaptureCapabilities describes the executor-owned packet acquisition path.
// It is deliberately separate from engine/profile filtering capability.
type CaptureCapabilities struct {
	BackendKind CaptureBackendKind     `json:"backend_kind"`
	Transports  []CaptureTransport     `json:"transports"`
	Directions  []strategyir.Direction `json:"directions"`
	IPFamilies  []strategyir.IPFamily  `json:"ip_families"`
}

type Capabilities struct {
	Backend    Backend             `json:"backend"`
	Provenance Provenance          `json:"provenance"`
	Profile    ProfileCapabilities `json:"profile"`
	Capture    CaptureCapabilities `json:"capture"`
}

// Get reports static backend capability, never the current machine environment.
func Get(backend Backend) Capabilities {
	zapretOps := []strategyir.OperationKind{
		strategyir.OperationSplit, strategyir.OperationMultiSplit,
		strategyir.OperationMultiDisorder, strategyir.OperationHTTPHostCase,
		strategyir.OperationFakeInjection, strategyir.OperationHostFakeSplit,
		strategyir.OperationWindowShaping,
	}
	zapretProfile := ProfileCapabilities{
		Transports:                 []strategyir.Transport{strategyir.TransportTCP, strategyir.TransportUDP, strategyir.TransportQUIC},
		ApplicationProtocols:       []strategyir.ApplicationProtocol{strategyir.ApplicationAny, strategyir.ApplicationHTTP, strategyir.ApplicationTLS, strategyir.ApplicationQUIC},
		MultiProtocolPayloadFilter: true,
		IPFamilies:                 []strategyir.IPFamily{strategyir.IPFamilyV4, strategyir.IPFamilyV6},
		Operations:                 zapretOps,
		PositionAnchors:            []strategyir.PositionAnchor{strategyir.AnchorHost, strategyir.AnchorEndHost, strategyir.AnchorMidSLD, strategyir.AnchorSNIExt, strategyir.AnchorMethod},
		PortRanges:                 true, ManagedHostlists: true, AutoHostlists: true, IPSetReferences: true,
		QUIC: true, FakePayloads: true,
		FakeRepeat: true, FakeTTL: true, FakeSequenceOffset: true, FakeAcknowledgmentOffset: true, FakeTCPMD5: true, FakeTCPTimestamp: true,
		RangeDirections: []strategyir.RangeDirection{strategyir.RangeDirectionIn, strategyir.RangeDirectionOut},
		RangeCounters:   []strategyir.RangeCounter{strategyir.RangeCounterDataPacketNumber, strategyir.RangeCounterRelativeSequence},
		FakePayloadRefs: []string{"fake-default-udp", "quic-google", "stun-pat", "tls-clienthello-default", "tls-google"},
		LuaModules:      []string{"zapret-antidpi.lua"},
		LuaFunctions:    []string{"fake", "hostfakesplit", "multisplit", "multidisorder", "wssize"},
	}
	zapret := func(backend Backend, captureKind CaptureBackendKind) Capabilities {
		return Capabilities{
			Backend:    backend,
			Provenance: Provenance{Tag: "v1.0.5.1", Commit: "a1bca5a85e25ab138e9617a560c262fcf53e969a"},
			Profile:    zapretProfile,
			Capture: CaptureCapabilities{
				BackendKind: captureKind,
				Transports:  []CaptureTransport{CaptureTransportTCP, CaptureTransportUDP},
				Directions:  []strategyir.Direction{strategyir.DirectionOutbound, strategyir.DirectionInbound, strategyir.DirectionBoth},
				IPFamilies:  []strategyir.IPFamily{strategyir.IPFamilyV4, strategyir.IPFamilyV6},
			},
		}
	}
	switch backend {
	case Zapret2Windows:
		return zapret(backend, CaptureWinDivert)
	case Zapret2Linux:
		return zapret(backend, CaptureNFQUEUE)
	case Zapret1TPWSDarwin:
		return Capabilities{
			Backend:    backend,
			Provenance: Provenance{BaseTag: "v72.13", Commit: "d437963452674faadfd45adcd62466272b5a2fcd"},
			Profile: ProfileCapabilities{
				Transports:           []strategyir.Transport{strategyir.TransportTCP},
				ApplicationProtocols: []strategyir.ApplicationProtocol{strategyir.ApplicationAny},
				IPFamilies:           []strategyir.IPFamily{strategyir.IPFamilyV4, strategyir.IPFamilyV6},
				Operations:           []strategyir.OperationKind{strategyir.OperationSplit, strategyir.OperationMultiSplit, strategyir.OperationDisorder, strategyir.OperationTLSRecordSplit, strategyir.OperationHTTPHostCase},
				PositionAnchors:      []strategyir.PositionAnchor{strategyir.AnchorMidSLD, strategyir.AnchorMethod},
				PortRanges:           true,
			},
			Capture: CaptureCapabilities{
				BackendKind: CaptureSOCKSTCP,
				Transports:  []CaptureTransport{CaptureTransportTCP},
				Directions:  []strategyir.Direction{strategyir.DirectionOutbound},
				IPFamilies:  []strategyir.IPFamily{strategyir.IPFamilyV4, strategyir.IPFamilyV6},
			},
		}
	default:
		return Capabilities{Backend: backend}
	}
}

type CompileStatus string

const (
	StatusCompiled    CompileStatus = "COMPILED"
	StatusUnsupported CompileStatus = "UNSUPPORTED"
	StatusInvalid     CompileStatus = "INVALID"
)

type ReasonCode string

const (
	UnsupportedOperation           ReasonCode = "UNSUPPORTED_OPERATION"
	UnsupportedTransport           ReasonCode = "UNSUPPORTED_TRANSPORT"
	UnsupportedApplicationProtocol ReasonCode = "UNSUPPORTED_APPLICATION_PROTOCOL"
	UnsupportedPositionAnchor      ReasonCode = "UNSUPPORTED_POSITION_ANCHOR"
	UnsupportedIPFamily            ReasonCode = "UNSUPPORTED_IP_FAMILY"
	UnsupportedPortScope           ReasonCode = "UNSUPPORTED_PORT_SCOPE"
	UnsupportedScope               ReasonCode = "UNSUPPORTED_SCOPE"
	UnsupportedDirection           ReasonCode = "UNSUPPORTED_DIRECTION"
	UnsupportedCapture             ReasonCode = "UNSUPPORTED_CAPTURE"
	UnsupportedFakeModifier        ReasonCode = "UNSUPPORTED_FAKE_MODIFIER"
	UnsupportedCutoff              ReasonCode = "UNSUPPORTED_CUTOFF"
	MissingAsset                   ReasonCode = "MISSING_ASSET"
	EngineVersionTooOld            ReasonCode = "ENGINE_VERSION_TOO_OLD"
	InvalidIR                      ReasonCode = "INVALID_IR"
)

type Reason struct {
	Code   ReasonCode `json:"code"`
	Detail string     `json:"detail"`
}

// CapturePlan is structured executor input. It deliberately stores no shell,
// firewall, or engine option strings.
type CapturePlan struct {
	BackendKind    CaptureBackendKind     `json:"backend_kind"`
	Transport      CaptureTransport       `json:"transport"`
	Direction      strategyir.Direction   `json:"direction"`
	IPFamilies     []strategyir.IPFamily  `json:"ip_families"`
	TCPPorts       []strategyir.PortRange `json:"tcp_ports,omitempty"`
	UDPPorts       []strategyir.PortRange `json:"udp_ports,omitempty"`
	RawCaptureRefs []string               `json:"raw_capture_refs,omitempty"`
}

type Plan struct {
	// EngineArgv contains only engine/profile flags. ${asset:<logical-id>} must
	// be resolved by trusted product code.
	EngineArgv []string    `json:"engine_argv"`
	Capture    CapturePlan `json:"capture"`
	AssetRefs  []string    `json:"asset_refs,omitempty"`
}

type CompileResult struct {
	Status              CompileStatus               `json:"status"`
	Backend             Backend                     `json:"backend"`
	StrategyFingerprint string                      `json:"strategy_fingerprint,omitempty"`
	Plan                Plan                        `json:"plan,omitempty"`
	RequiredAssets      []string                    `json:"required_assets,omitempty"`
	DerivedRequirements engine.StrategyRequirements `json:"derived_requirements"`
	Unsupported         []Reason                    `json:"unsupported_reasons,omitempty"`
	Warnings            []string                    `json:"warnings,omitempty"`
}

// Compile is pure. It fails closed before any unsupported semantic element can be omitted.
func Compile(strategy strategyir.Strategy, backend Backend) CompileResult {
	result := CompileResult{Backend: backend}
	normalized, err := strategyir.Canonicalize(strategy)
	if err != nil {
		result.Status = StatusInvalid
		result.Unsupported = []Reason{{Code: InvalidIR, Detail: err.Error()}}
		return result
	}
	fingerprint, err := strategyir.Fingerprint(normalized)
	if err != nil {
		result.Status = StatusInvalid
		result.Unsupported = []Reason{{Code: InvalidIR, Detail: err.Error()}}
		return result
	}
	result.StrategyFingerprint = fingerprint
	if !knownBackend(backend) {
		result.Status = StatusUnsupported
		result.Unsupported = []Reason{{Code: UnsupportedOperation, Detail: "native backend has no implementation"}}
		return result
	}
	if reasons := compatibility(normalized, Get(backend)); len(reasons) != 0 {
		result.Status = StatusUnsupported
		result.Unsupported = reasons
		return result
	}
	engineArgv, assets, err := compileArgs(normalized, backend)
	if err != nil {
		result.Status = StatusUnsupported
		result.Unsupported = []Reason{{Code: UnsupportedOperation, Detail: err.Error()}}
		return result
	}
	capture, err := compileCapture(normalized, backend)
	if err != nil {
		result.Status = StatusUnsupported
		result.Unsupported = []Reason{{Code: UnsupportedCapture, Detail: err.Error()}}
		return result
	}
	result.Status = StatusCompiled
	result.Plan = Plan{EngineArgv: engineArgv, Capture: capture, AssetRefs: assets}
	result.RequiredAssets = assets
	result.DerivedRequirements = deriveRequirements(normalized, backend)
	return result
}

func knownBackend(backend Backend) bool {
	return backend == Zapret2Windows || backend == Zapret2Linux || backend == Zapret1TPWSDarwin
}

func compatibility(strategy strategyir.Strategy, caps Capabilities) []Reason {
	var reasons []Reason
	add := func(code ReasonCode, detail string) { reasons = append(reasons, Reason{Code: code, Detail: detail}) }
	profile := caps.Profile
	for _, transport := range strategy.Transport {
		if !has(profile.Transports, transport) {
			add(UnsupportedTransport, string(transport))
		}
	}
	if !has(caps.Capture.Transports, captureTransportFor(strategy.Transport[0])) {
		add(UnsupportedCapture, "capture cannot acquire "+string(strategy.Transport[0]))
	}
	for _, protocol := range strategy.Selector.ApplicationProtocols {
		if !has(profile.ApplicationProtocols, protocol) {
			add(UnsupportedApplicationProtocol, string(protocol))
		}
	}
	if len(strategy.Selector.ApplicationProtocols) > 1 && !profile.MultiProtocolPayloadFilter {
		add(UnsupportedApplicationProtocol, "backend cannot preserve multiple application protocols")
	}
	if !supportsFamilies(strategy.Selector.IPFamilies, profile.IPFamilies) {
		add(UnsupportedIPFamily, "engine/profile filter cannot preserve requested IP families")
	}
	if !supportsFamilies(strategy.Selector.IPFamilies, caps.Capture.IPFamilies) {
		add(UnsupportedIPFamily, "capture cannot preserve requested IP families")
	}
	if !has(caps.Capture.Directions, strategy.Selector.Direction) {
		add(UnsupportedDirection, string(strategy.Selector.Direction))
	}
	if !profile.PortRanges && (len(strategy.Selector.TCPPorts) > 1 || len(strategy.Selector.UDPPorts) > 1) {
		add(UnsupportedPortScope, "multiple port ranges")
	}
	host := strategy.Selector.Scope.Host
	if (host.Mode == strategyir.HostScopeManagedList && !profile.ManagedHostlists) ||
		(host.Mode == strategyir.HostScopeAutoHostlist && !profile.AutoHostlists) ||
		(host.Mode == strategyir.HostScopeIPSetReference && !profile.IPSetReferences) ||
		(len(strategy.Selector.Scope.IPSetIDs) > 0 && !profile.IPSetReferences) {
		add(UnsupportedScope, "host/IP logical scope")
	}
	if !profile.ManagedHostlists && (host.Mode == strategyir.HostScopeExplicit || len(strategy.Selector.Scope.ExcludeHostListIDs) != 0 || len(strategy.Selector.Scope.ExcludeIPSetIDs) != 0) {
		add(UnsupportedScope, "backend cannot preserve host/IP scope")
	}
	for _, operation := range strategy.Operations {
		if !has(profile.Operations, operation.Type) {
			add(UnsupportedOperation, string(operation.Type))
		}
		for _, position := range operation.Positions {
			if position.Anchor != "" && !has(profile.PositionAnchors, position.Anchor) {
				add(UnsupportedPositionAnchor, string(position.Anchor))
			}
		}
		if operation.PayloadRef != "" && !profile.FakePayloads {
			add(MissingAsset, "backend does not accept fake payload assets")
		}
		if operation.OverlapPatternRef != "" && !profile.FakePayloads {
			add(MissingAsset, "backend does not accept overlap pattern assets")
		}
		if operation.PayloadRef != "" && !has(profile.FakePayloadRefs, operation.PayloadRef) {
			add(MissingAsset, operation.PayloadRef)
		}
		if operation.OverlapPatternRef != "" && !has(profile.FakePayloadRefs, operation.OverlapPatternRef) {
			add(MissingAsset, operation.OverlapPatternRef)
		}
		if operation.Fake != nil {
			if operation.Fake.Repeat > 0 && !profile.FakeRepeat {
				add(UnsupportedFakeModifier, "repeat")
			}
			if operation.Fake.TTL != nil && !profile.FakeTTL {
				add(UnsupportedFakeModifier, "TTL")
			}
			if operation.Fake.SequenceOffset != nil && !profile.FakeSequenceOffset {
				add(UnsupportedFakeModifier, "sequence offset")
			}
			if operation.Fake.AcknowledgmentOffset != nil && !profile.FakeAcknowledgmentOffset {
				add(UnsupportedFakeModifier, "acknowledgment offset")
			}
			if operation.Fake.TCPMD5 && !profile.FakeTCPMD5 {
				add(UnsupportedFakeModifier, "TCP MD5")
			}
			if operation.Fake.TCPTimestamp && !profile.FakeTCPTimestamp {
				add(UnsupportedFakeModifier, "TCP timestamp")
			}
		}
	}
	if strategy.Range != nil {
		if !has(profile.RangeDirections, strategy.Range.Direction) {
			add(UnsupportedCutoff, "direction "+string(strategy.Range.Direction))
		}
		if !has(profile.RangeCounters, strategy.Range.Counter) {
			add(UnsupportedCutoff, "counter "+string(strategy.Range.Counter))
		}
	}
	return dedupeReasons(reasons)
}

func supportsFamilies(required, available []strategyir.IPFamily) bool {
	if len(required) == 1 && required[0] == strategyir.IPFamilyAny {
		return has(available, strategyir.IPFamilyV4) && has(available, strategyir.IPFamilyV6)
	}
	for _, family := range required {
		if !has(available, family) {
			return false
		}
	}
	return true
}

func has[T comparable](items []T, item T) bool {
	for _, candidate := range items {
		if candidate == item {
			return true
		}
	}
	return false
}
func dedupeReasons(in []Reason) []Reason {
	seen := map[string]bool{}
	out := make([]Reason, 0, len(in))
	for _, reason := range in {
		key := string(reason.Code) + "\x00" + reason.Detail
		if !seen[key] {
			seen[key] = true
			out = append(out, reason)
		}
	}
	return out
}

func compileArgs(strategy strategyir.Strategy, backend Backend) ([]string, []string, error) {
	if backend == Zapret1TPWSDarwin {
		return compileTPWS(strategy)
	}
	args := selectorArgs(strategy)
	assets := scopeArgs(&args, strategy.Selector.Scope)
	if strategy.Range != nil {
		rangeArg, err := cutoffArg(*strategy.Range)
		if err != nil {
			return nil, nil, err
		}
		args = append(args, rangeArg)
	}
	for _, operation := range strategy.Operations {
		operationArgs, operationAssets, err := compileZapretOperation(operation)
		if err != nil {
			return nil, nil, err
		}
		args = append(args, operationArgs...)
		assets = append(assets, operationAssets...)
	}
	return args, sortedUniqueStrings(assets), nil
}

func selectorArgs(strategy strategyir.Strategy) []string {
	args := []string{"--filter-l3=" + familyList(strategy.Selector.IPFamilies)}
	if has(strategy.Transport, strategyir.TransportTCP) {
		args = append(args, "--filter-tcp="+ports(strategy.Selector.TCPPorts))
	}
	if has(strategy.Transport, strategyir.TransportUDP) || has(strategy.Transport, strategyir.TransportQUIC) {
		args = append(args, "--filter-udp="+ports(strategy.Selector.UDPPorts))
	}
	if strategy.Selector.ApplicationProtocols[0] != strategyir.ApplicationAny {
		payloads := make([]string, len(strategy.Selector.ApplicationProtocols))
		for i, protocol := range strategy.Selector.ApplicationProtocols {
			payloads[i] = payloadName(protocol)
		}
		args = append(args, "--payload="+strings.Join(payloads, ","))
	}
	return args
}

func compileCapture(strategy strategyir.Strategy, backend Backend) (CapturePlan, error) {
	kind := CaptureBackendKind("")
	switch backend {
	case Zapret2Windows:
		kind = CaptureWinDivert
	case Zapret2Linux:
		kind = CaptureNFQUEUE
	case Zapret1TPWSDarwin:
		kind = CaptureSOCKSTCP
	default:
		return CapturePlan{}, fmt.Errorf("unsupported capture backend %s", backend)
	}
	return CapturePlan{
		BackendKind: kind,
		Transport:   captureTransportFor(strategy.Transport[0]),
		Direction:   strategy.Selector.Direction,
		IPFamilies:  append([]strategyir.IPFamily(nil), strategy.Selector.IPFamilies...),
		TCPPorts:    append([]strategyir.PortRange(nil), strategy.Selector.TCPPorts...),
		UDPPorts:    append([]strategyir.PortRange(nil), strategy.Selector.UDPPorts...),
	}, nil
}

func captureTransportFor(transport strategyir.Transport) CaptureTransport {
	if transport == strategyir.TransportTCP {
		return CaptureTransportTCP
	}
	return CaptureTransportUDP
}

// RenderWindowsCaptureArgv generates WinDivert constructor options from a
// structured capture plan. Linux NFQUEUE and macOS SOCKS capture have no
// engine argv representation.
func RenderWindowsCaptureArgv(capture CapturePlan) ([]string, error) {
	if capture.BackendKind != CaptureWinDivert {
		return nil, fmt.Errorf("capture backend %s does not use WinDivert argv", capture.BackendKind)
	}
	args := []string{"--wf-l3=" + familyList(capture.IPFamilies)}
	var portsForTransport []strategyir.PortRange
	var proto string
	switch capture.Transport {
	case CaptureTransportTCP:
		portsForTransport, proto = capture.TCPPorts, "tcp"
	case CaptureTransportUDP:
		portsForTransport, proto = capture.UDPPorts, "udp"
	default:
		return nil, fmt.Errorf("unsupported WinDivert transport %s", capture.Transport)
	}
	portScope := ports(portsForTransport)
	switch capture.Direction {
	case strategyir.DirectionInbound:
		return append(args, "--wf-"+proto+"-in="+portScope), nil
	case strategyir.DirectionBoth:
		return append(args, "--wf-"+proto+"-in="+portScope, "--wf-"+proto+"-out="+portScope), nil
	case strategyir.DirectionOutbound:
		return append(args, "--wf-"+proto+"-out="+portScope), nil
	default:
		return nil, fmt.Errorf("unsupported WinDivert direction %s", capture.Direction)
	}
}
func payloadName(protocol strategyir.ApplicationProtocol) string {
	switch protocol {
	case strategyir.ApplicationHTTP:
		return "http_req"
	case strategyir.ApplicationTLS:
		return "tls_client_hello"
	case strategyir.ApplicationQUIC:
		return "quic_initial"
	default:
		return "all"
	}
}

func familyList(families []strategyir.IPFamily) string {
	if len(families) == 1 && families[0] == strategyir.IPFamilyAny {
		return "ipv4,ipv6"
	}
	values := make([]string, len(families))
	for i, family := range families {
		switch family {
		case strategyir.IPFamilyV4:
			values[i] = "ipv4"
		case strategyir.IPFamilyV6:
			values[i] = "ipv6"
		}
	}
	return strings.Join(values, ",")
}

func scopeArgs(args *[]string, scope strategyir.Scope) []string {
	assets := []string{}
	host := scope.Host
	switch host.Mode {
	case strategyir.HostScopeExplicit:
		*args = append(*args, "--hostlist-domains="+strings.Join(sortedUniqueStrings(host.Hosts), ","))
	case strategyir.HostScopeManagedList:
		*args = append(*args, "--hostlist=${asset:"+host.ID+"}")
		assets = append(assets, host.ID)
	case strategyir.HostScopeAutoHostlist:
		*args = append(*args, "--hostlist-auto=${asset:"+host.ID+"}")
		assets = append(assets, host.ID)
	case strategyir.HostScopeIPSetReference:
		*args = append(*args, "--ipset=${asset:"+host.ID+"}")
		assets = append(assets, host.ID)
	}
	for _, id := range sortedUniqueStrings(scope.ExcludeHostListIDs) {
		*args = append(*args, "--hostlist-exclude=${asset:"+id+"}")
		assets = append(assets, id)
	}
	for _, id := range sortedUniqueStrings(scope.IPSetIDs) {
		*args = append(*args, "--ipset=${asset:"+id+"}")
		assets = append(assets, id)
	}
	for _, id := range sortedUniqueStrings(scope.ExcludeIPSetIDs) {
		*args = append(*args, "--ipset-exclude=${asset:"+id+"}")
		assets = append(assets, id)
	}
	return assets
}

func compileZapretOperation(operation strategyir.Operation) ([]string, []string, error) {
	args, assets := []string{}, []string{}
	positions := positionList(operation.Positions)
	var value string
	switch operation.Type {
	case strategyir.OperationSplit, strategyir.OperationMultiSplit:
		value = "multisplit:pos=" + positions
	case strategyir.OperationMultiDisorder:
		value = "multidisorder:pos=" + positions
	case strategyir.OperationHostFakeSplit:
		value = "hostfakesplit:midhost=" + positions
		if operation.HostTemplate != "" {
			value += ":host=" + operation.HostTemplate
		}
	case strategyir.OperationFakeInjection:
		value = "fake:blob=${asset:" + operation.PayloadRef + "}"
		assets = append(assets, operation.PayloadRef)
	case strategyir.OperationHTTPHostCase:
		value = "http_hostcase"
	case strategyir.OperationWindowShaping:
		if operation.Window == nil {
			return nil, nil, fmt.Errorf("window shaping has no window")
		}
		value = "wssize:wsize=" + strconv.Itoa(operation.Window.Window)
		if operation.Window.Scale > 0 {
			value += ":scale=" + strconv.Itoa(operation.Window.Scale)
		}
	default:
		return nil, nil, fmt.Errorf("unsupported operation %s", operation.Type)
	}
	if operation.SequenceOverlap != nil {
		value += ":seqovl=" + strconv.Itoa(*operation.SequenceOverlap)
	}
	if operation.OverlapPatternRef != "" {
		value += ":seqovl_pattern=${asset:" + operation.OverlapPatternRef + "}"
		assets = append(assets, operation.OverlapPatternRef)
	}
	if operation.Fake != nil {
		value += fakeSuffix(*operation.Fake)
	}
	args = append(args, "--lua-desync="+value)
	return args, assets, nil
}
func fakeSuffix(fake strategyir.FakeModifiers) string {
	value := ""
	if fake.Repeat > 0 {
		value += ":repeats=" + strconv.Itoa(fake.Repeat)
	}
	if fake.TTL != nil {
		value += ":ttl=" + strconv.Itoa(*fake.TTL)
	}
	if fake.SequenceOffset != nil {
		value += ":tcp_seq=" + strconv.Itoa(*fake.SequenceOffset)
	}
	if fake.AcknowledgmentOffset != nil {
		value += ":tcp_ack=" + strconv.Itoa(*fake.AcknowledgmentOffset)
	}
	if fake.TCPMD5 {
		value += ":tcp_md5"
	}
	if fake.TCPTimestamp {
		value += ":tcp_ts"
	}
	return value
}

func cutoffArg(cutoff strategyir.Cutoff) (string, error) {
	var flag, counter string
	switch cutoff.Direction {
	case strategyir.RangeDirectionIn:
		flag = "--in-range="
	case strategyir.RangeDirectionOut:
		flag = "--out-range="
	default:
		return "", fmt.Errorf("unsupported cutoff direction %s", cutoff.Direction)
	}
	switch cutoff.Counter {
	case strategyir.RangeCounterDataPacketNumber:
		counter = "d"
	case strategyir.RangeCounterRelativeSequence:
		counter = "s"
	default:
		return "", fmt.Errorf("unsupported cutoff counter %s", cutoff.Counter)
	}
	return flag + "-" + counter + strconv.Itoa(cutoff.Limit), nil
}
func positionList(positions []strategyir.PositionExpr) string {
	values := make([]string, len(positions))
	for index, position := range positions {
		if position.Absolute != nil {
			values[index] = strconv.Itoa(*position.Absolute)
		} else {
			values[index] = strings.ToLower(string(position.Anchor))
			if position.Offset > 0 {
				values[index] += "+" + strconv.Itoa(position.Offset)
			} else if position.Offset < 0 {
				values[index] += strconv.Itoa(position.Offset)
			}
		}
	}
	return strings.Join(values, ",")
}

func compileTPWS(strategy strategyir.Strategy) ([]string, []string, error) {
	args := []string{"--filter-l3=" + familyList(strategy.Selector.IPFamilies)}
	if len(strategy.Selector.TCPPorts) > 0 {
		args = append(args, "--filter-tcp="+ports(strategy.Selector.TCPPorts))
	}
	for _, operation := range strategy.Operations {
		switch operation.Type {
		case strategyir.OperationSplit, strategyir.OperationMultiSplit:
			args = append(args, "--split-pos="+positionList(operation.Positions))
		case strategyir.OperationHTTPHostCase:
			args = append(args, "--hostcase")
		case strategyir.OperationDisorder:
			args = append(args, "--disorder")
		case strategyir.OperationTLSRecordSplit:
			args = append(args, "--tlsrec="+positionList(operation.Positions))
		default:
			return nil, nil, fmt.Errorf("tpws does not support %s", operation.Type)
		}
	}
	return args, nil, nil
}
func ports(ports []strategyir.PortRange) string {
	values := make([]string, len(ports))
	for index, port := range ports {
		if port.Start == port.End {
			values[index] = strconv.Itoa(int(port.Start))
		} else {
			values[index] = strconv.Itoa(int(port.Start)) + "-" + strconv.Itoa(int(port.End))
		}
	}
	return strings.Join(values, ",")
}
func sortedUniqueStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	index := 1
	for _, value := range out[1:] {
		if value != out[index-1] {
			out[index] = value
			index++
		}
	}
	return out[:index]
}

func deriveRequirements(strategy strategyir.Strategy, backend Backend) engine.StrategyRequirements {
	requirements := engine.StrategyRequirements{TCPTimestamps: engine.TimestampsNone}
	if backend == Zapret2Windows || backend == Zapret2Linux {
		requirements.EngineMinVersion = "v1.0.5.1"
		requirements.LuaModules = []string{"zapret-antidpi.lua"}
		if len(payloadAssets(strategy)) != 0 {
			requirements.LuaModules = append(requirements.LuaModules, "init_vars.lua")
		}
	}
	if strategy.Selector.Direction == strategyir.DirectionInbound || strategy.Selector.Direction == strategyir.DirectionBoth {
		if has(strategy.Transport, strategyir.TransportTCP) {
			requirements.InboundTCP = true
		}
		if has(strategy.Transport, strategyir.TransportUDP) || has(strategy.Transport, strategyir.TransportQUIC) {
			requirements.InboundUDP = true
		}
	}
	if has(strategy.Transport, strategyir.TransportQUIC) {
		requirements.QUIC = true
	}
	for _, family := range strategy.Selector.IPFamilies {
		if family == strategyir.IPFamilyV6 {
			requirements.IPv6 = true
		}
	}
	for _, operation := range strategy.Operations {
		if operation.Fake != nil && operation.Fake.TCPTimestamp {
			requirements.TCPTimestamps = engine.TimestampsRequired
			break
		}
	}
	return requirements
}

func payloadAssets(strategy strategyir.Strategy) []string {
	assets := []string{}
	for _, operation := range strategy.Operations {
		if operation.PayloadRef != "" {
			assets = append(assets, operation.PayloadRef)
		}
		if operation.OverlapPatternRef != "" {
			assets = append(assets, operation.OverlapPatternRef)
		}
	}
	return sortedUniqueStrings(assets)
}
