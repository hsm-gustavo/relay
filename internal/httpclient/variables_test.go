package httpclient

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestResolveEnvironmentVariables(t *testing.T) {
	template := Request{Method: "POST", URL: "{{base_url}}/users/{{ id }}?source={{source}}", Params: "search={{search}}", Body: `{"name":"{{name}}","id":{{id}}}`, Auth: Auth{Type: "Bearer", Token: "{{token}}"}, Timeout: "{{timeout}}"}
	values := map[string]string{"base_url": "http://localhost:8080", "id": "42", "source": "relay", "search": "Ada & Grace", "name": "Ada", "token": "secret", "timeout": "2m"}
	resolved, err := template.Resolve(values)
	if err != nil {
		t.Fatal(err)
	}
	req, err := resolved.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.String() != "http://localhost:8080/users/42?search=Ada+%26+Grace&source=relay" {
		t.Errorf("URL = %s", req.URL)
	}
	body, _ := io.ReadAll(req.Body)
	if string(body) != `{"name":"Ada","id":42}` || req.Header.Get("Authorization") != "Bearer secret" || resolved.Timeout != "2m" {
		t.Fatal("body, auth or timeout was not resolved")
	}
	if template.URL != "{{base_url}}/users/{{ id }}?source={{source}}" || values["base_url"] != "http://localhost:8080" {
		t.Fatal("template or environment changed")
	}
	values["base_url"] = "https://api.example.com"
	production, err := template.Resolve(values)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(production.URL, "https://api.example.com/") {
		t.Fatal("switching environment did not change the target")
	}
}

func TestVariableErrorsAndLiteralValues(t *testing.T) {
	for _, tt := range []struct {
		url  string
		want string
	}{
		{"{{missing}}/users", "undefined variable"}, {"{{base_url/users", "unclosed"}, {"{{}}", "invalid variable"}, {"{{bad name}}", "invalid variable"},
	} {
		_, err := (Request{URL: tt.url}).Resolve(nil)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s error = %v", tt.url, err)
		}
	}
	r, err := (Request{URL: "{{empty}}/{{literal}}", Auth: Auth{Type: "None", Token: "{{unused}}"}}).Resolve(map[string]string{"empty": "", "literal": "{{not_recursive}}"})
	if err != nil || r.URL != "/{{not_recursive}}" {
		t.Fatal("values must be substituted once, including empty values")
	}
	r, err = (Request{Auth: Auth{Type: "Basic", Username: "{{user}}", Password: "{{pass}}"}}).Resolve(map[string]string{"user": "ada", "pass": "secret"})
	if err != nil || r.Auth.Username != "ada" || r.Auth.Password != "secret" {
		t.Fatal("basic auth variables were not resolved")
	}
	_, err = (Request{Body: `{"value":"{{missing}}"}`}).Resolve(nil)
	if err == nil || !strings.Contains(err.Error(), "JSON body") {
		t.Fatal("missing variable must identify its field")
	}
}
