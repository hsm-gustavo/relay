package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponseLimitAndPartialRead(t *testing.T) {
	for _, tt := range []struct {
		name   string
		body   string
		length string
		want   error
	}{
		{"too large", strings.Repeat("x", MaxResponseBytes+100), "", ErrResponseTooLarge},
		{"truncated", "partial", "100", io.ErrUnexpectedEOF},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.length != "" {
					w.Header().Set("Content-Length", tt.length)
				}
				w.Header().Set("X-Test", "preserved")
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()
			req, _ := (Request{Method: "GET", URL: server.URL}).Build(context.Background())
			result, err := Execute(server.Client(), req)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v", err)
			}
			if result == nil || !result.Incomplete || result.Status != "200 OK" || result.Headers.Get("X-Test") != "preserved" || len(result.Body) == 0 || len(result.Body) > MaxResponseBytes {
				t.Fatal("partial response was lost or unbounded")
			}
		})
	}
}

func TestDeepJSONDoesNotExpand(t *testing.T) {
	source := strings.Repeat("[", 1000) + "0" + strings.Repeat("]", 1000)
	if got := FormatBody([]byte(source), true); got != source {
		t.Fatal("deep JSON should remain raw instead of expanding")
	}
}
