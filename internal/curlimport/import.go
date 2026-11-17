// Package curlimport converts a supported subset of cURL into Relay requests.
// It parses text only and never executes commands, reads files, or sends requests.
package curlimport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hsm-gustavo/relay/internal/httpclient"
)

type Result struct {
	Request  httpclient.Request
	Warnings []string
}

const MaxCommandBytes = 256 << 10

func Parse(command string) (Result, error) {
	if len(command) > MaxCommandBytes {
		return Result{}, fmt.Errorf("cURL command exceeds the 256 KiB import limit")
	}
	args, err := words(strings.TrimSpace(command))
	if err != nil {
		return Result{}, err
	}
	if len(args) == 0 || (args[0] != "curl" && args[0] != "curl.exe") {
		return Result{}, fmt.Errorf("paste one command beginning with curl (POSIX-style quoting)")
	}
	result := Result{Request: httpclient.Request{Method: http.MethodGet}}
	result.Warnings = []string{"Relay follows redirects and defaults to a 30-second total timeout. Review these settings before sending."}
	r := &result.Request
	method := ""
	head, get, hasBody, jsonBody, endOptions := false, false, false, false, false
	encodedData := false
	body, contentType := "", ""
	var data []string
	var headers []string
	setURL := func(value string) error {
		if r.URL != "" {
			return fmt.Errorf("only one URL per import is supported")
		}
		r.URL = value
		return nil
	}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if endOptions {
			if err := setURL(arg); err != nil {
				return Result{}, err
			}
			continue
		}
		if arg == "--" {
			endOptions = true
			continue
		}
		if !strings.HasPrefix(arg, "-") {
			if err := setURL(arg); err != nil {
				return Result{}, err
			}
			continue
		}
		option, value, attached := arg, "", false
		if strings.HasPrefix(arg, "--") {
			if name, v, ok := strings.Cut(arg, "="); ok {
				option, value, attached = name, v, true
			}
		} else if len(arg) > 2 && strings.ContainsRune("XHdum", rune(arg[1])) {
			option, value, attached = arg[:2], arg[2:], true
		}
		needsValue := false
		switch option {
		case "-X", "--request", "--url", "-H", "--header", "-u", "--user", "--oauth2-bearer", "-m", "--max-time", "-d", "--data", "--data-raw", "--data-binary", "--data-urlencode", "--json":
			needsValue = true
		}
		if needsValue && !attached {
			if i+1 >= len(args) {
				return Result{}, fmt.Errorf("%s requires a value", option)
			}
			i++
			value = args[i]
		}
		if !needsValue && attached {
			return Result{}, fmt.Errorf("%s does not accept a value", option)
		}
		switch option {
		case "-X", "--request":
			if value == "" {
				return Result{}, fmt.Errorf("HTTP method cannot be empty")
			}
			method = value
		case "--url":
			if err := setURL(value); err != nil {
				return Result{}, err
			}
		case "-I", "--head":
			head = true
		case "-G", "--get":
			get = true
		case "-H", "--header":
			if strings.HasPrefix(value, "@") {
				return Result{}, fmt.Errorf("header files are not supported")
			}
			headers = append(headers, value)
		case "-u", "--user":
			user, password, ok := strings.Cut(value, ":")
			if !ok {
				return Result{}, fmt.Errorf("use username:password for Basic Auth; password prompts are not supported")
			}
			if r.Auth.Type != "" {
				return Result{}, fmt.Errorf("multiple authentication settings are not supported")
			}
			r.Auth = httpclient.Auth{Type: "Basic", Username: user, Password: password}
		case "--oauth2-bearer":
			if r.Auth.Type != "" {
				return Result{}, fmt.Errorf("multiple authentication settings are not supported")
			}
			r.Auth = httpclient.Auth{Type: "Bearer", Token: value}
		case "-m", "--max-time":
			duration, err := time.ParseDuration(value + "s")
			if err != nil || duration < 0 {
				return Result{}, fmt.Errorf("max-time must be nonnegative seconds")
			}
			r.Timeout = duration.String()
		case "-d", "--data", "--data-raw", "--data-binary", "--json", "--data-urlencode":
			if option != "--data-raw" && strings.HasPrefix(value, "@") {
				return Result{}, fmt.Errorf("file and stdin bodies are not supported; paste inline JSON instead")
			}
			if option == "--data-urlencode" {
				encodedData = true
				key, v, ok := strings.Cut(value, "=")
				if !ok || key == "" {
					return Result{}, fmt.Errorf("data-urlencode supports only inline name=value pairs with --get")
				}
				value = key + "=" + url.QueryEscape(v)
			}

			hasBody = true
			body = value
			data = append(data, value)
			if option == "--json" {
				jsonBody = true
				contentType = "application/json"
				result.Warnings = append(result.Warnings, "--json's Accept: application/json header is not imported; Relay has no custom request-header editor.")
			}

		case "-L", "--location", "-s", "--silent", "-S", "--show-error", "-i", "--include":
		default:
			// Output-only short flags may be grouped, e.g. -sSL.
			grouped := strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--")
			for _, flag := range arg[1:] {
				if !strings.ContainsRune("sSLi", flag) {
					grouped = false
				}
			}
			if !grouped {
				return Result{}, fmt.Errorf("unsupported cURL option %s; Relay cannot represent this setting", option)
			}
		}
	}
	if strings.TrimSpace(r.URL) == "" {
		return Result{}, fmt.Errorf("the cURL command needs a URL")
	}
	if len(data) > 1 && !get {
		return Result{}, fmt.Errorf("multiple body options are supported only with --get")
	}
	if encodedData && !get {
		return Result{}, fmt.Errorf("--data-urlencode requires --get; Relay cannot send form bodies")
	}
	if jsonBody && get {
		return Result{}, fmt.Errorf("combining --json and --get is not supported")
	}
	if head && get {
		return Result{}, fmt.Errorf("combining --head and --get is not supported")
	}
	if head {
		r.Method = http.MethodHead
	}
	if hasBody && !get {
		r.Method = http.MethodPost
		r.Body = body
	}
	if method != "" {
		r.Method = method
	}
	if head && (hasBody || r.Method != http.MethodHead) {
		return Result{}, fmt.Errorf("--head with a body or a different explicit method cannot be represented")
	}
	supported := false
	for _, m := range httpclient.Methods {
		if r.Method == m {
			supported = true
			break
		}
	}
	if !supported {
		return Result{}, fmt.Errorf("HTTP method %q is not supported by Relay", r.Method)
	}
	if get && hasBody {
		query := strings.Join(data, "&")
		if _, err := url.ParseQuery(query); err != nil {
			return Result{}, fmt.Errorf("invalid --get query data: %w", err)
		}
		base, fragment, fragmented := strings.Cut(r.URL, "#")
		separator := "?"
		if strings.Contains(base, "?") {
			separator = "&"
		}
		r.URL = base + separator + query
		if fragmented {
			r.URL += "#" + fragment
		}
	}
	for _, header := range headers {
		name, value, ok := strings.Cut(header, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return Result{}, fmt.Errorf("headers must use Name: value; header suppression is not supported")
		}
		value = strings.TrimSpace(value)
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "content-type":
			if contentType != "" && !strings.EqualFold(contentType, value) {
				return Result{}, fmt.Errorf("conflicting Content-Type headers")
			}
			contentType = value
		case "cookie":
			return Result{}, fmt.Errorf("cookies are not supported by Relay yet")
		case "authorization":
			if r.Auth.Type != "" {
				return Result{}, fmt.Errorf("multiple authentication settings are not supported")
			}
			scheme, credentials, ok := strings.Cut(value, " ")
			if !ok {
				return Result{}, fmt.Errorf("Authorization supports Basic or Bearer only")
			}
			switch strings.ToLower(scheme) {
			case "bearer":
				r.Auth = httpclient.Auth{Type: "Bearer", Token: strings.TrimSpace(credentials)}
			case "basic":
				decoded, err := base64.StdEncoding.DecodeString(credentials)
				if err != nil {
					return Result{}, fmt.Errorf("invalid Basic Authorization encoding")
				}
				user, password, ok := strings.Cut(string(decoded), ":")
				if !ok {
					return Result{}, fmt.Errorf("Basic Authorization must encode username:password")
				}
				r.Auth = httpclient.Auth{Type: "Basic", Username: user, Password: password}
			default:
				return Result{}, fmt.Errorf("Authorization supports Basic or Bearer only")
			}
		default:
			result.Warnings = append(result.Warnings, fmt.Sprintf("Header %q is not imported; Relay has no custom request-header editor.", strings.TrimSpace(name)))
		}
	}
	if r.Body != "" || (hasBody && !get) {
		if !json.Valid([]byte(r.Body)) && !strings.Contains(r.Body, "{{") {
			return Result{}, fmt.Errorf("Relay supports inline JSON bodies only; form data and other body formats cannot be imported")
		}
		if contentType != "" && !strings.EqualFold(contentType, "application/json") {
			return Result{}, fmt.Errorf("Relay supports Content-Type: application/json only")
		}
		if contentType == "" && !jsonBody {
			result.Warnings = append(result.Warnings, "cURL defaults --data to form Content-Type; Relay will send this JSON body as application/json.")
		}
	} else if contentType != "" {
		return Result{}, fmt.Errorf("Relay cannot set Content-Type without a JSON body")
	}
	if !strings.Contains(r.URL+r.Params+r.Body+r.Auth.Username+r.Auth.Password+r.Auth.Token, "{{") {
		if _, err := r.Build(context.Background()); err != nil {
			return Result{}, err
		}
	}
	return result, nil
}
