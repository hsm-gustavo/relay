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
	contentType := ""
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
		} else if len(arg) > 2 && strings.ContainsRune("XHdumF", rune(arg[1])) {
			option, value, attached = arg[:2], arg[2:], true
		}
		needsValue := false
		switch option {
		case "-F", "--form", "--form-string", "-X", "--request", "--url", "-H", "--header", "-u", "--user", "--oauth2-bearer", "-m", "--max-time", "-d", "--data", "--data-raw", "--data-binary", "--data-urlencode", "--json":
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
		case "-F", "--form", "--form-string":
			if hasBody && len(data) > 0 {
				return Result{}, fmt.Errorf("cannot mix multipart fields and data options")
			}
			field, err := parseMultipartField(value, option == "--form-string")
			if err != nil {
				return Result{}, err
			}
			r.FormFields = append(r.FormFields, field)
			r.BodyType = httpclient.BodyMultipart
			hasBody = true
		case "-d", "--data", "--data-raw", "--data-binary", "--json", "--data-urlencode":
			if r.BodyType == httpclient.BodyMultipart {
				return Result{}, fmt.Errorf("cannot mix multipart fields and data options")
			}
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
		if r.BodyType != httpclient.BodyMultipart {
			r.Body = strings.Join(data, "&")
		}
	}
	if r.BodyType == httpclient.BodyMultipart && get {
		return Result{}, fmt.Errorf("multipart fields cannot be combined with --get")
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
	if r.BodyType == httpclient.BodyMultipart {
		for _, field := range r.FormFields {
			if field.Type == "file" {
				result.Warnings = append(result.Warnings, "Multipart file paths are imported without reading files. Review them in Body before sending.")
				break
			}
		}
		if contentType != "" && !strings.EqualFold(contentType, httpclient.BodyMultipart) {
			return Result{}, fmt.Errorf("multipart Content-Type must omit any custom boundary; Relay generates it")
		}
	} else if hasBody && !get {
		if jsonBody || strings.EqualFold(contentType, httpclient.BodyJSON) || (contentType == "" && !encodedData && (json.Valid([]byte(r.Body)) || (strings.Contains(r.Body, "{{") && strings.HasPrefix(strings.TrimSpace(r.Body), "{")))) {
			if len(data) > 1 {
				return Result{}, fmt.Errorf("multiple JSON body options are not supported")
			}
			if contentType != "" && !strings.EqualFold(contentType, httpclient.BodyJSON) {
				return Result{}, fmt.Errorf("conflicting JSON Content-Type")
			}
			r.BodyType = httpclient.BodyJSON
			if !json.Valid([]byte(r.Body)) && !strings.Contains(r.Body, "{{") {
				return Result{}, fmt.Errorf("invalid JSON body")
			}
			if contentType == "" && !jsonBody {
				result.Warnings = append(result.Warnings, "cURL defaults --data to form Content-Type; Relay will send this JSON body as application/json.")
			}
		} else {
			if contentType != "" && !strings.EqualFold(contentType, httpclient.BodyURLEncoded) {
				return Result{}, fmt.Errorf("supported body Content-Types are JSON, URL-encoded, and multipart")
			}
			fields, err := parseURLEncoded(r.Body)
			if err != nil {
				return Result{}, err
			}
			r.BodyType = httpclient.BodyURLEncoded
			r.FormFields = fields
			r.Body = ""
		}
	} else if contentType != "" {
		return Result{}, fmt.Errorf("Relay cannot set Content-Type without a body")
	}

	if !strings.Contains(r.URL+r.Params+r.Body+r.Auth.Username+r.Auth.Password+r.Auth.Token, "{{") {
		if request, err := r.Build(context.Background()); err != nil {
			return Result{}, err
		} else if request.Body != nil {
			_ = request.Body.Close()
		}
	}
	return result, nil
}
