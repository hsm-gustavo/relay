package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/hsm-gustavo/relay/internal/workspace"
)

func TestSessionCookieResponsesStayOffDisk(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	dir := t.TempDir()
	store, err := workspace.Open(filepath.Join(dir, "workspace.json"))
	if err != nil {
		t.Fatal(err)
	}
	const token = "relay-cookie-dummy-secret"
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		headers := make(http.Header)
		body := "login"
		if req.URL.Path == "/login" {
			headers.Set("Set-Cookie", "sid="+token+"; Path=/")
		} else if cookie, err := req.Cookie("sid"); err == nil {
			body = cookie.Value
		}
		return &http.Response{Status: "200 OK", StatusCode: 200, Header: headers, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	view := newWorkspaceView(app.NewWindow("Relay"), client, store)
	queue := make(chan func(), 2)
	view.dispatch = func(f func()) { queue <- f }
	for _, path := range []string{"/login", "/me"} {
		view.active.url.SetText("https://example.test" + path)
		view.active.execute()
		awaitUI(t, queue)
		if !view.active.draft.SessionOnly || !store.State.History[0].SessionOnly || view.active.draft.Response == nil {
			t.Fatalf("private response unavailable in live view for %s", path)
		}
	}
	if got := string(view.active.draft.Response.Body); got != token {
		t.Fatal("cookie was not reused")
	}
	reopened, err := workspace.Open(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.State.Draft.Response != nil || reopened.State.History[0].Response != nil || reopened.State.History[1].Response != nil {
		t.Fatal("private response was persisted")
	}
	data, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), token) {
		t.Fatal("cookie value was persisted")
	}
	bodies, err := filepath.Glob(filepath.Join(dir, "bodies", "*.bin"))
	if err != nil || len(bodies) != 0 {
		t.Fatalf("unexpected saved bodies: %v %v", bodies, err)
	}
}
