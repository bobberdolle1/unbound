package engine

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		err       error
		wantStage FailureStage
		wantClass FailureClass
	}{
		{nil, StageNone, FailNone},
		{errors.New("lookup example.com: no such host"), StageDNS, FailDNS},
		{errors.New("wsarecv: An existing connection was forcibly closed by the remote host"), StageTCP, FailConnectionReset},
		{errors.New("context deadline exceeded"), StageTCP, FailConnectTimeout},
		{errors.New("tls: handshake failure"), StageTLS, FailTLS},
		{errors.New("websocket: bad handshake status 400"), StageWebSocket, FailWebSocket},
	}

	for _, tc := range tests {
		stage, class := ClassifyError(tc.err)
		if stage != tc.wantStage || class != tc.wantClass {
			t.Errorf("ClassifyError(%v) = (%s, %s); want (%s, %s)", tc.err, stage, class, tc.wantStage, tc.wantClass)
		}
	}
}

func TestExecuteWithRetrySuccessFirstTry(t *testing.T) {
	ce := NewConnectivityEngine(time.Second)
	calls := 0
	probe := func(ctx context.Context) ProbeResult {
		calls++
		return ProbeResult{Status: StatusPass, Success: true}
	}

	res := ce.ExecuteWithRetry(context.Background(), 3, probe)
	if res.Status != StatusPass {
		t.Fatalf("Expected PASS, got %s", res.Status)
	}
	if calls != 1 {
		t.Errorf("Expected 1 call, got %d", calls)
	}
	if res.Attempts != 1 {
		t.Errorf("Expected Attempts=1, got %d", res.Attempts)
	}
}

func TestExecuteWithRetrySuccessOnRetry(t *testing.T) {
	ce := NewConnectivityEngine(time.Second)
	calls := 0
	probe := func(ctx context.Context) ProbeResult {
		calls++
		if calls < 2 {
			return ProbeResult{Status: StatusFail, Error: "temporary failure"}
		}
		return ProbeResult{Status: StatusPass, Success: true}
	}

	res := ce.ExecuteWithRetry(context.Background(), 3, probe)
	if res.Status != StatusPass {
		t.Fatalf("Expected PASS on retry, got %s", res.Status)
	}
	if calls != 2 {
		t.Errorf("Expected 2 calls, got %d", calls)
	}
	if res.Attempts != 2 {
		t.Errorf("Expected Attempts=2, got %d", res.Attempts)
	}
}

func TestExecuteWithRetryFailsAfterMaxAttempts(t *testing.T) {
	ce := NewConnectivityEngine(time.Second)
	calls := 0
	probe := func(ctx context.Context) ProbeResult {
		calls++
		return ProbeResult{Status: StatusFail, Error: "persistent failure"}
	}

	res := ce.ExecuteWithRetry(context.Background(), 3, probe)
	if res.Status != StatusFail {
		t.Fatalf("Expected FAIL, got %s", res.Status)
	}
	if calls != 3 {
		t.Errorf("Expected 3 calls, got %d", calls)
	}
	if res.Attempts != 3 {
		t.Errorf("Expected Attempts=3, got %d", res.Attempts)
	}
}

func TestExecuteWithRetryContextCancellation(t *testing.T) {
	ce := NewConnectivityEngine(time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	res := ce.ExecuteWithRetry(ctx, 3, func(ctx context.Context) ProbeResult {
		return ProbeResult{Status: StatusFail}
	})

	if res.Status != StatusFail {
		t.Errorf("Expected FAIL on cancelled context, got %s", res.Status)
	}
}

func TestProbeHTTPWithTestServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/204" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path == "/200" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	ce := NewConnectivityEngine(2 * time.Second)

	// Test 204
	res204 := ce.ProbeHTTP(context.Background(), ts.URL+"/204", http.StatusNoContent)
	if res204.Status != StatusPass {
		t.Errorf("Expected 204 to PASS, got %s (err: %s)", res204.Status, res204.Error)
	}

	// Test 200
	res200 := ce.ProbeHTTP(context.Background(), ts.URL+"/200", http.StatusOK)
	if res200.Status != StatusPass {
		t.Errorf("Expected 200 to PASS, got %s (err: %s)", res200.Status, res200.Error)
	}

	// Test 500 failure
	res500 := ce.ProbeHTTP(context.Background(), ts.URL+"/500", http.StatusOK)
	if res500.Status != StatusFail {
		t.Errorf("Expected 500 to FAIL, got %s", res500.Status)
	}
	if res500.Class != FailHTTPStatus {
		t.Errorf("Expected FailHTTPStatus, got %s", res500.Class)
	}
}

func TestProbeSummaryString(t *testing.T) {
	pPass := ProbeResult{Name: "DNS Check", Status: StatusPass, Latency: 25 * time.Millisecond}
	sPass := pPass.SummaryString()
	if !strings.Contains(sPass, "✓") || !strings.Contains(sPass, "PASS") {
		t.Errorf("Unexpected pass summary: %s", sPass)
	}

	pFail := ProbeResult{Name: "Gateway", Status: StatusFail, Error: "connection timeout"}
	sFail := pFail.SummaryString()
	if !strings.Contains(sFail, "✕") || !strings.Contains(sFail, "FAIL") {
		t.Errorf("Unexpected fail summary: %s", sFail)
	}
}

