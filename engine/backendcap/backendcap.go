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

type Capabilities struct {
	Backend          Backend                     `json:"backend"`
	Provenance       Provenance                  `json:"provenance"`
	Transports       []strategyir.Transport      `json:"transports"`
	IPFamilies       []strategyir.IPFamily       `json:"ip_families"`
	Directions       []strategyir.Direction      `json:"directions"`
	Operations       []strategyir.OperationKind  `json:"operations"`
	PositionAnchors  []strategyir.PositionAnchor `json:"position_anchors"`
	PortRanges       bool                        `json:"port_ranges"`
	ManagedHostlists bool                        `json:"managed_hostlists"`
	AutoHostlists    bool                        `json:"auto_hostlists"`
	IPSetReferences  bool                        `json:"ip_set_references"`
	InboundCapture   bool                        `json:"inbound_capture"`
	QUIC             bool                        `json:"quic"`
	FakePayloads     bool                        `json:"fake_payloads"`
	FakePayloadRefs  []string                    `json:"fake_payload_refs,omitempty"`
	TTL              bool                        `json:"ttl"`
	TCPMD5           bool                        `json:"tcp_md5"`
	TCPTimestamp     bool                        `json:"tcp_timestamp"`
	PacketCutoff     bool                        `json:"packet_cutoff"`
	SequenceCutoff   bool                        `json:"sequence_cutoff"`
	LuaModules       []string                    `json:"lua_modules,omitempty"`
	LuaFunctions     []string                    `json:"lua_functions,omitempty"`
}

