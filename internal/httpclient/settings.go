package httpclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"time"
)

func (r Request) TimeoutDuration() (time.Duration, error) {
	value := strings.TrimSpace(r.Timeout)
	if value == "" {
		return 30 * time.Second, nil
	}
	timeout, err := time.ParseDuration(value)
	if err != nil || timeout < 0 {
		return 0, fmt.Errorf("timeout must be a duration such as 30s or 2m; use 0 for no timeout")
	}
	return timeout, nil
}

// ErrorMessage explains common failures without hiding HTTP error responses.
func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return "Request cancelled."
	}
	if errors.Is(err, ErrResponseTooLarge) {
		return err.Error()
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return "The server reset the connection. Any received response data is shown below."
	}
	var tlsRecord tls.RecordHeaderError
	if errors.As(err, &tlsRecord) {
		return "The server did not return a valid TLS response. Check whether this endpoint uses HTTP instead of HTTPS."
	}
	var timeout net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout()) {
		return "Request timed out. Increase the timeout in Settings or check the server."
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return "Host could not be resolved. Check the hostname and your network connection."
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "Connection refused. Check the host, port, and whether the server is running."
	}
	var authority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var certificate x509.CertificateInvalidError
	if errors.As(err, &authority) || errors.As(err, &hostname) || errors.As(err, &certificate) {
		return "TLS certificate verification failed. Check the server certificate and hostname."
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return "The connection closed before the full response was received."
	}
	return err.Error()
}
