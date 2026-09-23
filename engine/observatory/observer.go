package observatory

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"unbound/engine"
)

// Resolver is intentionally transport-neutral. Observatory has no dependency
// on concrete bypass providers or their process lifecycle.
type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type Options struct {
	Timeouts      Timeouts
	AddressFamily AddressFamily
	ResolvedIP    net.IP
	Transport     Transport
	NetworkLabel  string
	Resolver      Resolver
	// tlsConfig is an unexported hermetic-fixture seam. Production callers
	// always use the secure default configuration below.
	tlsConfig *tls.Config
	// Progress is synchronous and serialized per Observe call. Callers must not
	// block indefinitely; the observer itself never invokes callbacks concurrently.
	Progress func(StageEvidence)
}

type DirectTCPHTTPSObserver struct{}

func NewDirectTCPHTTPSObserver() *DirectTCPHTTPSObserver { return &DirectTCPHTTPSObserver{} }

func (o *DirectTCPHTTPSObserver) Observe(ctx context.Context, rawURL string, options Options) (ObservationResult, error) {
	started := time.Now().UTC()
	timeouts := options.Timeouts.normalized()
	if options.Transport == "" {
		options.Transport = TransportTCP
	}
	buildIdentity := engine.CurrentBuildIdentity()
	result := ObservationResult{
		SchemaVersion:    SchemaVersion,
		RunID:            newRunID(),
		StartedAt:        started,
		BuildIdentity:    buildIdentity,
		Platform:         buildIdentity.OS + "/" + buildIdentity.Arch,
		NetworkContext:   NetworkContext{NetworkLabel: boundedDetail(options.NetworkLabel)},
		Attempts:         []ConnectionAttempt{},
		Classification:   ClassUnknown,
		ExecutionContext: ExecutionContext{Mode: "direct"},
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		result.FinishedAt = time.Now().UTC()
		result.Target = Target{URL: sanitizeURL(parsed), RequestedProtocol: options.Transport}
		return result, fmt.Errorf("observatory requires an absolute https URL")
	}
	port := parsed.Port()
	if port == "" {
		port = "443"
	}
	result.Target = Target{
		URL:               sanitizeURL(parsed),
		Hostname:          parsed.Hostname(),
		Port:              port,
		RequestedProtocol: options.Transport,
	}

	overallCtx, cancel := context.WithTimeout(ctx, timeouts.Overall)
	defer cancel()
	emitter := &serializedEmitter{callback: options.Progress}

	if options.Transport == TransportQUIC {
		result = o.observeQUICUnsupported(overallCtx, result, parsed, options, emitter)
		result.FinishedAt = time.Now().UTC()
		return result, nil
	}
	if options.Transport != TransportTCP {
		result.FinishedAt = time.Now().UTC()
		return result, fmt.Errorf("unsupported transport %q", options.Transport)
	}

	addresses, resolveEvidence := resolveAddresses(overallCtx, parsed.Hostname(), options, timeouts.DNS)
	if resolveEvidence.Status != StatusPass {
		attempt := ConnectionAttempt{Transport: TransportTCP, Stages: []StageEvidence{resolveEvidence}}
		attempt.Stages = append(attempt.Stages, notReachedStages(resolveEvidence.StartedAt, StageConnect, StageHello, StageHandshake, StageHTTP)...)
		attempt.Stages = append(attempt.Stages, carryEvidence(resolveEvidence.StartedAt))
		attempt.ElapsedMS = elapsedMS(resolveEvidence.StartedAt)
		result.Attempts = append(result.Attempts, attempt)
		primary := 0
		result.PrimaryAttemptIndex = &primary
		result.FinalBoundary = StageResolve
		result.Classification = resolveEvidence.Class
		result.FinishedAt = time.Now().UTC()
		emitter.emit(resolveEvidence)
		return result, nil
	}
	result.ResolvedAddresses = toResolvedAddresses(addresses)
	for _, ip := range addresses {
		attempt := o.observeTCPAttempt(overallCtx, parsed, port, ip, resolveEvidence, timeouts, options.tlsConfig, emitter)
		result.Attempts = append(result.Attempts, attempt)
		if result.NetworkContext.LocalAddress == "" && attempt.LocalAddress != "" {
			result.NetworkContext.LocalAddress = attempt.LocalAddress
			result.NetworkContext.AddressFamily = attempt.AddressFamily
		}
	}
	summary := summarizeAttempts(result.Attempts)
	result.PrimaryAttemptIndex = &summary.attemptIndex
	result.FinalBoundary = summary.boundary
	result.Classification = summary.classification
	result.FinishedAt = time.Now().UTC()
	return result, nil
}

