package workspace

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCookieScopeAndClear(t *testing.T) {
	var sessions SessionCookies
	u, _ := url.Parse("https://app.example.com/api/login")
	jar := sessions.ForRequest("local")
	jar.SetCookies(u, []*http.Cookie{
		{Name: "session", Value: "secret", Path: "/api", Secure: true},
		{Name: "expired", Value: "expired", Path: "/", MaxAge: -1},
		{Name: "rejected", Value: "invalid", Path: "/", Domain: "com"},
	})
	probes := []struct {
		env, address string
		want         int
	}{
		{"local", "https://app.example.com/api/items", 1},
		{"local", "https://app.example.com/other", 0},
		{"local", "http://app.example.com/api/items", 0},
		{"local", "https://else.example.com/api/items", 0},
		{"production", "https://app.example.com/api/items", 0},
		{"", "https://app.example.com/api/items", 0},
	}
	for _, probe := range probes {
		u, _ := url.Parse(probe.address)
		if names := sessions.Names(probe.env, u); len(names) != probe.want {
			t.Errorf("%s %s: %v", probe.env, probe.address, names)
		}
	}
	sessions.Clear("local")
	jar.SetCookies(u, []*http.Cookie{{Name: "late", Value: "secret", Path: "/"}})
	if len(sessions.Names("local", u)) != 0 {
		t.Fatal("in-flight request repopulated a cleared jar")
	}
}

func TestCookieLoginAndRedirectReuse(t *testing.T) {
	var sessions SessionCookies
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "login", Value: "session-secret", Path: "/"})
			http.Redirect(w, r, "/profile", http.StatusFound)
			return
		}
		if c, err := r.Cookie("login"); err == nil && c.Value == "session-secret" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	for _, step := range []struct {
		env, path string
		status    int
	}{
		{"local", "/login", 204}, {"local", "/profile", 204}, {"production", "/profile", 401}, {"", "/profile", 401},
	} {
		jar := sessions.ForRequest(step.env)
		client := *server.Client()
		client.Jar = jar
		resp, err := client.Get(server.URL + step.path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != step.status || jar.Used() != strings.EqualFold(step.env, "local") {
			t.Errorf("step %+v: status=%d cookie use=%v", step, resp.StatusCode, jar.Used())
		}
	}
}