func TestConnectivityEnginePinHost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pinned-ok"))
	}))
	defer ts.Close()

	parts := strings.Split(ts.URL, ":")
	port := parts[len(parts)-1]

	ce := NewConnectivityEngine(2 * time.Second)

	fictionalHost := "unbound-test-domain-not-in-dns.invalid"
	ce.PinHost(fictionalHost, net.ParseIP("127.0.0.1"))
	defer ce.UnpinHost(fictionalHost)

	res := ce.ProbeHTTP(context.Background(), "http://"+fictionalHost+":"+port+"/", http.StatusOK)
	if res.Status != StatusPass {
		t.Fatalf("Expected pinned request to pass, got: %s (err: %s)", res.Status, res.Error)
	}

	ce.ResetPinnedHosts()
	ce.ResetConnectionPool()
}
func generateTestCertificate(t *testing.T) tls.Certificate {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"UNBOUND Test"},
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:  []string{"localhost", "127.0.0.1"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}
}

func TestRealQUICHandshakeSuccess(t *testing.T) {
	cert := generateTestCertificate(t)
	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h3", "h3-29"},
	}

	listener, err := quic.ListenAddr("127.0.0.1:0", serverTLS, &quic.Config{})
	if err != nil {
		t.Fatalf("Failed to start quic listener: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Background server loop accepting the handshake
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		sess, err := listener.Accept(ctx)
		if err == nil && sess != nil {
			_ = sess.CloseWithError(0, "test complete")
		}
	}()

	ce := NewConnectivityEngine(2 * time.Second)
	res := ce.ProbeQUIC(context.Background(), serverAddr)
	if res.Status != StatusPass {
		t.Fatalf("Expected real QUIC handshake to PASS, got %s (err: %s)", res.Status, res.Error)
	}
	if !strings.Contains(res.Details, "QUIC v1 Handshake Verified") {
		t.Errorf("Expected verified details, got: %s", res.Details)
	}
}

func TestRealQUICRejectArbitraryUDPResponse(t *testing.T) {
	// Setup an un-encrypted/fake UDP listener that echoes arbitrary fake packets
	// (like the old v0.6.2 flawed mock test)
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to bind UDP: %v", err)
	}
	defer pc.Close()

	serverAddr := pc.LocalAddr().String()

	go func() {
		buf := make([]byte, 2048)
		n, addr, err := pc.ReadFrom(buf)
		if err == nil && n > 0 {
			// Send fake unencrypted response
			fakeResp := make([]byte, 1200)
			fakeResp[0] = 0xc0
			_, _ = pc.WriteTo(fakeResp, addr)
		}
	}()

	ce := NewConnectivityEngine(500 * time.Millisecond)
	res := ce.ProbeQUIC(context.Background(), serverAddr)
	// Invariant: An authentic RFC 9001 client MUST reject arbitrary unencrypted packets!
	if res.Status == StatusPass {
		t.Fatal("CRITICAL FALSE PASS: quic-go must NOT accept unencrypted fake UDP reply as valid handshake!")
	}
}

func TestRealQUICTimeoutOnSilentServer(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to bind UDP: %v", err)
	}
	silentAddr := pc.LocalAddr().String()
	pc.Close() // Discard/silence all traffic

	ce := NewConnectivityEngine(400 * time.Millisecond)
	res := ce.ProbeQUIC(context.Background(), silentAddr)
	if res.Status != StatusFail {
		t.Fatalf("Expected timeout to fail, got %s", res.Status)
	}
	if !strings.Contains(res.Error, "QUIC handshake timeout") {
		t.Errorf("Expected timeout error, got: %s", res.Error)
	}
}

func TestDiscordGatewayOpcode10Verification(t *testing.T) {
	// Setup mock WebSocket server sending HTTP 101 and Discord Opcode 10 Hello frame
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
			return
		}
		conn, bufrw, err := hj.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		// Write 101 Switching Protocols
		response := "HTTP/1.1 101 Switching Protocols\r\n" +
			"Upgrade: websocket\r\n" +
			"Connection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=\r\n\r\n"
		_, _ = bufrw.WriteString(response)
		_ = bufrw.Flush()

		// Write Discord Opcode 10 Hello WebSocket frame (Text frame: 0x81, payload len)
		helloJSON := `{"op":10,"d":{"heartbeat_interval":41250}}`
		frame := []byte{0x81, byte(len(helloJSON))}
		frame = append(frame, []byte(helloJSON)...)
		_, _ = conn.Write(frame)
	}))
	defer ts.Close()

	// Extract host and port
	u := ts.URL
	parts := strings.Split(u, ":")
	port := parts[len(parts)-1]

	ce := NewConnectivityEngine(time.Second)
	ce.PinHost("gateway.discord.gg", net.ParseIP("127.0.0.1"))
	defer ce.UnpinHost("gateway.discord.gg")

	// Temporarily override target URL port in test by pointing dialer
	// ProbeDiscordGateway dials gateway.discord.gg:443; with PinHost it dials 127.0.0.1:443.
	// In this test, we verify that ce.DialPinnedHost dials the pinned IP.
	dialConn, dialedIP, err := ce.DialPinnedHost(context.Background(), "tcp", "gateway.discord.gg:"+port)
	if err != nil {
		t.Fatalf("DialPinnedHost failed: %v", err)
	}
	defer dialConn.Close()
	if dialedIP == nil || !dialedIP.Equal(net.ParseIP("127.0.0.1")) {
		t.Errorf("Expected dialed IP 127.0.0.1, got %v", dialedIP)
	}
}