func resolveAddresses(ctx context.Context, hostname string, options Options, timeout time.Duration) ([]net.IP, StageEvidence) {
	started := time.Now().UTC()
	if options.ResolvedIP != nil {
		ip := append(net.IP(nil), options.ResolvedIP...)
		if !acceptFamily(ip, options.AddressFamily) {
			return nil, failureEvidence(StageResolve, started, StatusFail, ClassDNSFailure, "address_family_filtered", "explicit address does not match requested family")
		}
		return []net.IP{ip}, passEvidence(StageResolve, started)
	}
	resolver := options.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	stageCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	addresses, err := resolver.LookupNetIP(stageCtx, "ip", hostname)
	if err != nil {
		return nil, resolutionFailure(started, err)
	}
	selected := make([]net.IP, 0, len(addresses))
	for _, address := range addresses {
		ip := net.IP(address.AsSlice())
		if acceptFamily(ip, options.AddressFamily) {
			selected = append(selected, append(net.IP(nil), ip...))
		}
	}
	if len(selected) == 0 {
		return nil, failureEvidence(StageResolve, started, StatusFail, ClassDNSFailure, "no_usable_addresses", "resolver returned no address matching requested family")
	}
	return selected, passEvidence(StageResolve, started)
}

func (o *DirectTCPHTTPSObserver) observeTCPAttempt(ctx context.Context, target *url.URL, port string, ip net.IP, resolve StageEvidence, timeouts Timeouts, fixtureTLSConfig *tls.Config, emitter *serializedEmitter) ConnectionAttempt {
	started := time.Now().UTC()
	attempt := ConnectionAttempt{ResolvedIP: ip.String(), AddressFamily: familyForIP(ip), Transport: TransportTCP, Stages: []StageEvidence{resolve}}
	emitter.emit(resolve)

	connectStarted := time.Now().UTC()
	dialCtx, cancelDial := context.WithTimeout(ctx, timeouts.Connect)
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", net.JoinHostPort(ip.String(), port))
	cancelDial()
	if err != nil {
		connect := connectFailure(connectStarted, err)
		attempt.Stages = append(attempt.Stages, connect)
		emitter.emit(connect)
		attempt.Stages = append(attempt.Stages, notReachedStages(time.Now().UTC(), StageHello, StageHandshake, StageHTTP)...)
		attempt.Stages = append(attempt.Stages, carryEvidence(time.Now().UTC()))
		attempt.ElapsedMS = elapsedMS(started)
		return attempt
	}
	defer conn.Close()
	attempt.LocalAddress = conn.LocalAddr().String()
	connect := passEvidence(StageConnect, connectStarted)
	attempt.Stages = append(attempt.Stages, connect)
	emitter.emit(connect)

	tracked := &writeTrackingConn{Conn: conn}
	tlsConfig := &tls.Config{ServerName: target.Hostname(), NextProtos: []string{"http/1.1"}, MinVersion: tls.VersionTLS12}
	if fixtureTLSConfig != nil {
		tlsConfig = fixtureTLSConfig.Clone()
		tlsConfig.ServerName = target.Hostname()
		tlsConfig.NextProtos = []string{"http/1.1"}
	}
	tlsConn := tls.Client(tracked, tlsConfig)
	handshakeStarted := time.Now().UTC()
	tlsCtx, cancelTLS := context.WithTimeout(ctx, timeouts.TLS)
	err = tlsConn.HandshakeContext(tlsCtx)
	cancelTLS()
	helloSentAt, helloWritten := tracked.firstWriteAt()
	hello := helloEvidence(handshakeStarted, helloSentAt, helloWritten)
	attempt.Stages = append(attempt.Stages, hello)
	emitter.emit(hello)
	if err != nil {
		handshake := tlsFailure(handshakeStarted, err)
		attempt.Stages = append(attempt.Stages, handshake)
		emitter.emit(handshake)
		attempt.Stages = append(attempt.Stages, notReachedStages(time.Now().UTC(), StageHTTP)...)
		attempt.Stages = append(attempt.Stages, carryEvidence(time.Now().UTC()))
		attempt.ElapsedMS = elapsedMS(started)
		return attempt
	}
	handshake := handshakeSuccess(handshakeStarted, tlsConn.ConnectionState(), target.Hostname())
	attempt.Stages = append(attempt.Stages, handshake)
	emitter.emit(handshake)

	httpEvidence := observeHTTP(ctx, tlsConn, target, timeouts.HTTP)
	attempt.Stages = append(attempt.Stages, httpEvidence)
	emitter.emit(httpEvidence)
	attempt.Stages = append(attempt.Stages, carryEvidence(time.Now().UTC()))
	attempt.ElapsedMS = elapsedMS(started)
	return attempt
}

