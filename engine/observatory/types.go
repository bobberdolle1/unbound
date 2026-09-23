// Package observatory collects read-only, stage-bounded connectivity evidence.
// It deliberately does not control bypass providers, system networking, or strategy selection.
package observatory

import (
	"time"

	"unbound/engine"
)

const SchemaVersion = 1

type Stage string

const (
	StageResolve   Stage = "resolve"
	StageConnect   Stage = "connect"
	StageHello     Stage = "hello"
	StageHandshake Stage = "handshake"
	StageHTTP      Stage = "http"
	StageCarry     Stage = "carry"
)

type Status string

const (
	StatusPass               Status = "PASS"
	StatusFail               Status = "FAIL"
	StatusTimeout            Status = "TIMEOUT"
	StatusReset              Status = "RESET"
	StatusSkipped            Status = "SKIPPED"
	StatusSkippedUnsupported Status = "SKIPPED_UNSUPPORTED"
	StatusNotReached         Status = "NOT_REACHED"
	StatusCancelled          Status = "CANCELLED"
)

// Classification is deliberately factual. It never asserts a censorship
// mechanism or assigns credit/blame to a packet strategy.
type Classification string

const (
	ClassSuccess               Classification = "SUCCESS"
	ClassDNSFailure            Classification = "DNS_FAILURE"
	ClassTCPConnectTimeout     Classification = "TCP_CONNECT_TIMEOUT"
	ClassTCPConnectionRefused  Classification = "TCP_CONNECTION_REFUSED"
	ClassTCPConnectionReset    Classification = "TCP_CONNECTION_RESET"
	ClassTCPNetworkUnreachable Classification = "TCP_NETWORK_UNREACHABLE"
	ClassTCPOtherFailure       Classification = "TCP_OTHER_FAILURE"
	ClassTLSHandshakeTimeout   Classification = "TLS_HANDSHAKE_TIMEOUT"
	ClassTLSHandshakeReset     Classification = "TLS_HANDSHAKE_RESET"
	ClassTLSCertificateFailure Classification = "TLS_CERTIFICATE_FAILURE"
	ClassTLSProtocolFailure    Classification = "TLS_PROTOCOL_FAILURE"
	ClassTLSOtherFailure       Classification = "TLS_OTHER_FAILURE"
	ClassHTTPStatus            Classification = "HTTP_STATUS"
	ClassHTTPTimeout           Classification = "HTTP_TIMEOUT"
	ClassHTTPReset             Classification = "HTTP_RESET"
	ClassHTTPProtocolFailure   Classification = "HTTP_PROTOCOL_FAILURE"
	ClassQUICUnsupported       Classification = "QUIC_UNSUPPORTED"
	ClassQUICHandshakeTimeout  Classification = "QUIC_HANDSHAKE_TIMEOUT"
	ClassQUICHandshakeFailure  Classification = "QUIC_HANDSHAKE_FAILURE"
	ClassTransportFailure      Classification = "TRANSPORT_FAILURE"
	ClassRemoteFailure         Classification = "REMOTE_FAILURE"
	ClassUnknown               Classification = "UNKNOWN"
)

type AddressFamily string

const (
	AddressFamilyAny  AddressFamily = "any"
	AddressFamilyIPv4 AddressFamily = "ipv4"
	AddressFamilyIPv6 AddressFamily = "ipv6"
)

type Transport string

const (
	TransportTCP  Transport = "tcp"
	TransportQUIC Transport = "quic"
)

// Target is a generic endpoint, not a service verdict. URL is sanitized before
// it enters an ObservationResult so persisted evidence never contains userinfo
// or query values.
type Target struct {
	Name              string    `json:"name,omitempty"`
	URL               string    `json:"url"`
	Hostname          string    `json:"hostname"`
	Port              string    `json:"port"`
	RequestedProtocol Transport `json:"requested_protocol"`
}

type NetworkContext struct {
	Interface           string        `json:"interface,omitempty"`
	LocalAddress        string        `json:"local_address,omitempty"`
	AddressFamily       AddressFamily `json:"address_family,omitempty"`
	DefaultGateway      string        `json:"default_gateway,omitempty"`
	ConfiguredResolvers []string      `json:"configured_resolvers,omitempty"`
	NetworkLabel        string        `json:"network_label,omitempty"`
}

