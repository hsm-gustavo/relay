package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var Methods = []string{
	http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
	http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace,
}

type Auth struct {
	Type     string
	Username string
	Password string
	Token    string
}

type Request struct {
	Method string
	URL    string
	// Params contains one unescaped key=value pair per line. Duplicate keys are allowed.
	Params  string
	Body    string
	Auth    Auth
	Timeout string // Empty uses the default 30 seconds; 0 disables the timeout.
}

type Response struct {
	Status   string
	Duration time.Duration
	Headers  http.Header
	Body     []byte
}

func (r Request) Build(ctx context.Context) (*http.Request, error) {
	validMethod := false
	for _, method := range Methods {
		if r.Method == method {
			validMethod = true
			break
		}
	}
	if !validMethod {
		return nil, fmt.Errorf("select a supported HTTP method")
	}
	rawURL := strings.TrimSpace(r.URL)
	scheme, _, hasScheme := strings.Cut(rawURL, "://")
	hasScheme = hasScheme && !strings.ContainsAny(scheme, "/?#")
	// Add the scheme before parsing so host:port is not mistaken for a scheme.
	if strings.HasPrefix(rawURL, "//") {
		rawURL = "http:" + rawURL
	} else if rawURL != "" && !hasScheme &&
		!strings.HasPrefix(strings.ToLower(rawURL), "http:") &&
		!strings.HasPrefix(strings.ToLower(rawURL), "https:") {
		rawURL = "http://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil, fmt.Errorf("enter a complete HTTP or HTTPS URL")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("invalid URL query: %w", err)
	}
	for i, line := range strings.Split(r.Params, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("query parameter on line %d must be key=value", i+1)
		}
		query.Add(strings.TrimSpace(key), value)
	}
	u.RawQuery = query.Encode()
	var body io.Reader
	if strings.TrimSpace(r.Body) != "" {
		if !json.Valid([]byte(r.Body)) {
			return nil, fmt.Errorf("request body must be valid JSON")
		}
		body = strings.NewReader(r.Body)
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	switch r.Auth.Type {
	case "", "None":
	case "Basic":
		if strings.Contains(r.Auth.Username, ":") {
			return nil, fmt.Errorf("Basic Auth username cannot contain a colon")
		}
		req.SetBasicAuth(r.Auth.Username, r.Auth.Password)
	case "Bearer":
		token := strings.TrimSpace(r.Auth.Token)
		if token == "" || strings.ContainsAny(token, " \t\r\n") {
			return nil, fmt.Errorf("enter a bearer token without spaces or line breaks")
		}
		req.Header.Set("Authorization", "Bearer "+token)
	default:
		return nil, fmt.Errorf("choose None, Basic, or Bearer authentication")
	}
	return req, nil
}

// Execute measures the complete transfer, including reading the response body.
func Execute(client *http.Client, req *http.Request) (*Response, error) {
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return &Response{
		Status: resp.Status, Duration: time.Since(start),
		Headers: resp.Header.Clone(), Body: body,
	}, nil
}

// FormatBody preserves non-JSON responses and the original bytes in raw mode.
func FormatBody(body []byte, pretty bool) string {
	if pretty {
		var formatted bytes.Buffer
		if err := json.Indent(&formatted, body, "", "  "); err == nil {
			return formatted.String()
		}
	}
	return string(body)
}

func FormatHeaders(headers http.Header) string {
	var formatted bytes.Buffer
	_ = headers.Write(&formatted)
	return formatted.String()
}