func observeHTTP(ctx context.Context, conn *tls.Conn, target *url.URL, timeout time.Duration) StageEvidence {
	started := time.Now().UTC()
	request := &http.Request{Method: http.MethodGet, URL: target, Host: target.Host, Header: make(http.Header), Close: true}
	request.Header.Set("User-Agent", engine.UserAgent())
	request.Header.Set("Accept", "*/*")
	deadline := time.Now().Add(timeout)
	_ = conn.SetDeadline(deadline)
	cancelClose := closeOnCancel(ctx, conn)
	defer cancelClose()
	if err := request.Write(conn); err != nil {
		return httpFailure(started, err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		return httpFailure(started, err)
	}
	defer response.Body.Close()
	class := Classification("")
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		class = ClassHTTPStatus
	}
	evidence := StageEvidence{Stage: StageHTTP, Status: StatusPass, StartedAt: started, ElapsedMS: elapsedMS(started), Class: class, HTTPProtocol: response.Proto, HTTPStatus: response.StatusCode, PathComplete: true, ResponseHeaders: allowedHeaders(response.Header)}
	if location := response.Header.Get("Location"); location != "" {
		evidence.Redirect = boundedDetail(location)
	}
	return evidence
}

func (o *DirectTCPHTTPSObserver) observeQUICUnsupported(ctx context.Context, result ObservationResult, target *url.URL, options Options, emitter *serializedEmitter) ObservationResult {
	addresses, resolve := resolveAddresses(ctx, target.Hostname(), options, options.Timeouts.normalized().DNS)
	result.ResolvedAddresses = toResolvedAddresses(addresses)
	attempt := ConnectionAttempt{Transport: TransportQUIC, Stages: []StageEvidence{resolve}}
	emitter.emit(resolve)
	if resolve.Status == StatusPass && len(addresses) > 0 {
		attempt.ResolvedIP, attempt.AddressFamily = addresses[0].String(), familyForIP(addresses[0])
	}
	unsupported := failureEvidence(StageConnect, time.Now().UTC(), StatusSkippedUnsupported, ClassQUICUnsupported, "quic_not_implemented", "QUIC observation requires a real QUIC handshake implementation")
	attempt.Stages = append(attempt.Stages, unsupported)
	emitter.emit(unsupported)
	attempt.Stages = append(attempt.Stages, unsupportedStages(time.Now().UTC(), StageHello, StageHandshake, StageHTTP)...)
	attempt.Stages = append(attempt.Stages, carryEvidence(time.Now().UTC()))
	result.Attempts = []ConnectionAttempt{attempt}
	primary := 0
	result.PrimaryAttemptIndex = &primary
	result.FinalBoundary = StageConnect
	result.Classification = ClassQUICUnsupported
	return result
}

type attemptSummary struct {
	attemptIndex   int
	boundary       Stage
	classification Classification
	pathComplete   bool
	targetSuccess  bool
}

// summarizeAttempts chooses the deepest factual boundary. Ties use the first
// attempt in resolver-returned order, which PrimaryAttemptIndex makes explicit.
func summarizeAttempts(attempts []ConnectionAttempt) attemptSummary {
	best := attemptSummary{attemptIndex: 0, boundary: StageResolve, classification: ClassUnknown}
	for index, attempt := range attempts {
		candidate := summarizeAttempt(index, attempt)
		if attemptTargetSuccessful(attempt) {
			candidate.boundary = StageCarry
			candidate.classification = ClassSuccess
			candidate.targetSuccess = true
			return candidate
		}
		if stageDepth(candidate.boundary) > stageDepth(best.boundary) {
			best = candidate
		}
	}
	return best
}

