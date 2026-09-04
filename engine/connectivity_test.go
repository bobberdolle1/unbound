package engine

import (
	"context"
	"errors"
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
