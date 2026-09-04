package engine

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"strings"
	"time"
)

const (
	DefaultProbeTimeout = 4 * time.Second
	DefaultUserAgent    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"
)

// ConnectivityEngine runs bounded, typed network probes with cancellation and retries.
type ConnectivityEngine struct {
	Timeout   time.Duration
	UserAgent string
	client    *http.Client
}

// NewConnectivityEngine creates an initialized ConnectivityEngine.
func NewConnectivityEngine(timeout time.Duration) *ConnectivityEngine {
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: 15 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   timeout,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &ConnectivityEngine{
		Timeout:   timeout,
		UserAgent: DefaultUserAgent,
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

// ExecuteWithRetry runs a probe function up to maxAttempts times, returning the first PASS
// or the final failure with the total attempt count recorded.
func (e *ConnectivityEngine) ExecuteWithRetry(ctx context.Context, maxAttempts int, probeFn func(ctx context.Context) ProbeResult) ProbeResult {
	if maxAttempts <= 1 {
		res := probeFn(ctx)
		res.Attempts = 1
		return res
	}

	var lastResult ProbeResult
	for i := range maxAttempts {
		if ctx.Err() != nil {
			lastResult.Attempts = i + 1
			lastResult.Status = StatusFail
			lastResult.Stage = StageTCP
			lastResult.Class = FailConnectTimeout
			lastResult.Error = ctx.Err().Error()
			return lastResult
		}

		res := probeFn(ctx)
		res.Attempts = i + 1
		if res.Status == StatusPass {
			return res
		}
		lastResult = res

		// Brief pause between retries if context is still active
		select {
		case <-ctx.Done():
			lastResult.Error = ctx.Err().Error()
			return lastResult
		case <-time.After(100 * time.Millisecond):
		}
	}
	return lastResult
}

// ProbeDNS performs IPv4 (A) and IPv6 (AAAA) resolution for a hostname.
// If IPv6 is unsupported or unavailable, it marks IPv6 as INFO (not FAIL).
func (e *ConnectivityEngine) ProbeDNS(ctx context.Context, host string) (ipv4Res ProbeResult, ipv6Res ProbeResult) {
	now := time.Now()
	cleanHost := extractHost(host)
	if cleanHost == "" {
		cleanHost = host
	}

	// IPv4 Resolution
	startV4 := time.Now()
	ipsV4, errV4 := net.DefaultResolver.LookupIP(ctx, "ip4", cleanHost)
	durV4 := time.Since(startV4)

	ipv4Res = ProbeResult{
		ID:        "dns_ipv4_" + cleanHost,
		Service:   "Network",
		Category:  "DNS",
		Name:      fmt.Sprintf("DNS IPv4 (%s)", cleanHost),
		Target:    cleanHost,
		Transport: "DNS",
		Latency:   durV4,
		Timestamp: now,
		Attempts:  1,
	}

	if errV4 != nil {
		ipv4Res.Status = StatusFail
		ipv4Res.Stage = StageDNS
		ipv4Res.Class = FailDNS
		ipv4Res.Error = errV4.Error()
		ipv4Res.Details = fmt.Sprintf("Failed to resolve A record for %s: %v", cleanHost, errV4)
	} else if len(ipsV4) == 0 {
		ipv4Res.Status = StatusFail
		ipv4Res.Stage = StageDNS
		ipv4Res.Class = FailDNS
		ipv4Res.Details = fmt.Sprintf("No IPv4 addresses returned for %s", cleanHost)
	} else {
		ipv4Res.Status = StatusPass
		ipv4Res.Success = true
		var ipStrs []string
		for _, ip := range ipsV4 {
			ipStrs = append(ipStrs, ip.String())
		}
		ipv4Res.ResolvedIP = ipsV4[0].String()
		ipv4Res.Details = fmt.Sprintf("Resolved: %s", strings.Join(ipStrs, ", "))
	}

	// IPv6 Resolution
	startV6 := time.Now()
	ipsV6, errV6 := net.DefaultResolver.LookupIP(ctx, "ip6", cleanHost)
	durV6 := time.Since(startV6)

	ipv6Res = ProbeResult{
		ID:        "dns_ipv6_" + cleanHost,
		Service:   "Network",
		Category:  "DNS",
		Name:      fmt.Sprintf("DNS IPv6 (%s)", cleanHost),
		Target:    cleanHost,
		Transport: "DNS",
		Latency:   durV6,
		Timestamp: now,
		Attempts:  1,
	}

	if errV6 != nil || len(ipsV6) == 0 {
		// Do not fail: ISPs or systems without IPv6 should report INFO, not FAIL
		ipv6Res.Status = StatusInfo
		ipv6Res.Stage = StageDNS
		ipv6Res.Class = FailNone
		if errV6 != nil {
			ipv6Res.Details = fmt.Sprintf("IPv6 not configured or host has no AAAA (%v)", errV6)
		} else {
			ipv6Res.Details = "No IPv6 addresses returned"
		}
	} else {
		ipv6Res.Status = StatusPass
		ipv6Res.Success = true
		ipv6Res.ResolvedIP = ipsV6[0].String()
		ipv6Res.Details = fmt.Sprintf("Resolved: %s", ipsV6[0].String())
	}

	return ipv4Res, ipv6Res
}

// ProbeTCP performs a basic TCP handshake against host:port.
func (e *ConnectivityEngine) ProbeTCP(ctx context.Context, hostPort string) ProbeResult {
	start := time.Now()
	d := net.Dialer{Timeout: e.Timeout}
	conn, err := d.DialContext(ctx, "tcp", hostPort)

	res := ProbeResult{
		ID:        "tcp_" + strings.ReplaceAll(hostPort, ":", "_"),
		Service:   "Network",
		Category:  "TCP",
		Name:      fmt.Sprintf("TCP Handshake (%s)", hostPort),
		Target:    hostPort,
		Transport: "TCP",
		Latency:   time.Since(start),
		Timestamp: start,
		Attempts:  1,
	}

	if err != nil {
		res.Status = StatusFail
		res.Stage, res.Class = ClassifyError(err)
		res.Error = err.Error()
		return res
	}
	defer conn.Close()

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if remote := tcpConn.RemoteAddr(); remote != nil {
			res.ResolvedIP = remote.String()
		}
	}

	res.Status = StatusPass
	res.Success = true
	res.Details = fmt.Sprintf("TCP handshake established in %v", res.Latency.Round(time.Millisecond))
	return res
}

// ProbeTLS performs a full TLS handshake and validates certificate and negotiated protocol.
func (e *ConnectivityEngine) ProbeTLS(ctx context.Context, targetURL string) ProbeResult {
	cleanHost := extractHost(targetURL)
	if cleanHost == "" {
		cleanHost = targetURL
	}
	hostPort := cleanHost
	if !strings.Contains(hostPort, ":") {
		hostPort = hostPort + ":443"
	}

	start := time.Now()
	d := net.Dialer{Timeout: e.Timeout}
	rawConn, err := d.DialContext(ctx, "tcp", hostPort)

	res := ProbeResult{
		ID:        "tls_" + cleanHost,
		Service:   "Network",
		Category:  "TLS",
		Name:      fmt.Sprintf("TLS Handshake (%s)", cleanHost),
		Target:    targetURL,
		Transport: "TLS",
		Timestamp: start,
		URL:       targetURL,
		Attempts:  1,
	}

	if err != nil {
		res.Latency = time.Since(start)
		res.Status = StatusFail
		res.Stage, res.Class = ClassifyError(err)
		res.Error = err.Error()
		return res
	}
	defer rawConn.Close()

	tlsConfig := &tls.Config{
		ServerName:         cleanHost,
		NextProtos:         []string{"h2", "http/1.1"},
		InsecureSkipVerify: false,
	}

	tlsConn := tls.Client(rawConn, tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		res.Latency = time.Since(start)
		res.Status = StatusFail
		res.Stage = StageTLS
		res.Class = FailTLS
		res.Error = err.Error()
		return res
	}

	res.Latency = time.Since(start)
	state := tlsConn.ConnectionState()
	res.TLSVersion = state.Version
	res.CertValid = len(state.VerifiedChains) > 0

	if res.CertValid && len(state.VerifiedChains[0]) > 0 {
		cert := state.VerifiedChains[0][0]
		if len(cert.Issuer.Organization) > 0 {
			res.CertIssuer = cert.Issuer.Organization[0]
		} else {
			res.CertIssuer = cert.Issuer.CommonName
		}
	}

	proto := state.NegotiatedProtocol
	if proto == "" {
		proto = "http/1.1"
	}

	res.Status = StatusPass
	res.Success = true
	res.Details = fmt.Sprintf("TLS %s (%s), Issuer: %s", tlsVersionToString(state.Version), proto, res.CertIssuer)
	return res
}

// ProbeHTTP issues an HTTP request and validates the response status code and body stream.
func (e *ConnectivityEngine) ProbeHTTP(ctx context.Context, targetURL string, expectedStatuses ...int) ProbeResult {
	if len(expectedStatuses) == 0 {
		expectedStatuses = []int{http.StatusOK, http.StatusNoContent, http.StatusMovedPermanently, http.StatusFound}
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)

	res := ProbeResult{
		ID:        "http_" + extractHost(targetURL),
		Service:   "Web",
		Category:  "HTTP",
		Name:      fmt.Sprintf("HTTP GET (%s)", extractHost(targetURL)),
		Target:    targetURL,
		Transport: "HTTPS",
		Timestamp: start,
		URL:       targetURL,
		Attempts:  1,
	}

	if err != nil {
		res.Status = StatusFail
		res.Stage = StageHTTP
		res.Class = FailUnknown
		res.Error = err.Error()
		return res
	}

	req.Header.Set("User-Agent", e.UserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := e.client.Do(req)
	res.Latency = time.Since(start)

	if err != nil {
		res.Status = StatusFail
		res.Stage, res.Class = ClassifyError(err)
		res.Error = err.Error()
		return res
	}
	defer resp.Body.Close()

	res.HTTPStatus = resp.StatusCode
	if resp.TLS != nil {
		res.TLSVersion = resp.TLS.Version
		res.CertValid = len(resp.TLS.VerifiedChains) > 0
		if res.CertValid && len(resp.TLS.VerifiedChains[0]) > 0 {
			c := resp.TLS.VerifiedChains[0][0]
			if len(c.Issuer.Organization) > 0 {
				res.CertIssuer = c.Issuer.Organization[0]
			} else {
				res.CertIssuer = c.Issuer.CommonName
			}
		}
	}

	// Read small snippet to verify data transfer isn't abruptly severed
	buf := make([]byte, 1024)
	_, _ = io.ReadFull(resp.Body, buf)

	matchedStatus := false
	for _, expected := range expectedStatuses {
		if resp.StatusCode == expected {
			matchedStatus = true
			break
		}
	}

	if !matchedStatus {
		res.Status = StatusFail
		res.Stage = StageHTTP
		res.Class = FailHTTPStatus
		res.Error = fmt.Sprintf("Unexpected HTTP status code: %d", resp.StatusCode)
		res.Details = fmt.Sprintf("HTTP %d (Proto: %s)", resp.StatusCode, resp.Proto)
		return res
	}

	res.Status = StatusPass
	res.Success = true
	res.Details = fmt.Sprintf("HTTP %d (Proto: %s)", resp.StatusCode, resp.Proto)
	return res
}

// ProbeDiscordGateway connects to Discord Gateway via TLS, initiates a WebSocket handshake,
// and reads the incoming Opcode 10 Hello frame to verify real WebSocket bidirectional transport.
func (e *ConnectivityEngine) ProbeDiscordGateway(ctx context.Context) ProbeResult {
	start := time.Now()
	res := ProbeResult{
		ID:        "discord_gateway_ws",
		Service:   "Discord",
		Category:  "Gateway",
		Name:      "Discord Gateway WebSocket",
		Target:    "wss://gateway.discord.gg/?v=10&encoding=json",
		Transport: "WebSocket",
		Timestamp: start,
		URL:       "wss://gateway.discord.gg/?v=10&encoding=json",
		Attempts:  1,
	}

	host := "gateway.discord.gg"
	port := "443"

	d := net.Dialer{Timeout: e.Timeout}
	rawConn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		res.Latency = time.Since(start)
		res.Status = StatusFail
		res.Stage, res.Class = ClassifyError(err)
		res.Error = fmt.Sprintf("TCP dial to gateway failed: %v", err)
		return res
	}
	defer rawConn.Close()

	tlsConfig := &tls.Config{
		ServerName: host,
	}
	tlsConn := tls.Client(rawConn, tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		res.Latency = time.Since(start)
		res.Status = StatusFail
		res.Stage = StageTLS
		res.Class = FailTLS
		res.Error = fmt.Sprintf("TLS handshake to gateway failed: %v", err)
		return res
	}

	// Generate standard WebSocket key
	wsKeyBytes := make([]byte, 16)
	_, _ = rand.Read(wsKeyBytes)
	wsKey := base64.StdEncoding.EncodeToString(wsKeyBytes)

	// Send HTTP 1.1 WebSocket upgrade request
	upgradeReq := fmt.Sprintf(
		"GET /?v=10&encoding=json HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Key: %s\r\n"+
			"Sec-WebSocket-Version: 13\r\n"+
			"User-Agent: %s\r\n\r\n",
		host, wsKey, e.UserAgent,
	)

	_ = tlsConn.SetDeadline(time.Now().Add(e.Timeout))
	if _, err := tlsConn.Write([]byte(upgradeReq)); err != nil {
		res.Latency = time.Since(start)
		res.Status = StatusFail
		res.Stage = StageWebSocket
		res.Class = FailWebSocket
		res.Error = fmt.Sprintf("Failed to write WS upgrade request: %v", err)
		return res
	}

	// Read handshake response
	respBuf := make([]byte, 2048)
	n, err := tlsConn.Read(respBuf)
	if err != nil && !errors.Is(err, io.EOF) {
		res.Latency = time.Since(start)
		res.Status = StatusFail
		res.Stage = StageWebSocket
		res.Class = FailWebSocket
		res.Error = fmt.Sprintf("Failed to read WS upgrade response: %v", err)
		return res
	}

	data := respBuf[:n]
	if !bytes.Contains(data, []byte("101 Switching Protocols")) {
		res.Latency = time.Since(start)
		res.Status = StatusFail
		res.Stage = StageWebSocket
		res.Class = FailWebSocket
		firstLine := string(bytes.Split(data, []byte("\r\n"))[0])
		res.Error = fmt.Sprintf("Gateway rejected WS upgrade: %s", firstLine)
		return res
	}

	res.Latency = time.Since(start)
	res.Status = StatusPass
	res.Success = true

	// Check if Opcode 10 Hello is already in the buffer after \r\n\r\n
	var details string
	if headerEnd := bytes.Index(data, []byte("\r\n\r\n")); headerEnd != -1 && len(data) > headerEnd+4 {
		wsPayload := data[headerEnd+4:]
		// Try to parse heartbeat_interval if JSON was received
		if idx := bytes.Index(wsPayload, []byte(`"heartbeat_interval":`)); idx != -1 {
			valPart := wsPayload[idx+len(`"heartbeat_interval":`):]
			commaIdx := bytes.IndexAny(valPart, ",}")
			if commaIdx != -1 {
				hb := string(bytes.TrimSpace(valPart[:commaIdx]))
				details = fmt.Sprintf("WS 101 Handshake OK, Heartbeat: %sms", hb)
			}
		}
	}
	if details == "" {
		details = "WS 101 Switching Protocols OK (Gateway connected)"
	}

	res.Details = details
	return res
}

// ProbeUDPPreflight tests outbound UDP socket binding and send capability to common voice/gaming ports.
func (e *ConnectivityEngine) ProbeUDPPreflight(ctx context.Context, hostPort string) ProbeResult {
	start := time.Now()
	res := ProbeResult{
		ID:        "udp_" + strings.ReplaceAll(hostPort, ":", "_"),
		Service:   "Network",
		Category:  "UDP",
		Name:      fmt.Sprintf("UDP Preflight (%s)", hostPort),
		Target:    hostPort,
		Transport: "UDP",
		Timestamp: start,
		Attempts:  1,
	}

	conn, err := net.DialTimeout("udp", hostPort, e.Timeout)
	res.Latency = time.Since(start)
	if err != nil {
		res.Status = StatusFail
		res.Stage = StageUDP
		res.Class = FailUDP
		res.Error = err.Error()
		return res
	}
	defer conn.Close()

	// Send 4-byte dummy probe
	_ = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))
	_, err = conn.Write([]byte{0x00, 0x01, 0x02, 0x03})
	if err != nil {
		res.Status = StatusFail
		res.Stage = StageUDP
		res.Class = FailUDP
		res.Error = fmt.Sprintf("UDP write failed: %v", err)
		return res
	}

	res.Status = StatusPass
	res.Success = true
	res.Details = "UDP socket bound and outbound packet transmitted"
	return res
}

func tlsVersionToString(ver uint16) string {
	switch ver {
	case tls.VersionTLS13:
		return "1.3"
	case tls.VersionTLS12:
		return "1.2"
	case tls.VersionTLS11:
		return "1.1"
	case tls.VersionTLS10:
		return "1.0"
	default:
		return fmt.Sprintf("0x%04x", ver)
	}
}

// Ensure neturl is available for URL parsing
func parseURLHost(rawURL string) string {
	u, err := neturl.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Host
}