func summarizeAttempt(index int, attempt ConnectionAttempt) attemptSummary {
	summary := attemptSummary{attemptIndex: index, boundary: StageResolve, classification: ClassUnknown}
	for _, evidence := range attempt.Stages {
		if evidence.Stage == StageHTTP && evidence.PathComplete {
			summary.boundary = StageHTTP
			summary.pathComplete = true
			if evidence.Class == ClassHTTPStatus {
				summary.classification = ClassHTTPStatus
			} else {
				summary.classification = ClassSuccess
			}
			continue
		}
		if isTerminalFailure(evidence) && stageDepth(evidence.Stage) >= stageDepth(summary.boundary) {
			summary.boundary = evidence.Stage
			summary.classification = evidence.Class
		}
	}
	return summary
}

func attemptPathComplete(attempt ConnectionAttempt) bool {
	for _, evidence := range attempt.Stages {
		if evidence.Stage == StageHTTP && evidence.PathComplete {
			return true
		}
	}
	return false
}

func attemptTargetSuccessful(attempt ConnectionAttempt) bool {
	for _, evidence := range attempt.Stages {
		if evidence.Stage == StageHTTP && evidence.PathComplete {
			return evidence.Class != ClassHTTPStatus
		}
	}
	return false
}

func isTerminalFailure(evidence StageEvidence) bool {
	return evidence.Class != "" && (evidence.Status == StatusFail || evidence.Status == StatusTimeout || evidence.Status == StatusReset || evidence.Status == StatusCancelled || evidence.Status == StatusSkippedUnsupported)
}

func stageDepth(stage Stage) int {
	switch stage {
	case StageResolve:
		return 1
	case StageConnect:
		return 2
	case StageHello:
		return 3
	case StageHandshake:
		return 4
	case StageHTTP:
		return 5
	default:
		return 0
	}
}

func toResolvedAddresses(addresses []net.IP) []ResolvedAddress {
	result := make([]ResolvedAddress, 0, len(addresses))
	for index, ip := range addresses {
		result = append(result, ResolvedAddress{IP: ip.String(), AddressFamily: familyForIP(ip), ResolverOrder: index})
	}
	return result
}

func acceptFamily(ip net.IP, family AddressFamily) bool {
	return family == "" || family == AddressFamilyAny || (family == AddressFamilyIPv4 && ip.To4() != nil) || (family == AddressFamilyIPv6 && ip.To4() == nil)
}
func familyForIP(ip net.IP) AddressFamily {
	if ip.To4() != nil {
		return AddressFamilyIPv4
	}
	return AddressFamilyIPv6
}
func elapsedMS(started time.Time) int64 { return time.Since(started).Milliseconds() }

func sanitizeURL(value *url.URL) string {
	if value == nil {
		return ""
	}
	copy := *value
	copy.User = nil
	copy.RawQuery = ""
	copy.ForceQuery = false
	return copy.String()
}

func allowedHeaders(headers http.Header) map[string]string {
	allowed := map[string]string{}
	for _, key := range []string{"Content-Type", "Location"} {
		if value := headers.Get(key); value != "" {
			allowed[strings.ToLower(key)] = boundedDetail(value)
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

func closeOnCancel(ctx context.Context, closer io.Closer) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = closer.Close()
		case <-done:
		}
	}()
	return func() { close(done) }
}

type serializedEmitter struct {
	mu       sync.Mutex
	callback func(StageEvidence)
}

func (e *serializedEmitter) emit(evidence StageEvidence) {
	if e.callback == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.callback(evidence)
}

type writeTrackingConn struct {
	net.Conn
	mu         sync.Mutex
	writes     int
	firstWrite time.Time
}

func (c *writeTrackingConn) Write(data []byte) (int, error) {
	n, err := c.Conn.Write(data)
	if n > 0 {
		c.mu.Lock()
		c.writes++
		if c.firstWrite.IsZero() {
			c.firstWrite = time.Now().UTC()
		}
		c.mu.Unlock()
	}
	return n, err
}

func (c *writeTrackingConn) firstWriteAt() (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.firstWrite, !c.firstWrite.IsZero()
}

