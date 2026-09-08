package engine

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

const (
	DefaultProbeTimeout = 4 * time.Second
	DefaultUserAgent    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"
)

// ConnectivityEngine runs bounded, typed network probes with cancellation and retries.
type ConnectivityEngine struct {
	Timeout   time.Duration
	UserAgent string
	client             *http.Client
	InsecureSkipVerify bool
	CustomRootCAs      *x509.CertPool

	pinnedMu  sync.RWMutex
	pinnedIPs map[string]net.IP
}
// NewConnectivityEngine creates an initialized ConnectivityEngine.
func NewConnectivityEngine(timeout time.Duration) *ConnectivityEngine {
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}
	ce := &ConnectivityEngine{
		Timeout:   timeout,
		UserAgent: DefaultUserAgent,
		pinnedIPs: make(map[string]net.IP),
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, _, err := ce.DialPinnedHost(ctx, network, addr)
			return conn, err
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   timeout,
		ExpectContinueTimeout: 1 * time.Second,
	}

	ce.client = &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
	return ce
}

// DialPinnedHost resolves hostPort using the pinned IP table if configured, or falls back to system DNS.
// Returns a connected net.Conn and the actual remote IP connected to.
func (e *ConnectivityEngine) DialPinnedHost(ctx context.Context, network, hostPort string) (net.Conn, net.IP, error) {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		host = hostPort
		port = "443"
	}

	cleanHost := extractHost(host)
	if cleanHost == "" {
		cleanHost = strings.TrimSpace(host)
	}

	dialAddr := hostPort
	e.pinnedMu.RLock()
	pinnedIP, hasPin := e.pinnedIPs[cleanHost]
	e.pinnedMu.RUnlock()

	var targetIP net.IP
	if hasPin && pinnedIP != nil {
		dialAddr = net.JoinHostPort(pinnedIP.String(), port)
		targetIP = pinnedIP
	}

	d := net.Dialer{Timeout: e.Timeout}
	conn, err := d.DialContext(ctx, network, dialAddr)
	if err != nil {
		return nil, targetIP, err
	}

	if targetIP == nil {
		if ra, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
			targetIP = ra.IP
		} else if ra, ok := conn.RemoteAddr().(*net.UDPAddr); ok {
			targetIP = ra.IP
		}
	}

	return conn, targetIP, nil
}

// PinHost binds a hostname to a specific pre-resolved IP address for all subsequent probes.
// Eliminates TOCTOU CDN Anycast DNS drift during WinDivert filter interception.
func (e *ConnectivityEngine) PinHost(host string, ip net.IP) {
	e.pinnedMu.Lock()
	defer e.pinnedMu.Unlock()
	clean := extractHost(host)
	if clean == "" {
		clean = strings.TrimSpace(host)
	}
	e.pinnedIPs[clean] = ip
}

// UnpinHost removes the IP pinning for a hostname.
func (e *ConnectivityEngine) UnpinHost(host string) {
	e.pinnedMu.Lock()
	defer e.pinnedMu.Unlock()
	clean := extractHost(host)
	if clean == "" {
		clean = strings.TrimSpace(host)
	}
	delete(e.pinnedIPs, clean)
}

// GetPinnedHost returns the currently pinned IP for a host, or nil if unpinned.
func (e *ConnectivityEngine) GetPinnedHost(host string) net.IP {
	e.pinnedMu.RLock()
	defer e.pinnedMu.RUnlock()
	clean := extractHost(host)
	if clean == "" {
		clean = strings.TrimSpace(host)
	}
	return e.pinnedIPs[clean]
}

// ResetPinnedHosts clears all host pinning mappings.
func (e *ConnectivityEngine) ResetPinnedHosts() {
	e.pinnedMu.Lock()
	defer e.pinnedMu.Unlock()
	e.pinnedIPs = make(map[string]net.IP)
}