// Get reports static backend capability, never the current machine environment.
func Get(backend Backend) Capabilities {
	zapretOps := []strategyir.OperationKind{
		strategyir.OperationSplit, strategyir.OperationMultiSplit,
		strategyir.OperationMultiDisorder, strategyir.OperationHTTPHostCase,
		strategyir.OperationFakeInjection, strategyir.OperationHostFakeSplit,
		strategyir.OperationWindowShaping,
	}
	zapret := func(backend Backend) Capabilities {
		return Capabilities{
			Backend:         backend,
			Provenance:      Provenance{Tag: "v1.0.5.1", Commit: "a1bca5a85e25ab138e9617a560c262fcf53e969a"},
			Transports:      []strategyir.Transport{strategyir.TransportTCP, strategyir.TransportUDP, strategyir.TransportQUIC},
			IPFamilies:      []strategyir.IPFamily{strategyir.IPFamilyV4, strategyir.IPFamilyV6},
			Directions:      []strategyir.Direction{strategyir.DirectionOutbound, strategyir.DirectionInbound, strategyir.DirectionBoth},
			Operations:      zapretOps,
			PositionAnchors: []strategyir.PositionAnchor{strategyir.AnchorHost, strategyir.AnchorEndHost, strategyir.AnchorMidSLD, strategyir.AnchorSNIExt, strategyir.AnchorMethod},
			PortRanges:      true, ManagedHostlists: true, AutoHostlists: true, IPSetReferences: true,
			InboundCapture: true, QUIC: true, FakePayloads: true, TTL: true, TCPMD5: true,
			TCPTimestamp: true, PacketCutoff: true, SequenceCutoff: true,
			FakePayloadRefs: []string{"fake-default-udp", "quic-google", "stun-pat", "tls-clienthello-default", "tls-google"},
			LuaModules:      []string{"zapret-antidpi.lua"},
			LuaFunctions:    []string{"fake", "hostfakesplit", "multisplit", "multidisorder", "wssize"},
		}
	}
	switch backend {
	case Zapret2Windows, Zapret2Linux:
		return zapret(backend)
	case Zapret1TPWSDarwin:
		return Capabilities{
			Backend:         backend,
			Provenance:      Provenance{BaseTag: "v72.13", Commit: "d437963452674faadfd45adcd62466272b5a2fcd"},
			Transports:      []strategyir.Transport{strategyir.TransportTCP},
			IPFamilies:      []strategyir.IPFamily{strategyir.IPFamilyV4, strategyir.IPFamilyV6},
			Directions:      []strategyir.Direction{strategyir.DirectionOutbound},
			Operations:      []strategyir.OperationKind{strategyir.OperationSplit, strategyir.OperationMultiSplit, strategyir.OperationDisorder, strategyir.OperationTLSRecordSplit, strategyir.OperationHTTPHostCase},
			PositionAnchors: []strategyir.PositionAnchor{strategyir.AnchorMidSLD, strategyir.AnchorMethod},
			PortRanges:      true,
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
	UnsupportedOperation      ReasonCode = "UNSUPPORTED_OPERATION"
	UnsupportedTransport      ReasonCode = "UNSUPPORTED_TRANSPORT"
	UnsupportedPositionAnchor ReasonCode = "UNSUPPORTED_POSITION_ANCHOR"
	UnsupportedIPFamily       ReasonCode = "UNSUPPORTED_IP_FAMILY"
	UnsupportedPortScope      ReasonCode = "UNSUPPORTED_PORT_SCOPE"
	UnsupportedDirection      ReasonCode = "UNSUPPORTED_DIRECTION"
	UnsupportedFakeModifier   ReasonCode = "UNSUPPORTED_FAKE_MODIFIER"
	UnsupportedCutoff         ReasonCode = "UNSUPPORTED_CUTOFF"
	MissingAsset              ReasonCode = "MISSING_ASSET"
	EngineVersionTooOld       ReasonCode = "ENGINE_VERSION_TOO_OLD"
	InvalidIR                 ReasonCode = "INVALID_IR"
)

type Reason struct {
	Code   ReasonCode `json:"code"`
	Detail string     `json:"detail"`
}

type Plan struct {
	// Argv contains backend flags. ${asset:<logical-id>} must be resolved by trusted product code.
	Argv      []string `json:"argv"`
	AssetRefs []string `json:"asset_refs,omitempty"`
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
	fingerprint, err := strategyir.Fingerprint(strategy)
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
	if reasons := compatibility(strategy, Get(backend)); len(reasons) != 0 {
		result.Status = StatusUnsupported
		result.Unsupported = reasons
		return result
	}
	argv, assets, err := compileArgs(strategy, backend)
	if err != nil {
		result.Status = StatusUnsupported
		result.Unsupported = []Reason{{Code: UnsupportedOperation, Detail: err.Error()}}
		return result
	}
	result.Status = StatusCompiled
	result.Plan = Plan{Argv: argv, AssetRefs: assets}
	result.RequiredAssets = assets
	result.DerivedRequirements = deriveRequirements(strategy, backend)
	return result
}

func knownBackend(backend Backend) bool {
	return backend == Zapret2Windows || backend == Zapret2Linux || backend == Zapret1TPWSDarwin
}

func compatibility(strategy strategyir.Strategy, caps Capabilities) []Reason {
	var reasons []Reason
	add := func(code ReasonCode, detail string) { reasons = append(reasons, Reason{Code: code, Detail: detail}) }
	for _, transport := range strategy.Transport {
		if !has(caps.Transports, transport) {
			add(UnsupportedTransport, string(transport))
		}
	}
	for _, family := range strategy.Selector.IPFamilies {
		if family != strategyir.IPFamilyAny && !has(caps.IPFamilies, family) {
			add(UnsupportedIPFamily, string(family))
		}
	}
	if !has(caps.Directions, strategy.Selector.Direction) {
		add(UnsupportedDirection, string(strategy.Selector.Direction))
	}
	if !caps.PortRanges && (len(strategy.Selector.TCPPorts) > 1 || len(strategy.Selector.UDPPorts) > 1) {
		add(UnsupportedPortScope, "multiple port ranges")
	}
	host := strategy.Selector.Scope.Host
	if (host.Mode == strategyir.HostScopeManagedList && !caps.ManagedHostlists) ||
		(host.Mode == strategyir.HostScopeAutoHostlist && !caps.AutoHostlists) ||
		(host.Mode == strategyir.HostScopeIPSetReference && !caps.IPSetReferences) ||
		(len(strategy.Selector.Scope.IPSetIDs) > 0 && !caps.IPSetReferences) {
		add(UnsupportedPortScope, "host/IP logical scope")
	}
	if !caps.ManagedHostlists && (host.Mode == strategyir.HostScopeExplicit || len(strategy.Selector.Scope.ExcludeHostListIDs) != 0 || len(strategy.Selector.Scope.ExcludeIPSetIDs) != 0) {
		add(UnsupportedPortScope, "backend cannot preserve host/IP scope")
	}
	for _, operation := range strategy.Operations {
		if !has(caps.Operations, operation.Type) {
			add(UnsupportedOperation, string(operation.Type))
		}
		for _, position := range operation.Positions {
			if position.Anchor != "" && !has(caps.PositionAnchors, position.Anchor) {
				add(UnsupportedPositionAnchor, string(position.Anchor))
			}
		}
		if operation.PayloadRef != "" && !caps.FakePayloads {
			add(MissingAsset, "backend does not accept fake payload assets")
		}
		if operation.OverlapPatternRef != "" && !caps.FakePayloads {
			add(MissingAsset, "backend does not accept overlap pattern assets")
		}
		if operation.PayloadRef != "" && !has(caps.FakePayloadRefs, operation.PayloadRef) {
			add(MissingAsset, operation.PayloadRef)
		}
		if operation.OverlapPatternRef != "" && !has(caps.FakePayloadRefs, operation.OverlapPatternRef) {
			add(MissingAsset, operation.OverlapPatternRef)
		}
		if operation.Fake != nil {
			if operation.Fake.TTL != nil && !caps.TTL {
				add(UnsupportedFakeModifier, "TTL")
			}
			if operation.Fake.TCPMD5 && !caps.TCPMD5 {
				add(UnsupportedFakeModifier, "TCP MD5")
			}
			if operation.Fake.TCPTimestamp && !caps.TCPTimestamp {
				add(UnsupportedFakeModifier, "TCP timestamp")
			}
		}
		if operation.Cutoff != nil {
			if operation.Cutoff.PacketCount != nil && !caps.PacketCutoff {
				add(UnsupportedCutoff, "packet count")
			}
			if operation.Cutoff.SequenceBytes != nil && !caps.SequenceCutoff {
				add(UnsupportedCutoff, "sequence bytes")
			}
		}
	}
	return dedupeReasons(reasons)
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
	args := []string{}
	if has(strategy.Transport, strategyir.TransportTCP) && len(strategy.Selector.TCPPorts) > 0 {
		args = append(args, "--filter-tcp="+ports(strategy.Selector.TCPPorts))
		args = append(args, captureArgs("tcp", strategy.Selector.Direction, ports(strategy.Selector.TCPPorts))...)
	}
	if (has(strategy.Transport, strategyir.TransportUDP) || has(strategy.Transport, strategyir.TransportQUIC)) && len(strategy.Selector.UDPPorts) > 0 {
		args = append(args, "--filter-udp="+ports(strategy.Selector.UDPPorts))
		args = append(args, captureArgs("udp", strategy.Selector.Direction, ports(strategy.Selector.UDPPorts))...)
	}
	if len(strategy.Selector.ApplicationProtocols) == 1 && strategy.Selector.ApplicationProtocols[0] != strategyir.ApplicationAny {
		args = append(args, "--payload="+payloadName(strategy.Selector.ApplicationProtocols[0]))
	}
	return args
}
func captureArgs(proto string, direction strategyir.Direction, portScope string) []string {
	switch direction {
	case strategyir.DirectionInbound:
		return []string{"--wf-" + proto + "-in=" + portScope}
	case strategyir.DirectionBoth:
		return []string{"--wf-" + proto + "-in=" + portScope, "--wf-" + proto + "-out=" + portScope}
	default:
		return []string{"--wf-" + proto + "-out=" + portScope}
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
	if operation.Cutoff != nil {
		if operation.Cutoff.PacketCount != nil {
			args = append(args, "--out-range=-d"+strconv.Itoa(*operation.Cutoff.PacketCount))
		}
		if operation.Cutoff.SequenceBytes != nil {
			args = append(args, "--in-range=-s"+strconv.Itoa(*operation.Cutoff.SequenceBytes))
		}
	}
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
	args := []string{}
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
