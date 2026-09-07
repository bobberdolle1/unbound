package engine

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestProbeQUICHonestSemantics(t *testing.T) {
	// 1. Test against responsive mock QUIC UDP server
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to bind UDP: %v", err)
	}
	defer pc.Close()

	serverAddr := pc.LocalAddr().String()

	go func() {
		buf := make([]byte, 2048)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		// Verify incoming packet is at least 1200 bytes as required by RFC 9000
		if n >= 1200 && buf[0] == 0xc0 {
			// Send a valid mock QUIC response
			resp := make([]byte, 1200)
			resp[0] = 0xc0 // Long Header Initial
			resp[1] = 0x00
			resp[2] = 0x00
			resp[3] = 0x01
			_, _ = pc.WriteTo(resp, addr)
		}
	}()

	ce := NewConnectivityEngine(500 * time.Millisecond)
	passRes := ce.ProbeQUIC(context.Background(), serverAddr)
	if passRes.Status != StatusPass {
		t.Errorf("Expected QUIC probe to PASS on responsive server, got %s (err: %s)", passRes.Status, passRes.Error)
	}
	if !strings.Contains(passRes.Details, "verified") {
		t.Errorf("Expected verified details, got: %s", passRes.Details)
	}

	// 2. Test timeout on silent UDP port (no server reply)
	silentPc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to bind silent UDP: %v", err)
	}
	silentAddr := silentPc.LocalAddr().String()
	silentPc.Close() // closed immediately so it drops/silences packets

	failRes := ce.ProbeQUIC(context.Background(), silentAddr)
	if failRes.Status != StatusFail {
		t.Errorf("Expected QUIC probe to FAIL on silent server, got %s", failRes.Status)
	}
	if !strings.Contains(failRes.Error, "QUIC handshake response timeout") {
		t.Errorf("Expected timeout error message, got: %s", failRes.Error)
	}
}