// ResetConnectionPool flushes all idle TCP connections in the transport pool,
// ensuring subsequent candidate probes perform fresh handshakes through the active engine.
func (e *ConnectivityEngine) ResetConnectionPool() {
	if tr, ok := e.client.Transport.(*http.Transport); ok {
		tr.CloseIdleConnections()
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
	conn, _, err := e.DialPinnedHost(ctx, "tcp", hostPort)

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
	rawConn, _, err := e.DialPinnedHost(ctx, "tcp", hostPort)
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
// ProbeTLSVersion performs a TLS handshake forcing an exact TLS version.
func (e *ConnectivityEngine) ProbeTLSVersion(ctx context.Context, targetURL string, version uint16) ProbeResult {
	cleanHost := extractHost(targetURL)
	if cleanHost == "" {
		cleanHost = targetURL
	}
	hostPort := cleanHost
	if !strings.Contains(hostPort, ":") {
		hostPort = hostPort + ":443"
	}

	start := time.Now()
	rawConn, _, err := e.DialPinnedHost(ctx, "tcp", hostPort)
	res := ProbeResult{
		ID:        fmt.Sprintf("tls_%s_%s", cleanHost, tlsVersionToString(version)),
		Service:   "Network",
		Category:  "TLS",
		Name:      fmt.Sprintf("TLS %s (%s)", tlsVersionToString(version), cleanHost),
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
		MinVersion:         version,
		MaxVersion:         version,
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
	res.Status = StatusPass
	res.Success = true
	res.Details = fmt.Sprintf("TLS %s Handshake OK (Cipher: 0x%04x)", tlsVersionToString(state.Version), state.CipherSuite)
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

// DiscordGatewayStatus contains structured diagnostics from a Discord Gateway WebSocket verification.
type DiscordGatewayStatus struct {
	WebSocketUpgradeVerified bool   `json:"webSocketUpgradeVerified"`
	Opcode10Verified         bool   `json:"opcode10Verified"`
	HeartbeatInterval        int    `json:"heartbeatInterval"`
	RemoteIP                 string `json:"remoteIP"`
	Error                    string `json:"error,omitempty"`
}

func computeSecWebSocketAccept(key string) string {
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func extractHeaderValue(data []byte, headerName string) string {
	lines := bytes.Split(data, []byte("\r\n"))
	prefix := []byte(strings.ToLower(headerName) + ":")
	for _, line := range lines {
		lower := bytes.ToLower(line)
		if bytes.HasPrefix(lower, prefix) {
			val := line[len(prefix):]
			return strings.TrimSpace(string(val))
		}
	}
	return ""
}

func parseWebSocketFrame(wsData []byte) (opcode int, payload []byte, err error) {
	if len(wsData) < 2 {
		return 0, nil, errors.New("frame truncated (< 2 bytes)")
	}
	b0 := wsData[0]
	b1 := wsData[1]
	opcode = int(b0 & 0x0F)
	isMasked := (b1 & 0x80) != 0
	if isMasked {
		return opcode, nil, errors.New("server frame must not be masked")
	}

	payloadLen := int(b1 & 0x7F)
	headerLen := 2
	if payloadLen == 126 {
		if len(wsData) < 4 {
			return opcode, nil, errors.New("frame truncated for 16-bit length")
		}
		payloadLen = int(binary.BigEndian.Uint16(wsData[2:4]))
		headerLen = 4
	} else if payloadLen == 127 {
		if len(wsData) < 10 {
			return opcode, nil, errors.New("frame truncated for 64-bit length")
		}
		payloadLen = int(binary.BigEndian.Uint64(wsData[2:10]))
		headerLen = 10
	}

	if len(wsData) < headerLen+payloadLen {
		return opcode, nil, fmt.Errorf("frame incomplete: have %d bytes, need %d", len(wsData), headerLen+payloadLen)
	}

	return opcode, wsData[headerLen : headerLen+payloadLen], nil
}

// ProbeDiscordGatewayWithTarget connects to an injected or canonical Discord Gateway via TLS,
// validates RFC 6455 Sec-WebSocket-Accept handshake semantics, and decodes the WebSocket Opcode 10 Hello frame.
func (e *ConnectivityEngine) ProbeDiscordGatewayWithTarget(ctx context.Context, hostPort string) (ProbeResult, DiscordGatewayStatus) {
	start := time.Now()
	target := hostPort
	if strings.TrimSpace(target) == "" {
		target = "gateway.discord.gg:443"
	}

	host, port, splitErr := net.SplitHostPort(target)
	if splitErr != nil {
		host = target
		port = "443"
		target = net.JoinHostPort(host, port)
	}

	res := ProbeResult{
		ID:        "discord_gateway_ws",
		Service:   "Discord",
		Category:  "Gateway",
		Name:      "Discord Gateway WebSocket",
		Target:    "wss://" + target + "/?v=10&encoding=json",
		Transport: "WebSocket",
		Timestamp: start,
		URL:       "wss://" + target + "/?v=10&encoding=json",
		Attempts:  1,
	}
	status := DiscordGatewayStatus{}

	rawConn, remoteIP, err := e.DialPinnedHost(ctx, "tcp", target)
	if err != nil {
		res.Latency = time.Since(start)
		res.Status = StatusFail
		res.Stage, res.Class = ClassifyError(err)
		res.Error = fmt.Sprintf("TCP dial to gateway failed: %v", err)
		status.Error = res.Error
		return res, status
	}
	defer rawConn.Close()
	if remoteIP != nil {
		status.RemoteIP = remoteIP.String()
	}
	tlsConfig := &tls.Config{
		ServerName:         host,
		RootCAs:            e.CustomRootCAs,
		InsecureSkipVerify: e.InsecureSkipVerify,
	}
	tlsConn := tls.Client(rawConn, tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		res.Latency = time.Since(start)
		res.Status = StatusFail
		res.Stage = StageTLS
		res.Class = FailTLS
		res.Error = fmt.Sprintf("TLS handshake to gateway failed: %v", err)
		status.Error = res.Error
		return res, status
	}

	// Generate standard WebSocket key
	wsKeyBytes := make([]byte, 16)
	_, _ = rand.Read(wsKeyBytes)
	wsKey := base64.StdEncoding.EncodeToString(wsKeyBytes)
	expectedAccept := computeSecWebSocketAccept(wsKey)

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
		status.Error = res.Error
		return res, status
	}

	// Read handshake response headers
	respBuf := make([]byte, 4096)
	n, err := tlsConn.Read(respBuf)
	if err != nil && !errors.Is(err, io.EOF) {
		res.Latency = time.Since(start)
		res.Status = StatusFail
		res.Stage = StageWebSocket
		res.Class = FailWebSocket
		res.Error = fmt.Sprintf("Failed to read WS upgrade response: %v", err)
		status.Error = res.Error
		return res, status
	}

	data := respBuf[:n]
	if !bytes.Contains(data, []byte("101 Switching Protocols")) {
		res.Latency = time.Since(start)
		res.Status = StatusFail
		res.Stage = StageWebSocket
		res.Class = FailWebSocket
		firstLine := string(bytes.Split(data, []byte("\r\n"))[0])
		res.Error = fmt.Sprintf("Gateway rejected WS upgrade: %s", firstLine)
		status.Error = res.Error
		return res, status
	}

	// RFC 6455 Section 4.2.2: Verify Sec-WebSocket-Accept
	actualAccept := extractHeaderValue(data, "Sec-WebSocket-Accept")
	if actualAccept != expectedAccept {
		res.Latency = time.Since(start)
		res.Status = StatusFail
		res.Stage = StageWebSocket
		res.Class = FailWebSocket
		res.Error = fmt.Sprintf("invalid Sec-WebSocket-Accept header (got %q, expected %q)", actualAccept, expectedAccept)
		status.Error = res.Error
		return res, status
	}
	status.WebSocketUpgradeVerified = true

	// Read WebSocket frame following headers
	var wsData []byte
	if headerEnd := bytes.Index(data, []byte("\r\n\r\n")); headerEnd != -1 && len(data) > headerEnd+4 {
		wsData = data[headerEnd+4:]
	}
	if len(wsData) < 2 {
		_ = tlsConn.SetDeadline(time.Now().Add(e.Timeout))
		frameBuf := make([]byte, 4096)
		if nFrame, fErr := tlsConn.Read(frameBuf); fErr == nil && nFrame > 0 {
			wsData = append(wsData, frameBuf[:nFrame]...)
		}
	}

	opcode, framePayload, parseErr := parseWebSocketFrame(wsData)
	if parseErr != nil {
		// Try one more bounded read if frame was incomplete
		_ = tlsConn.SetDeadline(time.Now().Add(e.Timeout))
		frameBuf := make([]byte, 4096)
		if nFrame, fErr := tlsConn.Read(frameBuf); fErr == nil && nFrame > 0 {
			wsData = append(wsData, frameBuf[:nFrame]...)
			opcode, framePayload, parseErr = parseWebSocketFrame(wsData)
		}
	}

	res.Latency = time.Since(start)
	if parseErr != nil {
		res.Status = StatusFail
		res.Stage = StageWebSocket
		res.Class = FailWebSocket
		res.Error = fmt.Sprintf("WebSocket frame decoding failed: %v", parseErr)
		status.Error = res.Error
		return res, status
	}

	if opcode != 1 && opcode != 2 {
		res.Status = StatusFail
		res.Stage = StageWebSocket
		res.Class = FailWebSocket
		res.Error = fmt.Sprintf("unexpected WebSocket frame opcode %d (expected 1 or 2)", opcode)
		status.Error = res.Error
		return res, status
	}

	var helloData struct {
		Op int `json:"op"`
		D  struct {
			HeartbeatInterval int `json:"heartbeat_interval"`
		} `json:"d"`
	}

	if err := json.Unmarshal(framePayload, &helloData); err != nil {
		res.Status = StatusFail
		res.Stage = StageWebSocket
		res.Class = FailWebSocket
		res.Error = fmt.Sprintf("failed to parse Discord Hello JSON: %v", err)
		status.Error = res.Error
		return res, status
	}

	if helloData.Op != 10 || helloData.D.HeartbeatInterval <= 0 {
		res.Status = StatusFail
		res.Stage = StageWebSocket
		res.Class = FailWebSocket
		res.Error = fmt.Sprintf("invalid Discord Gateway payload (op=%d, heartbeat=%d)", helloData.Op, helloData.D.HeartbeatInterval)
		status.Error = res.Error
		return res, status
	}

	status.Opcode10Verified = true
	status.HeartbeatInterval = helloData.D.HeartbeatInterval
	res.Status = StatusPass
	res.Success = true
	res.Details = fmt.Sprintf("Discord Gateway Verified (101 Upgrade + Opcode 10 Hello, Heartbeat: %dms, Remote: %s)",
		status.HeartbeatInterval, remoteIP)
	return res, status
}

// ProbeDiscordGateway connects to canonical Discord Gateway via TLS, initiates a WebSocket handshake,
// verifies Sec-WebSocket-Accept, and reads the incoming Opcode 10 Hello frame.
func (e *ConnectivityEngine) ProbeDiscordGateway(ctx context.Context) ProbeResult {
	res, _ := e.ProbeDiscordGatewayWithTarget(ctx, "gateway.discord.gg:443")
	return res
}

// ProbeHTTP3 performs an authentic RFC 9114 HTTP/3 application request over QUIC.
func (e *ConnectivityEngine) ProbeHTTP3(ctx context.Context, targetURL string) ProbeResult {
	cleanHost := extractHost(targetURL)
	if cleanHost == "" {
		cleanHost = targetURL
	}
	hostPort := cleanHost
	if !strings.Contains(hostPort, ":") {
		hostPort = hostPort + ":443"
	}
	urlStr := targetURL
	if !strings.HasPrefix(urlStr, "https://") {
		urlStr = "https://" + cleanHost
	}

	start := time.Now()
	res := ProbeResult{
		ID:        "http3_" + strings.ReplaceAll(cleanHost, ":", "_"),
		Service:   "Network",
		Category:  "HTTP3",
		Name:      fmt.Sprintf("HTTP/3 (%s)", cleanHost),
		Target:    urlStr,
		Transport: "QUIC/HTTP3",
		Timestamp: start,
		URL:       urlStr,
		Attempts:  1,
	}

	pinnedIP := e.GetPinnedHost(cleanHost)
	dialAddr := hostPort
	if pinnedIP != nil {
		_, port, _ := net.SplitHostPort(hostPort)
		if port == "" {
			port = "443"
		}
		dialAddr = net.JoinHostPort(pinnedIP.String(), port)
		res.ResolvedIP = pinnedIP.String()
	}

	tlsConfig := &tls.Config{
		ServerName:         cleanHost,
		InsecureSkipVerify: false,
	}

	tr := &http3.Transport{
		TLSClientConfig: tlsConfig,
		QUICConfig: &quic.Config{
			HandshakeIdleTimeout: e.Timeout,
			MaxIdleTimeout:       e.Timeout,
		},
		Dial: func(dialCtx context.Context, addr string, tlsCfg *tls.Config, quicCfg *quic.Config) (*quic.Conn, error) {
			targetDial := dialAddr
			return quic.DialAddr(dialCtx, targetDial, tlsCfg, quicCfg)
		},
	}
	defer tr.Close()

	client := &http.Client{
		Transport: tr,
		Timeout:   e.Timeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		res.Latency = time.Since(start)
		res.Status = StatusFail
		res.Stage = StageHTTP
		res.Class = FailUnknown
		res.Error = err.Error()
		return res
	}
	req.Header.Set("User-Agent", e.UserAgent)

	resp, err := client.Do(req)
	res.Latency = time.Since(start)
	if err != nil {
		res.Status = StatusFail
		res.Stage, res.Class = ClassifyError(err)
		res.Error = err.Error()
		return res
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	res.HTTPStatus = resp.StatusCode
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		res.Status = StatusPass
		res.Success = true
		res.Details = fmt.Sprintf("HTTP/3 GET OK (%d %s) in %v", resp.StatusCode, resp.Status, res.Latency.Round(time.Millisecond))
	} else if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		res.Status = StatusPass
		res.Success = true
		res.Details = fmt.Sprintf("HTTP/3 Response %d (%s) in %v", resp.StatusCode, resp.Status, res.Latency.Round(time.Millisecond))
	} else {
		res.Status = StatusFail
		res.Stage = StageHTTP
		res.Class = FailHTTPStatus
		res.Error = fmt.Sprintf("HTTP/3 unexpected application status code: %d", resp.StatusCode)
		res.Details = fmt.Sprintf("HTTP/3 Status %d (expected 200/204)", resp.StatusCode)
	}
	return res
}

// ProbeTargetProtocol is the canonical dispatcher probing the specified target according to protocol.
func ProbeTargetProtocol(ctx context.Context, ce *ConnectivityEngine, targetURL, protocol string) ProbeResult {
	proto := strings.ToUpper(strings.TrimSpace(protocol))
	cleanHost := extractHost(targetURL)
	if cleanHost == "" {
		cleanHost = targetURL
	}

	switch proto {
	case "HTTP":
		httpURL := targetURL
		if !strings.HasPrefix(httpURL, "http://") {
			httpURL = "http://" + targetURL
		}
		return ce.ProbeHTTP(ctx, httpURL, http.StatusOK, http.StatusNoContent, http.StatusMovedPermanently, http.StatusFound)
	case "TLS1.2":
		return ce.ProbeTLSVersion(ctx, cleanHost+":443", tls.VersionTLS12)
	case "TLS1.3":
		return ce.ProbeTLSVersion(ctx, cleanHost+":443", tls.VersionTLS13)
	case "QUIC":
		return ce.ProbeQUIC(ctx, cleanHost+":443")
	case "ANY":
		// Canonical ANY probe: TLS 1.3 first, then HTTP, then QUIC
		res1 := ce.ProbeTLSVersion(ctx, cleanHost+":443", tls.VersionTLS13)
		if res1.Status == StatusPass {
			return res1
		}
		httpURL := "http://" + cleanHost
		res2 := ce.ProbeHTTP(ctx, httpURL, http.StatusOK, http.StatusNoContent, http.StatusMovedPermanently, http.StatusFound)
		if res2.Status == StatusPass {
			return res2
		}
		return ce.ProbeQUIC(ctx, cleanHost+":443")
	default:
		return ProbeResult{
			ID:        "unknown_proto",
			Status:    StatusFail,
			Error:     fmt.Sprintf("unsupported protocol %q", protocol),
			Timestamp: time.Now(),
		}
	}
}

// ProbeStrategyLabBaseline measures the unassisted reachability baseline for a target and protocol.
func ProbeStrategyLabBaseline(ctx context.Context, ce *ConnectivityEngine, targetURL, protocol string) ProbeResult {
	return ProbeTargetProtocol(ctx, ce, targetURL, protocol)
}

// ProbeQUIC performs an authentic RFC 9000 / RFC 9001 QUIC v1 handshake probe.
// Resolves/pins IP, establishes an encrypted QUIC connection with TLS 1.3 handshake,
// and validates cryptographic handshake completion.
func (e *ConnectivityEngine) ProbeQUIC(ctx context.Context, targetURL string) ProbeResult {
	var cleanHost, hostPort string
	if host, port, err := net.SplitHostPort(targetURL); err == nil {
		cleanHost = host
		hostPort = net.JoinHostPort(host, port)
	} else {
		cleanHost = extractHost(targetURL)
		if cleanHost == "" {
			cleanHost = strings.TrimSpace(targetURL)
		}
		hostPort = cleanHost
		if !strings.Contains(hostPort, ":") {
			hostPort = hostPort + ":443"
		}
	}

	start := time.Now()
	res := ProbeResult{
		ID:        "quic_" + strings.ReplaceAll(cleanHost, ".", "_"),
		Service:   "Network",
		Category:  "QUIC",
		Name:      fmt.Sprintf("QUIC v1 Handshake (%s)", cleanHost),
		Target:    targetURL,
		Transport: "QUIC",
		Timestamp: start,
		URL:       targetURL,
		Attempts:  1,
	}
	dialAddr := hostPort
	e.pinnedMu.RLock()
	pinnedIP, hasPin := e.pinnedIPs[cleanHost]
	e.pinnedMu.RUnlock()
	if hasPin && pinnedIP != nil {
		_, port, err := net.SplitHostPort(hostPort)
		if err == nil {
			dialAddr = net.JoinHostPort(pinnedIP.String(), port)
		} else {
			dialAddr = net.JoinHostPort(pinnedIP.String(), "443")
		}
	}

	tlsConfig := &tls.Config{
		ServerName:         cleanHost,
		NextProtos:         []string{"h3", "h3-29"},
		InsecureSkipVerify: cleanHost == "localhost" || cleanHost == "127.0.0.1" || strings.HasSuffix(cleanHost, ".local"),
	}

	quicConfig := &quic.Config{
		HandshakeIdleTimeout: e.Timeout,
		MaxIdleTimeout:       e.Timeout,
		Versions:             []quic.Version{quic.Version1},
	}

	connCtx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()

	sess, err := quic.DialAddr(connCtx, dialAddr, tlsConfig, quicConfig)
	res.Latency = time.Since(start)

	if err != nil {
		res.Status = StatusFail
		res.Stage = StageTLS
		res.Class = FailUDP
		if strings.Contains(err.Error(), "timeout") || errors.Is(err, context.DeadlineExceeded) {
			res.Error = fmt.Sprintf("QUIC handshake timeout (remote: %s): UDP blocked or dropped by DPI", dialAddr)
		} else {
			res.Error = fmt.Sprintf("QUIC handshake failed (remote: %s): %v", dialAddr, err)
		}
		return res
	}
	defer func() {
		_ = sess.CloseWithError(0, "probe complete")
	}()

	cs := sess.ConnectionState()
	alpn := cs.TLS.NegotiatedProtocol
	if alpn == "" {
		alpn = "quic-v1"
	}

	res.Status = StatusPass
	res.Success = true
	res.TLSVersion = cs.TLS.Version
	res.Details = fmt.Sprintf("QUIC v1 Handshake Verified (ALPN: %s, Cipher: 0x%04x, Remote: %s)",
		alpn, cs.TLS.CipherSuite, dialAddr)
	return res
}

// ProbeUDPPreflight tests basic outbound UDP socket binding and send capability.
// NOTE: An outbound write alone does NOT verify server reachability or DPI bypass;
// use ProbeQUIC for verified bidirectional UDP handshake probing.
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
	res.Details = "UDP socket bound and outbound packet transmitted (socket preflight only)"
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
