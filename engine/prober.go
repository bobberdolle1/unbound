package engine

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	neturl "net/url"
	"strings"
	"time"
)


func ProbeConnection(ctx context.Context, targetURL string) (ProbeResult, error) {
	result := ProbeResult{
		URL:     targetURL,
		Success: false,
	}

	host := extractHost(targetURL)
	if host == "" {
		return result, fmt.Errorf("invalid URL: %s", targetURL)
	}

	startTime := time.Now()

	tlsConfig := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}

	tlsDialer := &tls.Dialer{
		NetDialer: &net.Dialer{
			Timeout: 10 * time.Second,
		},
		Config: tlsConfig,
	}

	conn, err := tlsDialer.DialContext(ctx, "tcp4", host+":443")
	if err != nil {
		result.Error = err.Error()
		if strings.Contains(err.Error(), "connection reset") || strings.Contains(err.Error(), "ECONNRESET") {
			result.ConnectionRST = true
		}
		return result, err
	}
	defer conn.Close()

	result.Latency = time.Since(startTime)

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return result, fmt.Errorf("not a TLS connection")
	}

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		result.Error = fmt.Sprintf("TLS handshake failed: %v", err)
		return result, err
	}
	state := tlsConn.ConnectionState()
	if len(state.VerifiedChains) == 0 || len(state.VerifiedChains[0]) == 0 {
		result.Error = "TLS handshake returned no verified certificate chain"
		return result, fmt.Errorf("%s", result.Error)
	}
	cert := state.VerifiedChains[0][0]
	if len(cert.Issuer.Organization) > 0 {
		result.CertIssuer = cert.Issuer.Organization[0]
	} else {
		result.CertIssuer = cert.Issuer.CommonName
	}
	result.CertValid = true
	result.TLSVersion = state.Version
	result.Success = true
	return result, nil
}

func ProbeMultipleTargets(ctx context.Context, targets []string) []ProbeResult {
	results := make([]ProbeResult, 0, len(targets))

	for _, target := range targets {
		select {
		case <-ctx.Done():
			return results
		default:
			result, _ := ProbeConnection(ctx, target)
			results = append(results, result)
		}
	}

	return results
}

func CalculateProbeScore(results []ProbeResult) int {
	score := 0
	for _, r := range results {
		if r.Success && r.CertValid {
			points := 100

			// Give massive priority to YouTube/GoogleVideo to ensure it unblocks video
			if r.URL == "https://youtube.com" || r.URL == "https://googlevideo.com" {
				points = 500
			}

			score += points

			if r.Latency < 100*time.Millisecond {
				score += 20
			} else if r.Latency < 300*time.Millisecond {
				score += 10
			}
		}
	}
	return score
}

func SimplePing(ctx context.Context, targetURL string) (time.Duration, error) {
	host := extractHost(targetURL)
	if host == "" {
		return 0, fmt.Errorf("invalid URL: %s", targetURL)
	}

	startTime := time.Now()

	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}

	tlsConfig := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", host+":443", tlsConfig)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	latency := time.Since(startTime)
	return latency, nil
}

func extractHost(rawURL string) string {
	parsed, err := neturl.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}