func helloEvidence(started time.Time, sentAt time.Time, wrote bool) StageEvidence {
	if wrote {
		evidence := passEvidence(StageHello, started)
		evidence.HelloSentAt = &sentAt
		return evidence
	}
	return failureEvidence(StageHello, started, StatusNotReached, ClassUnknown, "client_hello_not_emitted", "TLS handshake ended before ClientHello bytes were emitted")
}
func passEvidence(stage Stage, started time.Time) StageEvidence {
	return StageEvidence{Stage: stage, Status: StatusPass, StartedAt: started, ElapsedMS: elapsedMS(started)}
}
func carryEvidence(started time.Time) StageEvidence {
	return StageEvidence{Stage: StageCarry, Status: StatusPass, StartedAt: started, ElapsedMS: elapsedMS(started), Detail: "evidence preserved without additional network activity"}
}
func notReachedStages(started time.Time, stages ...Stage) []StageEvidence {
	result := make([]StageEvidence, 0, len(stages))
	for _, stage := range stages {
		result = append(result, StageEvidence{Stage: stage, Status: StatusNotReached, StartedAt: started, ElapsedMS: 0})
	}
	return result
}
func unsupportedStages(started time.Time, stages ...Stage) []StageEvidence {
	result := make([]StageEvidence, 0, len(stages))
	for _, stage := range stages {
		result = append(result, StageEvidence{Stage: stage, Status: StatusSkippedUnsupported, StartedAt: started, ElapsedMS: 0, Class: ClassQUICUnsupported})
	}
	return result
}

func handshakeSuccess(started time.Time, state tls.ConnectionState, serverName string) StageEvidence {
	evidence := passEvidence(StageHandshake, started)
	evidence.TLSVersion = tlsVersionName(state.Version)
	evidence.ALPN = state.NegotiatedProtocol
	evidence.CipherSuite = tls.CipherSuiteName(state.CipherSuite)
	evidence.ServerName = serverName
	if len(state.PeerCertificates) > 0 {
		sum := sha256.Sum256(state.PeerCertificates[0].Raw)
		evidence.PeerCertificateSHA256 = hex.EncodeToString(sum[:])
	}
	return evidence
}

func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS13:
		return "TLS1.3"
	case tls.VersionTLS12:
		return "TLS1.2"
	case tls.VersionTLS11:
		return "TLS1.1"
	case tls.VersionTLS10:
		return "TLS1.0"
	default:
		return fmt.Sprintf("0x%04x", version)
	}
}

func resolutionFailure(started time.Time, err error) StageEvidence {
	if errors.Is(err, context.Canceled) {
		return failureEvidence(StageResolve, started, StatusCancelled, ClassUnknown, "context_cancelled", err.Error())
	}
	if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
		return failureEvidence(StageResolve, started, StatusTimeout, ClassDNSFailure, "dns_timeout", err.Error())
	}
	return failureEvidence(StageResolve, started, StatusFail, ClassDNSFailure, "dns_error", err.Error())
}
func connectFailure(started time.Time, err error) StageEvidence {
	status, class, code := normalizeConnectError(err)
	return failureEvidence(StageConnect, started, status, class, code, err.Error())
}
func tlsFailure(started time.Time, err error) StageEvidence {
	status, class, code := normalizeTLSError(err)
	return failureEvidence(StageHandshake, started, status, class, code, err.Error())
}
func httpFailure(started time.Time, err error) StageEvidence {
	status, class, code := normalizeHTTPError(err)
	return failureEvidence(StageHTTP, started, status, class, code, err.Error())
}
func failureEvidence(stage Stage, started time.Time, status Status, class Classification, code, detail string) StageEvidence {
	return StageEvidence{Stage: stage, Status: status, StartedAt: started, ElapsedMS: elapsedMS(started), Class: class, Error: code, Detail: boundedDetail(detail)}
}

func boundedDetail(value string) string {
	value = strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' {
			return -1
		}
		return r
	}, value)
	if len(value) > 512 {
		return value[:512]
	}
	return value
}

func isCertificateError(err error) bool {
	var unknown x509.UnknownAuthorityError
	var invalid x509.CertificateInvalidError
	var hostname x509.HostnameError
	return errors.As(err, &unknown) || errors.As(err, &invalid) || errors.As(err, &hostname)
}
