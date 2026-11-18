package httpclient

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestCustomHeaders(t *testing.T) {
	r := Request{Method: "POST", URL: "https://example.test?key=hidden", Body: "{}", Headers: []HeaderField{
		{Name: "X-Key", Value: "{{key}}"}, {Name: "x-key", Value: "second"},
		{Name: "Content-Type", Value: "application/problem+json"}, {Name: "Ignored", Value: "{{missing}}", Disabled: true},
	}}
	resolved, used, err := r.ResolveTracked(map[string]string{"key": "hidden"})
	if err != nil || !used["key"] || r.Headers[0].Value != "{{key}}" {
		t.Fatalf("resolution failed: %v", err)
	}
	req, err := resolved.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer req.Body.Close()
	if len(req.Header.Values("X-Key")) != 2 || req.Header.Get("Content-Type") != "application/problem+json" {
		t.Fatal(req.Header)
	}
	if text := InspectRequest(req, false, resolved.Headers); strings.Contains(text, "hidden") || strings.Contains(text, "second") {
		t.Fatal("inspector leaked values")
	}
	for _, row := range []HeaderField{{Name: "bad name"}, {Name: "Accept", Value: "a\nb"}, {Name: "Host"}, {Name: "Authorization", Value: "custom"}} {
		invalid := Request{Method: "GET", URL: r.URL, Auth: Auth{Type: "Bearer", Token: "abc"}, Headers: []HeaderField{row}}
		if _, err := invalid.Build(context.Background()); err == nil {
			t.Fatalf("accepted %s", row.Name)
		}
	}
}
func TestRedirectHeaderGuard(t *testing.T) {
	client := &http.Client{}
	GuardRedirect(client, []HeaderField{{Name: "X-Key"}})
	first, _ := http.NewRequest("GET", "https://example.test", nil)
	next, _ := http.NewRequest("GET", "https://example.test:8443", nil)
	next.Header.Set("X-Key", "sensitive")
	next.Header.Set("Authorization", "sensitive")
	if err := client.CheckRedirect(next, []*http.Request{first}); err != nil || len(next.Header) != 0 {
		t.Fatal("redirect forwarded credentials", err)
	}
}
