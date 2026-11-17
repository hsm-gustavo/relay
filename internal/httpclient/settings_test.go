package httpclient

import (
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestAuthentication(t *testing.T) {
	for _, tt := range []struct {
		name    string
		auth    Auth
		want    string
		invalid bool
	}{
		{name: "none", auth: Auth{Type: "None", Username: "ignored", Password: "ignored", Token: "ignored"}},
		{name: "basic", auth: Auth{Type: "Basic", Username: "user", Password: "p:ass"}, want: "Basic dXNlcjpwOmFzcw=="},
		{name: "bearer", auth: Auth{Type: "Bearer", Token: "  token.abc-123  "}, want: "Bearer token.abc-123"},
		{name: "invalid basic username", auth: Auth{Type: "Basic", Username: "user:name"}, invalid: true},
		{name: "empty bearer", auth: Auth{Type: "Bearer"}, invalid: true},
		{name: "header injection", auth: Auth{Type: "Bearer", Token: "token\r\nX-Test: bad"}, invalid: true},
		{name: "unknown auth", auth: Auth{Type: "Other"}, invalid: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req, err := (Request{Method: "GET", URL: "https://example.com", Auth: tt.auth}).Build(context.Background())
			if tt.invalid {
				if err == nil {
					t.Fatal("expected auth validation error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := req.Header.Get("Authorization"); got != tt.want {
				t.Errorf("authorization = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTimeoutSettings(t *testing.T) {
	for _, tt := range []struct {
		input   string
		want    time.Duration
		invalid bool
	}{
		{"", 30 * time.Second, false}, {" 2m ", 2 * time.Minute, false}, {"100ms", 100 * time.Millisecond, false},
		{"0", 0, false}, {"-1s", 0, true}, {"forever", 0, true}, {"30", 0, true},
	} {
		t.Run(tt.input, func(t *testing.T) {
			got, err := (Request{Timeout: tt.input}).TimeoutDuration()
			if tt.invalid {
				if err == nil {
					t.Fatal("expected timeout validation error")
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("timeout = %v, %v; want %v", got, err, tt.want)
			}
		})
	}
}

func TestErrorMessages(t *testing.T) {
	for _, tt := range []struct {
		err  error
		want string
	}{
		{context.Canceled, "cancelled"},
		{fmt.Errorf("read response: %w", context.DeadlineExceeded), "timed out"},
		{&url.Error{Op: "Get", URL: "http://example.com", Err: &net.DNSError{Name: "example.com", Err: "no such host"}}, "resolved"},
		{&net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}, "Connection refused"},
		{fmt.Errorf("send: %w", x509.UnknownAuthorityError{}), "TLS certificate"},
		{io.ErrUnexpectedEOF, "full response"},
	} {
		if got := ErrorMessage(tt.err); !strings.Contains(got, tt.want) {
			t.Errorf("message = %q, want %q", got, tt.want)
		}
	}
	if ErrorMessage(nil) != "" {
		t.Error("nil error should not produce a message")
	}
}
