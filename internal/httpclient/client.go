package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hsm-gustavo/relay/internal/extraction"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
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
	BodyType   string            `json:",omitempty"`
	FormFields []FormField       `json:",omitempty"`
	Extract    []extraction.Rule `json:",omitempty"`
	Method     string
	URL        string
	// Params contains one unescaped key=value pair per line. Duplicate keys are allowed.
	Params  string
	Body    string
	Auth    Auth
	Timeout string // Empty uses the default 30 seconds; 0 disables the timeout.
}

const MaxResponseBytes = 16 << 20

var ErrResponseTooLarge = errors.New("response exceeds the 16 MiB limit; only the first 16 MiB were retained")

type Response struct {
	Incomplete bool
	BodyFile   string `json:",omitempty"` // Used by local workspace storage.

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
	req, err := http.NewRequestWithContext(ctx, r.Method, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
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
		if token == "" || strings.IndexFunc(token, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
			return nil, fmt.Errorf("enter a bearer token without spaces or line breaks")
		}
		req.Header.Set("Authorization", "Bearer "+token)
	default:
		return nil, fmt.Errorf("choose None, Basic, or Bearer authentication")
	}
	body, contentType, replay, err := r.body(ctx)
	if err != nil {
		return nil, err
	}
	req.Body = body
	req.GetBody = replay
	if body != nil {
		req.Header.Set("Content-Type", contentType)
		if sized, ok := body.(interface{ Len() int }); ok {
			req.ContentLength = int64(sized.Len())
			if req.ContentLength == 0 {
				req.Body = http.NoBody
			}
		}
		if r.EffectiveBodyType() == BodyMultipart {
			req.ContentLength = -1
		}
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
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	result := &Response{Status: resp.Status, Duration: time.Since(start), Headers: resp.Header.Clone(), Body: body}
	if len(body) > MaxResponseBytes {
		result.Body = body[:MaxResponseBytes]
		result.Incomplete = true
		return result, ErrResponseTooLarge
	}
	if readErr != nil {
		result.Incomplete = true
		return result, fmt.Errorf("read response: %w", readErr)
	}
	return result, nil
}

// FormatBody preserves non-JSON responses and the original bytes in raw mode.
func FormatBody(body []byte, pretty bool) string {
	if pretty && len(body) <= 64<<10 && safeJSONDepth(body) {
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

func safeJSONDepth(body []byte) bool {
	depth := 0
	maxDepth := 0
	quoted, escaped := false, false
	for _, b := range body {
		if quoted {
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '"' {
				quoted = false
			}
			continue
		}
		switch b {
		case '"':
			quoted = true
		case '{', '[':
			depth++
			if depth > maxDepth {
				maxDepth = depth
			}
			if depth > 64 {
				return false
			}
		case '}', ']':
			depth--
		}
	}
	return len(body)*(2*maxDepth+1) <= 256<<10
}

func (r Request) HasEnabledExtractionRules() bool {
	for _, rule := range r.Extract {
		if !rule.Disabled {
			return true
		}
	}
	return false
}
