package main

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/hsm-gustavo/relay/internal/extraction"
	"github.com/hsm-gustavo/relay/internal/workspace"
)

func TestLoginExtractionFeedsNextRequestWithoutPersistence(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	local, _ := store.State.SaveEnvironment("", "Local", map[string]string{"base_url": "http://localhost:8080", "token": "manual"})
	store.State.ActiveEnvironmentID = local
	secret := "LOGIN-EXTRACTION-SECRET-123"
	sent := make(chan *http.Request, 2)
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		sent <- req
		body := `{"access_token":"` + secret + `","user":{"id":9007199254740993}}`
		if req.URL.Path != "/auth/login" {
			body = `{"echo":"` + secret + `"}`
		}
		return &http.Response{Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	view := newWorkspaceView(app.NewWindow("Relay"), client, store)
	queue := make(chan func(), 2)
	view.dispatch = func(fn func()) { queue <- fn }
	login := view.active
	login.url.SetText("{{base_url}}/auth/login")
	login.method.SetSelected("POST")
	login.extractRules = []extraction.Rule{{Path: "$.access_token", Variable: "token"}, {Path: "$.user.id", Variable: "user_id"}, {Path: "$.missing", Variable: "other"}}
	login.changed(false)
	login.execute()
	awaitUI(t, queue)
	<-sent
	if login.draft.Response == nil || login.extractResults[0].Status != extraction.Success || login.extractResults[2].Status != extraction.Failed {
		t.Fatal("extraction failed or prevented response viewing")
	}
	view.newRequest()
	next := view.active
	next.url.SetText("{{base_url}}/users/{{user_id}}")
	next.auth.SetSelected("Bearer")
	next.token.SetText("{{token}}")
	next.execute()
	awaitUI(t, queue)
	request := <-sent
	if request.URL.Path != "/users/9007199254740993" || request.Header.Get("Authorization") != "Bearer "+secret {
		t.Fatal("next request did not use extracted variables")
	}
	if !next.draft.SessionOnly || !store.State.History[0].SessionOnly {
		t.Fatal("session-derived request was not marked volatile")
	}
	if store.State.ActiveEnvironment().Variables["token"] != "manual" {
		t.Fatal("manual environment value was overwritten")
	}
	err := filepath.WalkDir(filepath.Dir(store.Path), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if bytes.Contains(data, []byte(secret)) {
			t.Errorf("secret leaked into %s", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := workspace.Open(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	restored := newWorkspaceView(app.NewWindow("Restored"), client, reopened)
	if len(restored.editors[0].extractRules) != 3 || restored.editors[0].draft.Response != nil {
		t.Fatal("rules were lost or volatile response persisted")
	}
	restored.active.execute()
	if !strings.Contains(restored.active.draft.Error, "user_id") {
		t.Fatal("extracted session values survived restart")
	}
}

func TestConcurrentExtractionTargetsSendingEnvironment(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	local, _ := store.State.SaveEnvironment("", "Local", map[string]string{"base_url": "http://local.example"})
	production, _ := store.State.SaveEnvironment("", "Production", map[string]string{"base_url": "https://production.example"})
	store.State.ActiveEnvironmentID = local
	releaseLocal, releaseProd := make(chan struct{}), make(chan struct{})
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		release := releaseLocal
		token := "local-token"
		if req.URL.Host == "production.example" {
			release = releaseProd
			token = "production-token"
		}
		select {
		case <-release:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
		return &http.Response{Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"token":"` + token + `"}`))}, nil
	})}
	view := newWorkspaceView(app.NewWindow("Relay"), client, store)
	queue := make(chan func(), 2)
	view.dispatch = func(fn func()) { queue <- fn }
	view.active.url.SetText("{{base_url}}/login")
	view.active.extractRules = []extraction.Rule{{Path: "$.token", Variable: "token"}}
	view.active.changed(false)
	view.active.execute()
	view.environments.SetSelected("Production")
	view.newRequest()
	view.active.url.SetText("{{base_url}}/login")
	view.active.extractRules = []extraction.Rule{{Path: "$.token", Variable: "token"}}
	view.active.changed(false)
	view.active.execute()
	close(releaseProd)
	awaitUI(t, queue)
	close(releaseLocal)
	awaitUI(t, queue)
	for _, tt := range []struct{ id, want string }{{local, "local-token"}, {production, "production-token"}} {
		store.State.ActiveEnvironmentID = tt.id
		request := view.active.draft.Request
		request.Auth.Type = "Bearer"
		request.Auth.Token = "{{token}}"
		resolved, _, err := view.session.Resolve(store.State.ActiveEnvironment(), request)
		if err != nil || resolved.Auth.Token != tt.want {
			t.Fatalf("wrong session for %s", tt.id)
		}
	}
}

func TestExtractionRuleEditingAndDisabledPersistence(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	view := newWorkspaceView(app.NewWindow("Relay"), &http.Client{}, store)
	editor := view.active
	editor.extractRules = []extraction.Rule{{Path: "$.token", Variable: "token"}, {Path: "$.id", Variable: "id", Disabled: true}}
	editor.changed(false)
	editor.refreshExtractionRules()
	if len(editor.rulesBox.Objects) != 2 {
		t.Fatal("rules were not displayed")
	}
	// Editing a reopened draft must not mutate its saved request/history rule slice.
	original := editor.draft.Request.Extract
	editor.extractRules[0].Path = "$.access_token"
	editor.changed(false)
	if original[0].Path != "$.token" || !editor.draft.Request.Extract[1].Disabled {
		t.Fatal("editing rules mutated original snapshot or lost disabled state")
	}
	editor.extractRules = editor.extractRules[1:]
	editor.changed(false)
	editor.refreshExtractionRules()
	if len(editor.rulesBox.Objects) != 1 || len(store.State.Draft.Request.Extract) != 1 {
		t.Fatal("rule removal was not saved")
	}
}
