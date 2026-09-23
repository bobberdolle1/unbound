package observatory

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strconv"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestDirectObserverSuccessAndStageProgression(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	server.StartTLS()
	defer server.Close()

	result := observeFixture(t, server.URL, serverTLSConfig(server), nil)
	if result.Classification != ClassSuccess || result.FinalBoundary != StageCarry {
		t.Fatalf("result = %s at %s, want SUCCESS at carry", result.Classification, result.FinalBoundary)
	}
	attempt := onlyAttempt(t, result)
	assertStages(t, attempt, []Stage{StageResolve, StageConnect, StageHello, StageHandshake, StageHTTP, StageCarry})
	if stage(t, attempt, StageHTTP).HTTPStatus != http.StatusNoContent {
		t.Fatal("HTTP status was not recorded")
	}
	if stage(t, attempt, StageHandshake).PeerCertificateSHA256 == "" {
		t.Fatal("leaf certificate fingerprint missing")
	}
	if result.BuildIdentity.OS == "" || result.BuildIdentity.Arch == "" {
		t.Fatal("build identity missing platform")
	}
}

func TestDirectObserverClosedPortKeepsLaterStagesNotReached(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()

	result := observeFixture(t, "https://localhost:"+portFromAddress(t, address), nil, nil)
	attempt := onlyAttempt(t, result)
	connect := stage(t, attempt, StageConnect)
	if connect.Class != ClassTCPConnectionRefused {
		t.Fatalf("connect class = %s (%s), want refused", connect.Class, connect.Detail)
	}
	for _, next := range []Stage{StageHello, StageHandshake, StageHTTP} {
		if got := stage(t, attempt, next).Status; got != StatusNotReached {
			t.Fatalf("%s status = %s, want NOT_REACHED", next, got)
		}
	}
}

func TestDirectObserverTLSTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			accepted <- connection
		}
	}()

	result := observeFixture(t, "https://localhost:"+portFromAddress(t, listener.Addr().String()), nil, func(options *Options) {
		options.Timeouts = Timeouts{Connect: time.Second, TLS: 80 * time.Millisecond, HTTP: time.Second, Overall: time.Second}
	})
	select {
	case connection := <-accepted:
		connection.Close()
	default:
	}
	attempt := onlyAttempt(t, result)
	if got := stage(t, attempt, StageHandshake).Class; got != ClassTLSHandshakeTimeout {
		t.Fatalf("TLS class = %s, want timeout", got)
	}
	if got := stage(t, attempt, StageHTTP).Status; got != StatusNotReached {
		t.Fatalf("HTTP = %s, want NOT_REACHED", got)
	}
}

func TestDirectObserverTLSReset(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		_ = connection.SetReadDeadline(time.Now().Add(time.Second))
		_, _ = connection.Read(make([]byte, 1))
		if tcp, ok := connection.(*net.TCPConn); ok {
			_ = tcp.SetLinger(0)
		}
		_ = connection.Close()
	}()

	result := observeFixture(t, "https://localhost:"+portFromAddress(t, listener.Addr().String()), nil, nil)
	attempt := onlyAttempt(t, result)
	got := stage(t, attempt, StageHandshake)
	if got.Status == StatusPass || got.Class == "" {
		t.Fatalf("unexpected TLS reset result: %#v", got)
	}
	if stage(t, attempt, StageHTTP).Status != StatusNotReached {
		t.Fatal("HTTP must remain NOT_REACHED after TLS reset")
	}
}

func TestDirectObserverHTTPStatus(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	server.StartTLS()
	defer server.Close()
	result := observeFixture(t, server.URL, serverTLSConfig(server), nil)
	attempt := onlyAttempt(t, result)
	httpStage := stage(t, attempt, StageHTTP)
	if httpStage.Class != ClassHTTPStatus || httpStage.HTTPStatus != http.StatusServiceUnavailable {
		t.Fatalf("HTTP evidence = %#v", httpStage)
	}
	if result.Classification != ClassHTTPStatus {
		t.Fatalf("classification = %s, want HTTP_STATUS", result.Classification)
	}
}

