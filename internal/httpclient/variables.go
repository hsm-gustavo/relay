package httpclient

import (
	"fmt"
	"strings"
)

func ValidVariableName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' {
			continue
		}
		if i > 0 && ((r >= '0' && r <= '9') || r == '.' || r == '-') {
			continue
		}
		return false
	}
	return true
}

// Resolve substitutes values once in a copy. Templates and environment values
// are never modified, and variable values are literal rather than recursive.
func (r Request) Resolve(variables map[string]string) (Request, error) {
	result, _, err := r.ResolveTracked(variables)
	return result, err
}

// ResolveTracked also reports referenced names, so session-derived values can stay off disk.
func (r Request) ResolveTracked(variables map[string]string) (Request, map[string]bool, error) {
	used := make(map[string]bool)
	fields := []struct {
		name   string
		target *string
	}{
		{"URL", &r.URL}, {"query params", &r.Params}, {"timeout", &r.Timeout},
	}
	switch r.EffectiveBodyType() {
	case BodyJSON:
		fields = append(fields, struct {
			name   string
			target *string
		}{"JSON body", &r.Body})
	case BodyURLEncoded, BodyMultipart:
		r.FormFields = append([]FormField(nil), r.FormFields...)
		for i := range r.FormFields {
			field := &r.FormFields[i]
			if field.Disabled {
				continue
			}
			fields = append(fields, struct {
				name   string
				target *string
			}{"form field name", &field.Name})
			if field.Type == "file" {
				fields = append(fields, struct {
					name   string
					target *string
				}{"upload path", &field.FilePath})
			} else {
				fields = append(fields, struct {
					name   string
					target *string
				}{"form value", &field.Value})
			}
		}
	}
	switch r.Auth.Type {
	case "Basic":
		fields = append(fields, struct {
			name   string
			target *string
		}{"username", &r.Auth.Username}, struct {
			name   string
			target *string
		}{"password", &r.Auth.Password})
	case "Bearer":
		fields = append(fields, struct {
			name   string
			target *string
		}{"bearer token", &r.Auth.Token})
	}
	r.Headers = append([]HeaderField(nil), r.Headers...)
	for i := range r.Headers {
		row := &r.Headers[i]
		if !row.Disabled {
			fields = append(fields, struct {
				name   string
				target *string
			}{"header name", &row.Name}, struct {
				name   string
				target *string
			}{"header value", &row.Value})
		}
	}
	for _, field := range fields {
		value, err := substitute(*field.target, variables, used)
		if err != nil {
			return Request{}, used, fmt.Errorf("%s: %w", field.name, err)
		}
		*field.target = value
	}
	return r, used, nil
}

func substitute(text string, variables map[string]string, used map[string]bool) (string, error) {
	var output strings.Builder
	for {
		start := strings.Index(text, "{{")
		if start < 0 {
			output.WriteString(text)
			return output.String(), nil
		}
		output.WriteString(text[:start])
		rest := text[start+2:]
		end := strings.Index(rest, "}}")
		if end < 0 {
			return "", fmt.Errorf("unclosed variable reference; use {{name}}")
		}
		name := strings.TrimSpace(rest[:end])
		if !ValidVariableName(name) {
			return "", fmt.Errorf("invalid variable name %q", name)
		}
		used[name] = true
		value, ok := variables[name]
		if !ok {
			return "", fmt.Errorf("undefined variable %q; select an environment that defines it", name)
		}
		output.WriteString(value)
		text = rest[end+2:]
	}
}
