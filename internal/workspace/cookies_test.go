package workspace

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
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

func TestAddManualCookie(t *testing.T) {
	var sessions SessionCookies
	parse := func(raw string) *url.URL {
		t.Helper()
		target, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		return target
	}
	target := parse("https://api.example.test/login")
	const secret = "manual-cookie-value"
	cookie := http.Cookie{Name: "sid", Value: secret, Path: "/api", Secure: true, Expires: time.Now().Add(time.Hour)}
	if err := sessions.Add("local", target, cookie); err != nil {
		t.Fatal(err)
	}
	matches := sessions.ForRequest("local").Cookies(parse("https://api.example.test/api/users"))
	if len(matches) != 1 || matches[0].Name != "sid" || matches[0].Value != secret {
		t.Fatalf("manual cookie was not selected: %d matches", len(matches))
	}
	for _, probe := range []struct{ id, raw string }{
		{"prod", "https://api.example.test/api/users"},
		{"", "https://api.example.test/api/users"},
		{"local", "https://sub.api.example.test/api/users"},
		{"local", "http://api.example.test/api/users"},
		{"local", "https://api.example.test/login"},
	} {
		if len(sessions.Names(probe.id, parse(probe.raw))) != 0 {
			t.Errorf("cookie escaped scope: %+v", probe)
		}
	}
	sessions.Clear("local")
	if len(sessions.Names("local", parse("https://api.example.test/api/users"))) != 0 {
		t.Fatal("manual cookie survived clearing the jar")
	}
}

func TestAddRejectsInvalidCookies(t *testing.T) {
	var sessions SessionCookies
	https, _ := url.Parse("https://api.example.test")
	httpURL, _ := url.Parse("http://api.example.test")
	for _, invalid := range []struct {
		target *url.URL
		cookie http.Cookie
	}{
		{nil, http.Cookie{Name: "sid", Path: "/"}},
		{https, http.Cookie{Name: "bad name", Path: "/"}},
		{https, http.Cookie{Name: "sid", Value: "bad\nvalue", Path: "/"}},
		{https, http.Cookie{Name: "sid", Path: "relative"}},
		{https, http.Cookie{Name: "sid", Path: "/", Domain: "example.test"}},
		{https, http.Cookie{Name: "sid", Path: "/", Expires: time.Now().Add(-time.Hour)}},
		{httpURL, http.Cookie{Name: "sid", Path: "/", Secure: true}},
	} {
		if err := sessions.Add("", invalid.target, invalid.cookie); err == nil {
			t.Error("accepted an invalid cookie")
		}
	}
	if len(sessions.Names("", https)) != 0 {
		t.Fatal("invalid cookie reached the jar")
	}
}
