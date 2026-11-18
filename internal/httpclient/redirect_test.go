package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedirectTimelineAndStop(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Key") != "" || r.Header.Get("Authorization") != "" {
			t.Error("credentials reached another origin")
		}
		if r.Method != "POST" {
			t.Errorf("replayed method = %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "{}" {
			t.Errorf("replayed body = %s", body)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer destination.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL+"?token=abc", http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	for _, stop := range []bool{false, true} {
		template := Request{Method: "POST", URL: origin.URL, Body: "{}", StopAtRedirect: stop, Headers: []HeaderField{{Name: "X-Key", Value: "secret"}}, Auth: Auth{Type: "Bearer", Token: "secret"}}
		req, err := template.Build(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		client := *origin.Client()
		var hops []RedirectHop
		ConfigureRedirects(&client, template, false, func(h RedirectHop) { hops = append(hops, h) })
		resp, err := Execute(&client, req)
		if err != nil {
			t.Fatal(err)
		}
		want := "204 No Content"
		if stop {
			want = "307 Temporary Redirect"
		}
		if resp.Status != want || len(hops) != 1 || hops[0].Method != "POST" || strings.Contains(hops[0].Destination, "abc") {
			t.Fatalf("response %s hops %+v", resp.Status, hops)
		}
	}
}
func TestRedirectLoopIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/", http.StatusFound) }))
	defer server.Close()
	template := Request{URL: server.URL, Method: "GET"}
	req, _ := template.Build(context.Background())
	client := *server.Client()
	hops := 0
	ConfigureRedirects(&client, template, false, func(RedirectHop) { hops++ })
	if _, err := Execute(&client, req); err == nil || hops != 10 {
		t.Fatalf("loop: hops=%d err=%v", hops, err)
	}
}

func TestRedirectCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/next" {
			t.Error("sent a cancelled redirect")
			return
		}
		http.Redirect(w, r, "/next", http.StatusFound)
	}))
	defer server.Close()
	template := Request{URL: server.URL, Method: "GET"}
	req, _ := template.Build(ctx)
	client := *server.Client()
	ConfigureRedirects(&client, template, false, func(RedirectHop) { cancel() })
	if _, err := Execute(&client, req); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
