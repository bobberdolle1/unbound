package observatory

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"syscall"
)

// Normalizers prefer typed context, net, and syscall errors. Native detail is
// retained separately in StageEvidence.Detail for platform-specific diagnosis.
func normalizeConnectError(err error) (Status, Classification, string) {
	if errors.Is(err, context.Canceled) {
		return StatusCancelled, ClassUnknown, "context_cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
		return StatusTimeout, ClassTCPConnectTimeout, "connect_timeout"
	}
	if isSocketError(err, syscall.ECONNREFUSED, 10061) {
		return StatusFail, ClassTCPConnectionRefused, "connection_refused"
	}
	if isSocketError(err, syscall.ECONNRESET, 10054) {
		return StatusReset, ClassTCPConnectionReset, "connection_reset"
	}
	if isSocketError(err, syscall.ENETUNREACH, syscall.EHOSTUNREACH, 10051, 10065) {
		return StatusFail, ClassTCPNetworkUnreachable, "network_unreachable"
	}
	return StatusFail, ClassTCPOtherFailure, "connect_error"
}

func normalizeTLSError(err error) (Status, Classification, string) {
	if errors.Is(err, context.Canceled) {
		return StatusCancelled, ClassUnknown, "context_cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
		return StatusTimeout, ClassTLSHandshakeTimeout, "tls_timeout"
	}
	if isSocketError(err, syscall.ECONNRESET, 10054) {
		return StatusReset, ClassTLSHandshakeReset, "tls_reset"
	}
	if isCertificateError(err) {
		return StatusFail, ClassTLSCertificateFailure, "tls_certificate_failure"
	}
	var recordHeader *tls.RecordHeaderError
	if errors.As(err, &recordHeader) {
		return StatusFail, ClassTLSProtocolFailure, "tls_protocol_failure"
	}
	return StatusFail, ClassTLSOtherFailure, "tls_error"
}

func normalizeHTTPError(err error) (Status, Classification, string) {
	if errors.Is(err, context.Canceled) {
		return StatusCancelled, ClassUnknown, "context_cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
		return StatusTimeout, ClassHTTPTimeout, "http_timeout"
	}
	if isSocketError(err, syscall.ECONNRESET, 10054) {
		return StatusReset, ClassHTTPReset, "http_reset"
	}
	var protocolErr *http.ProtocolError
	if errors.As(err, &protocolErr) {
		return StatusFail, ClassHTTPProtocolFailure, "http_protocol_failure"
	}
	return StatusFail, ClassHTTPProtocolFailure, "http_error"
}

func isTimeout(err error) bool {
	var networkErr net.Error
	return errors.As(err, &networkErr) && networkErr.Timeout()
}

func isSocketError(err error, wanted ...syscall.Errno) bool {
	for _, code := range wanted {
		if errors.Is(err, code) {
			return true
		}
	}
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	for _, code := range wanted {
		if errno == code {
			return true
		}
	}
	return false
}