func TestDirectObserverCertificateFailure(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { writer.WriteHeader(http.StatusNoContent) }))
	server.StartTLS()
	defer server.Close()
	result := observeFixture(t, server.URL, nil, func(options *Options) {
		options.Timeouts.TLS = 5 * time.Second
		options.Timeouts.Overall = 8 * time.Second
	})
	if got := stage(t, onlyAttempt(t, result), StageHandshake); got.Class != ClassTLSCertificateFailure {
		t.Fatalf("TLS class = %s (%s), want certificate failure", got.Class, got.Detail)
	}
}

func TestDirectObserverCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := NewDirectTCPHTTPSObserver().Observe(ctx, "https://localhost:443", Options{ResolvedIP: net.ParseIP("127.0.0.1"), Timeouts: DefaultTimeouts()})
	if err != nil {
		t.Fatal(err)
	}
	if got := stage(t, onlyAttempt(t, result), StageConnect).Status; got != StatusCancelled {
		t.Fatalf("connect status = %s, want CANCELLED", got)
	}
}

func TestPinnedEndpointPreservesHostnameAndIP(t *testing.T) {
	var receivedHost string
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedHost = request.Host
		writer.WriteHeader(http.StatusNoContent)
	}))
	server.StartTLS()
	defer server.Close()
	port := portFromAddress(t, server.Listener.Addr().String())
	result := observeFixture(t, "https://pinned.example:"+port+"/generate_204?secret=discard", serverTLSConfig(server), nil)
	attempt := onlyAttempt(t, result)
	if attempt.ResolvedIP != "127.0.0.1" || result.Target.Hostname != "pinned.example" {
		t.Fatalf("endpoint was not pinned: %#v %#v", attempt, result.Target)
	}
	if receivedHost != "pinned.example" {
		t.Fatalf("HTTP Host = %q, want original hostname", receivedHost)
	}
	if result.Target.URL != "https://pinned.example:"+port+"/generate_204" {
		t.Fatalf("stored URL was not redacted: %q", result.Target.URL)
	}
}

func TestQUICIsExplicitlyUnsupported(t *testing.T) {
	result, err := NewDirectTCPHTTPSObserver().Observe(context.Background(), "https://localhost:443", Options{Transport: TransportQUIC, ResolvedIP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	if result.Classification != ClassQUICUnsupported || stage(t, onlyAttempt(t, result), StageConnect).Status != StatusSkippedUnsupported {
		t.Fatalf("QUIC result = %#v", result)
	}
}

func TestProgressCallbacksAreSerialized(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { writer.WriteHeader(http.StatusNoContent) }))
	server.StartTLS()
	defer server.Close()
	var active, overlap int32
	result := observeFixture(t, server.URL, serverTLSConfig(server), func(options *Options) {
		options.Progress = func(StageEvidence) {
			if atomic.AddInt32(&active, 1) != 1 {
				atomic.StoreInt32(&overlap, 1)
			}
			time.Sleep(time.Millisecond)
			atomic.AddInt32(&active, -1)
		}
	})
	if result.Classification != ClassSuccess || atomic.LoadInt32(&overlap) != 0 {
		t.Fatal("progress callbacks overlapped")
	}
}

func TestSchemaRoundTripAndSensitivePersistenceSanitization(t *testing.T) {
	result := ObservationResult{SchemaVersion: SchemaVersion, RunID: "run", Target: Target{URL: "https://user:password@example.test/path?token=secret"}, Attempts: []ConnectionAttempt{{Stages: []StageEvidence{{Stage: StageHTTP, ResponseHeaders: map[string]string{"authorization": "secret", "content-type": "text/plain"}, Detail: "ok"}}}}}
	clean := sanitizeForPersistence(result)
	if clean.Target.URL != "https://example.test/path" {
		t.Fatalf("sanitized URL = %q", clean.Target.URL)
	}
	headers := clean.Attempts[0].Stages[0].ResponseHeaders
	if len(headers) != 1 || headers["content-type"] != "text/plain" {
		t.Fatalf("sensitive headers persisted: %#v", headers)
	}
	data, err := json.Marshal(clean)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ObservationResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != SchemaVersion {
		t.Fatalf("schema = %d", decoded.SchemaVersion)
	}
}

