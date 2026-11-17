package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMethodsAndResponse(t *testing.T) {
	for _, method := range Methods {
		t.Run(method, func(t *testing.T) {
			const payload = `{"message":"hello","id":9007199254740993}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != method {
					t.Errorf("method = %s, want %s", r.Method, method)
				}
				got, err := io.ReadAll(r.Body)
				if err != nil || string(got) != payload {
					t.Errorf("request body = %q, %v", got, err)
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("missing JSON content type")
				}
				w.Header().Add("X-Relay", "one")
				w.Header().Add("X-Relay", "two")
				w.WriteHeader(http.StatusTeapot)
				_, _ = io.WriteString(w, payload)
			}))
			defer server.Close()
			req, err := (Request{Method: method, URL: server.URL, Body: payload}).Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			result, err := Execute(server.Client(), req)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != "418 I'm a teapot" {
				t.Errorf("status = %q", result.Status)
			}
			if result.Duration <= 0 {
				t.Error("duration must include response transfer")
			}
			wantBody := payload
			if method == http.MethodHead {
				wantBody = ""
			}
			if string(result.Body) != wantBody {
				t.Errorf("body = %q, want %q", result.Body, wantBody)
			}
			if !reflect.DeepEqual(result.Headers.Values("X-Relay"), []string{"one", "two"}) {
				t.Errorf("headers = %v", result.Headers)
			}
		})
	}
}

func TestQueryParameters(t *testing.T) {
	req, err := (Request{
		Method: http.MethodGet,
		URL:    "https://example.com/path?tag=original&existing=a%2Bb#fragment",
		Params: "tag=go\r\ntag=fyne\nsearch=hello world & + café\nempty=\nequals=a=b\npercent=100%\n\n",
	}).Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"tag": {"original", "go", "fyne"}, "existing": {"a+b"},
		"search": {"hello world & + café"}, "empty": {""}, "equals": {"a=b"}, "percent": {"100%"},
	}
	if !reflect.DeepEqual(map[string][]string(req.URL.Query()), want) {
		t.Errorf("query = %v, want %v", req.URL.Query(), want)
	}
	if req.URL.Path != "/path" || req.URL.Fragment != "fragment" {
		t.Errorf("URL = %s", req.URL)
	}
	if req.Body != nil || req.Header.Get("Content-Type") != "" {
		t.Error("empty body must not set JSON content type")
	}
}

func TestURLSchemes(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"127.0.0.1:8080", "http://127.0.0.1:8080"},
		{"localhost:8080/api?q=hello%20world", "http://localhost:8080/api?q=hello+world"},
		{"example.com", "http://example.com"},
		{"example.com/api?next=https://other.example", "http://example.com/api?next=https%3A%2F%2Fother.example"},
		{" example.com/api ", "http://example.com/api"},
		{"[::1]:8080/api", "http://[::1]:8080/api"},
		{"//localhost:8080/api", "http://localhost:8080/api"},
		{"http://example.com", "http://example.com"},
		{"https://example.com/api", "https://example.com/api"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			req, err := (Request{Method: "GET", URL: tt.input}).Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if got := req.URL.String(); got != tt.want {
				t.Errorf("URL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExecuteWithoutScheme(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "local response")
	}))
	defer server.Close()
	req, err := (Request{Method: "GET", URL: strings.TrimPrefix(server.URL, "http://")}).Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(server.Client(), req)
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Body) != "local response" {
		t.Errorf("body = %q", result.Body)
	}
}

func TestInvalidRequests(t *testing.T) {
	tests := []struct {
		name  string
		input Request
		want  string
	}{
		{"missing method", Request{URL: "https://example.com"}, "method"},
		{"unknown method", Request{Method: "UNKNOWN", URL: "https://example.com"}, "method"},
		{"missing URL", Request{Method: "GET"}, "complete"},
		{"malformed HTTP scheme", Request{Method: "GET", URL: "http:/example.com"}, "complete"},
		{"malformed HTTPS scheme", Request{Method: "GET", URL: "https:example.com"}, "complete"},
		{"unsupported scheme", Request{Method: "GET", URL: "file:///etc/hosts"}, "complete"},
		{"missing host", Request{Method: "GET", URL: "https:///path"}, "complete"},
		{"malformed URL", Request{Method: "GET", URL: "http://["}, "invalid URL"},
		{"invalid URL query", Request{Method: "GET", URL: "https://example.com?q=%xx"}, "invalid URL query"},
		{"invalid param", Request{Method: "GET", URL: "https://example.com", Params: "ok=yes\nbroken"}, "line 2"},
		{"empty param name", Request{Method: "GET", URL: "https://example.com", Params: "=value"}, "key=value"},
		{"invalid JSON", Request{Method: "POST", URL: "https://example.com", Body: `{"broken":}`}, "valid JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.input.Build(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestFormatBody(t *testing.T) {
	for _, tt := range []struct {
		name, body, want string
		pretty           bool
	}{
		{"pretty", `{"id":9007199254740993,"ok":true}`, "{\n  \"id\": 9007199254740993,\n  \"ok\": true\n}", true},
		{"raw JSON", `{"ok":true}`, `{"ok":true}`, false},
		{"text fallback", "plain text\n", "plain text\n", true},
		{"invalid JSON fallback", `{"bad":}`, `{"bad":}`, true},
		{"empty", "", "", true},
		{"array", `[1,2]`, "[\n  1,\n  2\n]", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatBody([]byte(tt.body), tt.pretty); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatHeaders(t *testing.T) {
	headers := http.Header{"X-Test": {"one", "two"}, "Content-Type": {"application/json"}}
	want := "Content-Type: application/json\r\nX-Test: one\r\nX-Test: two\r\n"
	if got := FormatHeaders(headers); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	client := server.Client()
	client.Timeout = 30 * time.Millisecond
	req, err := (Request{Method: "GET", URL: server.URL}).Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(client, req); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want deadline exceeded", err)
	}
}

func TestReadFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = io.WriteString(w, "truncated")
	}))
	defer server.Close()
	req, err := (Request{Method: "GET", URL: server.URL}).Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(server.Client(), req); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("error = %v, want unexpected EOF", err)
	}
}
