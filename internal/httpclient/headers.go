package httpclient

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/http/httpguts"
)

type HeaderField struct {
	Name, Value string
	Disabled    bool
}

func (r Request) customHeaders() (http.Header, error) {
	headers := make(http.Header)
	for i, row := range r.Headers {
		if row.Disabled {
			continue
		}
		if !httpguts.ValidHeaderFieldName(row.Name) || !httpguts.ValidHeaderFieldValue(row.Value) {
			return nil, fmt.Errorf("invalid name or value in header row %d", i+1)
		}
		name := http.CanonicalHeaderKey(row.Name)
		switch name {
		case "Host", "Content-Length", "Transfer-Encoding", "Trailer", "Connection", "Upgrade", "Proxy-Connection", "Proxy-Authorization", "Keep-Alive", "Te":
			return nil, fmt.Errorf("header row %d is managed by the transport", i+1)
		case "Authorization":
			if headers.Get(name) != "" || len(headers.Values(name)) > 0 || (r.Auth.Type != "" && r.Auth.Type != "None") {
				return nil, fmt.Errorf("use either a single Authorization header or configured authentication")
			}
		case "Content-Type":
			if len(headers.Values(name)) > 0 || r.EffectiveBodyType() == BodyMultipart {
				return nil, fmt.Errorf("use one Content-Type header and let Relay generate multipart boundaries")
			}
		}
		headers.Add(name, row.Value)
	}
	return headers, nil
}

// DisplayURL hides user info, the fragment, and query values. A resolved
// session variable can appear anywhere in the URL, so hide the whole URL then.
func DisplayURL(u *url.URL, sensitive bool) string {
	if sensitive {
		return "[masked]"
	}
	copy := *u
	copy.User = nil
	copy.Fragment = ""
	copy.RawFragment = ""
	if copy.RawQuery != "" {
		copy.RawQuery = "masked"
	}
	return copy.String()
}

func InspectRequest(req *http.Request, sensitive bool, custom []HeaderField) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%s %s\n", req.Method, DisplayURL(req.URL, sensitive))
	headers := req.Header.Clone()
	for _, header := range custom {
		if !header.Disabled {
			headers[http.CanonicalHeaderKey(header.Name)] = []string{"[masked]"}
		}
	}
	for name := range headers {
		if sensitive || name == "Authorization" || name == "Cookie" {
			headers[name] = []string{"[masked]"}
		}
	}
	out.WriteString(FormatHeaders(headers))
	return out.String()
}