func TestErrorNormalization(t *testing.T) {
	if _, class, _ := normalizeConnectError(context.DeadlineExceeded); class != ClassTCPConnectTimeout {
		t.Fatal(class)
	}
	if _, class, _ := normalizeConnectError(syscall.ECONNREFUSED); class != ClassTCPConnectionRefused {
		t.Fatal(class)
	}
	if _, class, _ := normalizeConnectError(syscall.ECONNRESET); class != ClassTCPConnectionReset {
		t.Fatal(class)
	}
	if _, class, _ := normalizeTLSError(x509.UnknownAuthorityError{}); class != ClassTLSCertificateFailure {
		t.Fatal(class)
	}
	if _, class, _ := normalizeHTTPError(errors.New("opaque")); class != ClassHTTPProtocolFailure {
		t.Fatal(class)
	}
}

type fixtureResolver struct {
	addresses []netip.Addr
	err       error
}

func (resolver fixtureResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return resolver.addresses, resolver.err
}

func TestResolutionOrderIsDeterministic(t *testing.T) {
	addresses, evidence := resolveAddresses(context.Background(), "example.test", Options{Resolver: fixtureResolver{addresses: []netip.Addr{netip.MustParseAddr("2001:db8::1"), netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("192.0.2.1")}}}, time.Second)
	if evidence.Status != StatusPass {
		t.Fatal(evidence)
	}
	got := []string{addresses[0].String(), addresses[1].String(), addresses[2].String()}
	want := []string{"192.0.2.1", "192.0.2.2", "2001:db8::1"}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func observeFixture(t *testing.T, rawURL string, config *tls.Config, mutate func(*Options)) ObservationResult {
	t.Helper()
	options := Options{ResolvedIP: net.ParseIP("127.0.0.1"), Timeouts: Timeouts{DNS: time.Second, Connect: time.Second, TLS: time.Second, HTTP: time.Second, Overall: 3 * time.Second}, tlsConfig: config}
	if mutate != nil {
		mutate(&options)
	}
	result, err := NewDirectTCPHTTPSObserver().Observe(context.Background(), rawURL, options)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func serverTLSConfig(server *httptest.Server) *tls.Config {
	return &tls.Config{InsecureSkipVerify: true}
} //nolint:gosec -- hermetic local fixture only
func onlyAttempt(t *testing.T, result ObservationResult) ConnectionAttempt {
	t.Helper()
	if len(result.Attempts) != 1 {
		t.Fatalf("attempts = %d", len(result.Attempts))
	}
	return result.Attempts[0]
}
func stage(t *testing.T, attempt ConnectionAttempt, wanted Stage) StageEvidence {
	t.Helper()
	for _, evidence := range attempt.Stages {
		if evidence.Stage == wanted {
			return evidence
		}
	}
	t.Fatalf("missing stage %s", wanted)
	return StageEvidence{}
}
func assertStages(t *testing.T, attempt ConnectionAttempt, wanted []Stage) {
	t.Helper()
	if len(attempt.Stages) != len(wanted) {
		t.Fatalf("stage count = %d, want %d", len(attempt.Stages), len(wanted))
	}
	for index, expected := range wanted {
		if attempt.Stages[index].Stage != expected {
			t.Fatalf("stage %d = %s, want %s", index, attempt.Stages[index].Stage, expected)
		}
	}
}
func portFromAddress(t *testing.T, address string) string {
	t.Helper()
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := strconv.Atoi(port); err != nil {
		t.Fatal(err)
	}
	return port
}
func parseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
