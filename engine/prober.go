package engine

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strings"
	"time"
)

type ProbeResult struct {
	URL           string
	Success       bool
	Latency       time.Duration
	CertValid     bool
	CertIssuer    string
	Error         string
	ConnectionRST bool
}

var trustedIssuers = map[string][]string{
	"discord.com": {
		"DigiCert", "Let's Encrypt", "Google Trust Services",
	},
	"googlevideo.com": {
		"Google Trust Services", "GTS",
	},
	"youtube.com": {
		"Google Trust Services", "GTS",
	},
	"telegram.org": {
		"DigiCert", "Let's Encrypt",
	},
}

func ProbeConnection(ctx context.Context, targetURL string, engine DPIEngine) (ProbeResult, error) {
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
		ServerName:         host,
		InsecureSkipVerify: false,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			if len(verifiedChains) == 0 || len(verifiedChains[0]) == 0 {
				result.CertValid = false
				return fmt.Errorf("no certificate chain provided")
			}

			cert := verifiedChains[0][0]
			result.CertIssuer = cert.Issuer.Organization[0]

			if expectedIssuers, ok := trustedIssuers[host]; ok {
				for _, expected := range expectedIssuers {
					if strings.Contains(result.CertIssuer, expected) {
						result.CertValid = true
						return nil
					}
				}
				result.CertValid = false
				return fmt.Errorf("untrusted issuer: %s (expected one of %v)", result.CertIssuer, expectedIssuers)
			}

			result.CertValid = true
			return nil
		},
	}

	tlsDialer := &tls.Dialer{
		NetDialer: &net.Dialer{
			Timeout: 10 * time.Second,
		},
		Config: tlsConfig,
	}

	conn, err := tlsDialer.DialContext(ctx, "tcp", host+":443")
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

	result.Success = true
	return result, nil
}

func ProbeMultipleTargets(ctx context.Context, targets []string, engine DPIEngine) []ProbeResult {
	results := make([]ProbeResult, 0, len(targets))

	for _, target := range targets {
		select {
		case <-ctx.Done():
			return results
		default:
			result, _ := ProbeConnection(ctx, target, engine)
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
		ServerName:         host,
		InsecureSkipVerify: true,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", host+":443", tlsConfig)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	latency := time.Since(startTime)
	return latency, nil
}

func extractHost(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	if idx := strings.Index(url, "/"); idx != -1 {
		url = url[:idx]
	}
	if idx := strings.Index(url, ":"); idx != -1 {
		url = url[:idx]
	}
	return url
}
