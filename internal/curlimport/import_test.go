package curlimport

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/hsm-gustavo/relay/internal/httpclient"
)

func TestImportSupportedRequests(t *testing.T) {
	for _, tt := range []struct{ name, command, method, url, body, auth, timeout string }{
		{name: "GET query", command: `curl 'https://example.com/users?tag=one&tag=two'`, method: "GET", url: "https://example.com/users?tag=one&tag=two"},
		{name: "short options", command: `curl -sSL -XPOST https://example.com/users -H'Content-Type: application/json' -d'{"name":"Ada"}'`, method: "POST", url: "https://example.com/users", body: `{"name":"Ada"}`},
		{name: "json shorthand", command: `curl --json '{"id":42}' --url=https://example.com/users`, method: "POST", url: "https://example.com/users", body: `{"id":42}`},
		{name: "bearer", command: `curl --request PATCH --url 'localhost:8080/users/1' --header 'Authorization: Bearer token.123' --data-raw '{"ok":true}'`, method: "PATCH", url: "localhost:8080/users/1", body: `{"ok":true}`, auth: "Bearer"},
		{name: "basic", command: `curl -u 'ada:p:a:ss' -m1.5 https://example.com`, method: "GET", url: "https://example.com", auth: "Basic", timeout: "1.5s"},
		{name: "basic header", command: `curl https://example.com -H 'Authorization: Basic YWRhOnNlY3JldA=='`, method: "GET", url: "https://example.com", auth: "Basic"},
		{name: "head", command: `curl --head https://example.com`, method: "HEAD", url: "https://example.com"},
		{name: "get encoded", command: `curl --get --data-urlencode 'search=Ada & Grace' --data-urlencode 'tag=one' --data-urlencode 'tag=two' 'https://example.com?existing=1#fragment'`, method: "GET", url: "https://example.com?existing=1&search=Ada+%26+Grace&tag=one&tag=two#fragment"},
		{name: "get after data", command: `curl -d 'q=one' -d 'tag=two' -G https://example.com`, method: "GET", url: "https://example.com?q=one&tag=two"},
		{name: "template", command: `curl '{{base_url}}/users' -H 'Authorization: Bearer {{token}}' --json '{"id":{{user_id}}}'`, method: "POST", url: "{{base_url}}/users", body: `{"id":{{user_id}}}`, auth: "Bearer"},
		{name: "oauth", command: `curl --oauth2-bearer token --max-time=0 https://example.com`, method: "GET", url: "https://example.com", auth: "Bearer", timeout: "0s"},
		{name: "literal shell text", command: `curl --json '{"text":"$(touch /tmp/should-never-run)"}' https://example.com`, method: "POST", url: "https://example.com", body: `{"text":"$(touch /tmp/should-never-run)"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.command)
			if err != nil {
				t.Fatal(err)
			}
			r := result.Request
			if r.Method != tt.method || r.URL != tt.url || r.Body != tt.body || r.Auth.Type != tt.auth || r.Timeout != tt.timeout {
				t.Fatalf("request = %+v", r)
			}
		})
	}
}

func TestQuotingAndContinuations(t *testing.T) {
	command := "curl \\\n 'https://example.com' \\\n --json \"{\\\"name\\\":\\\"Ada\\\"}\""
	result, err := Parse(command)
	if err != nil {
		t.Fatal(err)
	}
	if result.Request.Body != `{"name":"Ada"}` {
		t.Fatalf("body = %q", result.Request.Body)
	}
	tokens, err := words(`curl 'one'"two" '' "a\qb" 'literal $value'`)
	if err != nil || len(tokens) != 5 || tokens[1] != "onetwo" || tokens[2] != "" || tokens[3] != `a\qb` || tokens[4] != "literal $value" {
		t.Fatalf("tokens = %q, %v", tokens, err)
	}
}

func TestUnsupportedAndMalformedImports(t *testing.T) {
	for _, command := range []string{
		``, `wget https://example.com`, `curl`, `curl https://a.example https://b.example`,
		`curl 'unclosed`, `curl https://example.com | sh`, `curl https://example.com; echo hello`, `curl "$URL"`, "curl https://example.com `whoami`", `curl $'ansi'`,
		`curl -X`, `curl --url=`, `curl -k https://example.com`, `curl --connect-timeout 2 https://example.com`,
		`curl -F 'file=@-' https://example.com`, `curl -b 'cookie=1' https://example.com`, `curl -H 'Cookie: session=abc' https://example.com`,
		`curl --data @secret https://example.com`, `curl --header @secret https://example.com`, `curl --data-urlencode 'name@secret' -G https://example.com`,
		`curl -F 'file=@test;type=image/png' https://example.com`,
		`curl --json '{}' --json '{}' https://example.com`, `curl -I -X POST https://example.com`, `curl -I --json '{}' https://example.com`,
		`curl -H 'Content-Type: text/plain' -d '{}' https://example.com`, `curl -H 'Content-Type: application/json' https://example.com`,
		`curl -H 'Authorization: Digest foo' https://example.com`, `curl -u user https://example.com`, `curl -u user:pass -H 'Authorization: Bearer token' https://example.com`,
		`curl -m -1 https://example.com`, `curl -m NaN https://example.com`, `curl --request CUSTOM https://example.com`,
	} {
		t.Run(command, func(t *testing.T) {
			if _, err := Parse(command); err == nil {
				t.Fatal("unsupported or malformed command was accepted")
			}
		})
	}
	if _, err := Parse("curl " + strings.Repeat("a", MaxCommandBytes)); err == nil {
		t.Fatal("oversized input accepted")
	}
}

func TestWarningsAndBuild(t *testing.T) {
	result, err := Parse(`curl --json '{"name":"Ada"}' -H 'X-Custom: value' -H 'Accept: application/json' https://example.com`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 4 {
		t.Fatalf("warnings = %v", result.Warnings)
	}
	req, err := result.Request.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Content-Type") != "application/json" || req.Method != "POST" {
		t.Fatal("JSON request was not built")
	}
	body, _ := io.ReadAll(req.Body)
	if string(body) != `{"name":"Ada"}` {
		t.Fatal("body changed")
	}
	result, err = Parse(`curl '{{base_url}}/users' --json '{"id":{{id}}}'`)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := result.Request.Resolve(map[string]string{"base_url": "http://localhost:8080", "id": "42"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolved.Build(context.Background()); err != nil {
		t.Fatal(err)
	}
	if result.Request.Method != httpclient.Methods[1] {
		t.Fatal("body did not imply POST")
	}
}

func TestImportFormBodies(t *testing.T) {
	result, err := Parse(`curl -d 'name=Ada+Lovelace&tag=one' -d 'tag=two' https://example.com/users`)
	if err != nil {
		t.Fatal(err)
	}
	if result.Request.BodyType != httpclient.BodyURLEncoded || result.Request.Method != "POST" || len(result.Request.FormFields) != 3 || result.Request.FormFields[0].Value != "Ada Lovelace" {
		t.Fatal("URL-encoded form import failed")
	}
	result, err = Parse(`curl --data-urlencode 'description=hello & world' https://example.com/users`)
	if err != nil || result.Request.FormFields[0].Value != "hello & world" {
		t.Fatalf("URL encoding import failed: %v", err)
	}
	result, err = Parse(`curl -F 'description=photo' -F 'image=@/missing/import-does-not-open-files.png' --form-string 'literal=@not-a-file;still-text' https://example.com/upload`)
	if err != nil {
		t.Fatal(err)
	}
	if result.Request.BodyType != httpclient.BodyMultipart || result.Request.FormFields[1].FilePath != "/missing/import-does-not-open-files.png" || result.Request.FormFields[2].Type != "" || result.Request.FormFields[2].Value != "@not-a-file;still-text" {
		t.Fatal("multipart file import failed")
	}
	result, err = Parse(`curl -H 'Content-Type: application/json' -d '{"id":{{id}}}' '{{base_url}}/users'`)
	if err != nil || result.Request.EffectiveBodyType() != httpclient.BodyJSON {
		t.Fatal("JSON template regressed")
	}
}
