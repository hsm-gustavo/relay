package main

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/hsm-gustavo/relay/internal/workspace"
)

func TestSwitchEnvironmentWithoutChangingTemplate(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	local, _ := store.State.SaveEnvironment("", "Local", map[string]string{"base_url": "http://localhost:8080", "token": "local-token"})
	_, _ = store.State.SaveEnvironment("", "Production", map[string]string{"base_url": "https://api.example.com", "token": "production-token"})
	store.State.ActiveEnvironmentID = local
	requests := make(chan *http.Request, 2)
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests <- req
		return &http.Response{Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
	})}
	view := newWorkspaceView(app.NewWindow("Relay"), client, store)
	queue := make(chan func(), 2)
	view.dispatch = func(fn func()) { queue <- fn }
	editor := view.active
	editor.url.SetText("{{base_url}}/users")
	editor.auth.SetSelected("Bearer")
	editor.token.SetText("{{token}}")
	editor.execute()
	awaitUI(t, queue)
	req := <-requests
	if req.URL.String() != "http://localhost:8080/users" || req.Header.Get("Authorization") != "Bearer local-token" {
		t.Fatal("local environment was not sent")
	}
	view.environments.SetSelected("Production")
	editor.execute()
	awaitUI(t, queue)
	req = <-requests
	if req.URL.String() != "https://api.example.com/users" || req.Header.Get("Authorization") != "Bearer production-token" {
		t.Fatal("production environment was not sent")
	}
	if editor.url.Text != "{{base_url}}/users" || store.State.History[0].Request.URL != "{{base_url}}/users" || store.State.History[0].EnvironmentName != "Production" || store.State.History[1].EnvironmentName != "Local" {
		t.Fatal("templates or history environment snapshots changed")
	}
	reopened, err := workspace.Open(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	restored := newWorkspaceView(app.NewWindow("Restored"), client, reopened)
	if restored.environments.Selected != "Production" || restored.active.url.Text != "{{base_url}}/users" {
		t.Fatal("environment or template was not restored")
	}
	restored.environments.SetSelected(noEnvironment)
	restored.active.execute()
	if !strings.Contains(restored.active.draft.Error, "base_url") || len(reopened.State.History) != 2 {
		t.Fatal("undefined variable did not prevent execution")
	}
}

func TestRunningRequestKeepsEnvironmentSnapshot(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	local, _ := store.State.SaveEnvironment("", "Local", map[string]string{"base_url": "http://localhost:8080"})
	_, _ = store.State.SaveEnvironment("", "Production", map[string]string{"base_url": "https://api.example.com"})
	store.State.ActiveEnvironmentID = local
	release := make(chan struct{})
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		select {
		case <-release:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
		return &http.Response{Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader("local response"))}, nil
	})}
	view := newWorkspaceView(app.NewWindow("Relay"), client, store)
	queue := make(chan func(), 1)
	view.dispatch = func(fn func()) { queue <- fn }
	view.active.url.SetText("{{base_url}}/users")
	view.active.execute()
	view.environments.SetSelected("Production")
	close(release)
	awaitUI(t, queue)
	if view.active.draft.EnvironmentName != "Local" || store.State.History[0].ResolvedRequest.URL != "http://localhost:8080/users" {
		t.Fatal("switching environments changed an in-flight request")
	}
}