type ResolvedAddress struct {
	IP            string        `json:"ip"`
	AddressFamily AddressFamily `json:"address_family"`
	ResolverOrder int           `json:"resolver_order"`
}

// StageEvidence records a boundary observed during one pinned connection
// attempt. Detail is bounded and never contains request credentials or bodies.
type StageEvidence struct {
	Stage                 Stage             `json:"stage"`
	Status                Status            `json:"status"`
	StartedAt             time.Time         `json:"started_at"`
	ElapsedMS             int64             `json:"elapsed_ms"`
	Class                 Classification    `json:"class,omitempty"`
	Error                 string            `json:"error,omitempty"`
	Detail                string            `json:"detail,omitempty"`
	HelloSentAt           time.Time         `json:"hello_sent_at,omitempty"`
	TLSVersion            string            `json:"tls_version,omitempty"`
	ALPN                  string            `json:"alpn,omitempty"`
	CipherSuite           string            `json:"cipher_suite,omitempty"`
	PeerCertificateSHA256 string            `json:"peer_certificate_sha256,omitempty"`
	ServerName            string            `json:"server_name,omitempty"`
	HTTPProtocol          string            `json:"http_protocol,omitempty"`
	HTTPStatus            int               `json:"http_status,omitempty"`
	PathComplete          bool              `json:"path_complete,omitempty"`
	Redirect              string            `json:"redirect,omitempty"`
	ResponseHeaders       map[string]string `json:"response_headers,omitempty"`
}

type ConnectionAttempt struct {
	ResolvedIP    string          `json:"resolved_ip,omitempty"`
	AddressFamily AddressFamily   `json:"address_family,omitempty"`
	Transport     Transport       `json:"transport"`
	LocalAddress  string          `json:"local_address,omitempty"`
	Stages        []StageEvidence `json:"stages"`
	ElapsedMS     int64           `json:"elapsed_ms"`
}

type ExecutionContext struct {
	Mode        string `json:"mode,omitempty"`
	ProfileName string `json:"profile_name,omitempty"`
	StrategyID  string `json:"strategy_id,omitempty"`
}

// ObservationResult is the versioned, serializable evidence produced by one
// read-only observation. ObservationRun is retained as the semantic name for a
// complete run; both names intentionally refer to the same data shape.
type ObservationResult struct {
	SchemaVersion       int                  `json:"schema_version"`
	RunID               string               `json:"run_id"`
	StartedAt           time.Time            `json:"started_at"`
	FinishedAt          time.Time            `json:"finished_at"`
	BuildIdentity       engine.BuildIdentity `json:"build_identity"`
	Platform            string               `json:"platform"`
	NetworkContext      NetworkContext       `json:"network_context"`
	ExecutionContext    ExecutionContext     `json:"execution_context,omitempty"`
	Target              Target               `json:"target"`
	ResolvedAddresses   []ResolvedAddress    `json:"resolved_addresses,omitempty"`
	Attempts            []ConnectionAttempt  `json:"attempts"`
	PrimaryAttemptIndex *int                 `json:"primary_attempt_index,omitempty"`
	FinalBoundary       Stage                `json:"final_boundary"`
	Classification      Classification       `json:"classification"`
}

type ObservationRun = ObservationResult

type Timeouts struct {
	DNS     time.Duration
	Connect time.Duration
	TLS     time.Duration
	HTTP    time.Duration
	Overall time.Duration
}

func DefaultTimeouts() Timeouts {
	return Timeouts{
		DNS:     5 * time.Second,
		Connect: 5 * time.Second,
		TLS:     8 * time.Second,
		HTTP:    8 * time.Second,
		Overall: 25 * time.Second,
	}
}

func (t Timeouts) normalized() Timeouts {
	defaults := DefaultTimeouts()
	if t.DNS <= 0 {
		t.DNS = defaults.DNS
	}
	if t.Connect <= 0 {
		t.Connect = defaults.Connect
	}
	if t.TLS <= 0 {
		t.TLS = defaults.TLS
	}
	if t.HTTP <= 0 {
		t.HTTP = defaults.HTTP
	}
	if t.Overall <= 0 {
		t.Overall = defaults.Overall
	}
	return t
}
