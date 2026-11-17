package curlimport

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/hsm-gustavo/relay/internal/httpclient"
)

func parseURLEncoded(body string) ([]httpclient.FormField, error) {
	var fields []httpclient.FormField
	for _, part := range strings.Split(body, "&") {
		if part == "" {
			continue
		}
		key, value, _ := strings.Cut(part, "=")
		key, err := url.QueryUnescape(key)
		if err != nil {
			return nil, fmt.Errorf("invalid URL-encoded field name")
		}
		value, err = url.QueryUnescape(value)
		if err != nil {
			return nil, fmt.Errorf("invalid URL-encoded field value")
		}
		if key == "" {
			return nil, fmt.Errorf("URL-encoded fields need a name")
		}
		fields = append(fields, httpclient.FormField{Name: key, Value: value})
	}
	return fields, nil
}

func parseMultipartField(text string, literal bool) (httpclient.FormField, error) {
	name, value, ok := strings.Cut(text, "=")
	if !ok || strings.TrimSpace(name) == "" {
		return httpclient.FormField{}, fmt.Errorf("multipart fields must use name=value or name=@file")
	}
	field := httpclient.FormField{Name: name, Value: value}
	if literal {
		return field, nil
	}
	if strings.HasPrefix(value, "<") {
		return field, fmt.Errorf("multipart file-as-text fields are not supported; use name=@file")
	}
	if strings.Contains(value, ";") {
		return field, fmt.Errorf("multipart type/filename modifiers are not supported; choose the file in Relay or use --form-string for literal text")
	}
	if strings.HasPrefix(value, "@") {
		path := strings.TrimPrefix(value, "@")
		if path == "" || path == "-" || strings.Contains(path, ",") {
			return field, fmt.Errorf("use one explicit local file path per multipart field; stdin and file lists are not supported")
		}
		field.Type = "file"
		field.FilePath = path
		field.Value = ""
	}
	return field, nil
}
