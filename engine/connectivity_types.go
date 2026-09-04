package engine

import (
	"fmt"
	"strings"
	"time"
)

// ProbeStatus represents the outcome of a connectivity probe.
type ProbeStatus string

const (
	StatusPass        ProbeStatus = "PASS"
	StatusFail        ProbeStatus = "FAIL"
	StatusWarning     ProbeStatus = "WARNING"
	StatusNotVerified ProbeStatus = "NOT_VERIFIED"
	StatusInfo        ProbeStatus = "INFO"
)

// FailureStage describes at which network layer a failure occurred.
type FailureStage string

const (
	StageNone      FailureStage = "NONE"
	StageDNS       FailureStage = "DNS"
	StageTCP       FailureStage = "TCP"
	StageTLS       FailureStage = "TLS"
	StageHTTP      FailureStage = "HTTP"
	StageWebSocket FailureStage = "WEBSOCKET"
	StageUDP       FailureStage = "UDP"
	StageSystem    FailureStage = "SYSTEM"
)

// FailureClass provides structured classification of failures.
type FailureClass string

const (
	FailNone              FailureClass = "NONE"
	FailDNS               FailureClass = "DNS_FAILURE"
	FailConnectTimeout    FailureClass = "CONNECT_TIMEOUT"
	FailConnectionReset   FailureClass = "CONNECTION_RESET"
	FailTLS               FailureClass = "TLS_FAILURE"
	FailHTTPStatus        FailureClass = "HTTP_STATUS"
	FailWebSocket         FailureClass = "WEBSOCKET_FAILURE"
	FailUDP               FailureClass = "UDP_FAILURE"
	FailProcessNotRunning FailureClass = "PROCESS_NOT_RUNNING"
	FailIntegrity         FailureClass = "INTEGRITY_FAILURE"
	FailMissingFile       FailureClass = "MISSING_FILE"
	FailInvalidProfile    FailureClass = "INVALID_PROFILE"
	FailPrivilege         FailureClass = "PRIVILEGE_FAILURE"
	FailConflictDetected  FailureClass = "CONFLICT_DETECTED"
	FailUpdateIntegrity   FailureClass = "UPDATE_INTEGRITY_FAILURE"
	FailUnknown           FailureClass = "UNKNOWN"
)

// ProbeResult represents the comprehensive result of a connectivity, system, or service probe.
type ProbeResult struct {
	ID            string        `json:"id"`
	Service       string        `json:"service"`       // e.g. "YouTube", "Discord", "Steam", "Network", "System", "Engine", "Autostart"
	Category      string        `json:"category"`      // e.g. "Web", "API", "CDN", "Gateway", "DNS", "Integrity", "Conflicts"
	Name          string        `json:"name"`          // Human readable display name
	Target        string        `json:"target"`        // URL, host, or resource descriptor
	Transport     string        `json:"transport"`     // "HTTPS", "TCP/TLS", "WebSocket", "UDP", "Local", "DNS"
	Status        ProbeStatus   `json:"status"`        // PASS, FAIL, WARNING, NOT_VERIFIED, INFO
	Latency       time.Duration `json:"latency"`
	Stage         FailureStage  `json:"stage,omitempty"`
	Class         FailureClass  `json:"class,omitempty"`
	Error         string        `json:"error,omitempty"`
	Details       string        `json:"details,omitempty"`
	ResolvedIP    string        `json:"resolvedIp,omitempty"`
	HTTPStatus    int           `json:"httpStatus,omitempty"`
	TLSVersion    uint16        `json:"tlsVersion,omitempty"`
	CertValid     bool          `json:"certValid,omitempty"`
	CertIssuer    string        `json:"certIssuer,omitempty"`
	Attempts      int           `json:"attempts"`
	Timestamp     time.Time     `json:"timestamp"`
	IsManualCheck bool          `json:"isManualCheck"`

	// Backwards compatibility with earlier prober.go fields
	URL           string `json:"url,omitempty"`
	Success       bool   `json:"success"`
	ConnectionRST bool   `json:"connectionRst,omitempty"`
}

// SummaryString returns a concise one-line summary of the probe.
func (p ProbeResult) SummaryString() string {
	icon := "✓"
	switch p.Status {
	case StatusPass:
		icon = "✓"
	case StatusFail:
		icon = "✕"
	case StatusWarning:
		icon = "!"
	case StatusNotVerified:
		icon = "?"
	case StatusInfo:
		icon = "ℹ"
	}

	name := p.Name
	if name == "" {
		name = p.Target
	}

	if p.Status == StatusPass {
		if p.Latency > 0 {
			return fmt.Sprintf("%s %s [%s] (%v)", icon, name, p.Status, p.Latency.Round(time.Millisecond))
		}
		return fmt.Sprintf("%s %s [%s]", icon, name, p.Status)
	}

	if p.Details != "" {
		return fmt.Sprintf("%s %s [%s]: %s", icon, name, p.Status, p.Details)
	}
	if p.Error != "" {
		return fmt.Sprintf("%s %s [%s]: %s", icon, name, p.Status, p.Error)
	}
	return fmt.Sprintf("%s %s [%s]", icon, name, p.Status)
}

// ClassifyError inspects a Go error and classifies its failure stage and failure class.
func ClassifyError(err error) (FailureStage, FailureClass) {
	if err == nil {
		return StageNone, FailNone
	}
	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "no such host") || strings.Contains(errStr, "lookup ") || strings.Contains(errStr, "dns"):
		return StageDNS, FailDNS
	case strings.Contains(errStr, "connection reset") || strings.Contains(errStr, "wsarecv") || strings.Contains(errStr, "10054"):
		return StageTCP, FailConnectionReset
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") || strings.Contains(errStr, "timed out"):
		return StageTCP, FailConnectTimeout
	case strings.Contains(errStr, "websocket") || strings.Contains(errStr, "101"):
		return StageWebSocket, FailWebSocket
	case strings.Contains(errStr, "tls") || strings.Contains(errStr, "handshake") || strings.Contains(errStr, "certificate"):
		return StageTLS, FailTLS
	default:
		return StageTCP, FailUnknown
	}
}
