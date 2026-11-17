package main

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/hsm-gustavo/relay/internal/httpclient"
	"github.com/hsm-gustavo/relay/internal/workspace"
)

func TestEditorSavesAndRestoresDraftAndCollection(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	path := filepath.Join(t.TempDir(), "workspace.json")
	store, err := workspace.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	collection, _ := store.State.AddCollection("Local API")
	view := newWorkspaceView(app.NewWindow("Relay"), &http.Client{}, store)
	view.active.name.SetText("Create user")
	view.active.method.SetSelected("POST")
	view.active.url.SetText("localhost:8080/users")
	view.active.params.SetText("active=true")
	view.active.body.SetText(`{"name":"Ada"}`)
	view.saveRequest()
	if len(store.State.Collections[0].Requests) != 1 {
		t.Fatal("request was not saved")
	}
	view.active.body.SetText(`{"name":"Grace"}`) // Unsaved collection edit must still survive as a draft.
	reopened, err := workspace.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	restored := newWorkspaceView(app.NewWindow("Reopened"), &http.Client{}, reopened)
	if restored.active.name.Text != "Create user" || restored.active.method.Selected != "POST" || restored.active.params.Text != "active=true" || restored.active.body.Text != `{"name":"Grace"}` {
		t.Fatal("editor draft was not restored")
	}
	if reopened.State.Collections[0].Requests[0].Request.Body != `{"name":"Ada"}` {
		t.Fatal("draft edit should not change saved request until Save")
	}
	restored.saveRequest()
	if len(reopened.State.Collections[0].Requests) != 1 || reopened.State.Collections[0].Requests[0].Request.Body != `{"name":"Grace"}` {
		t.Fatal("Save must update the original request")
	}
	restored.newRequest()
	restored.savedList.Select(0)
	if restored.active.body.Text != `{"name":"Grace"}` || reopened.State.Draft.CollectionID != collection {
		t.Fatal("saved request did not reopen")
	}
}

func TestHistoryReopenEditAndSave(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	_, _ = store.State.AddCollection("API")
	item := workspace.Item{Name: "Fetch", Request: httpclient.Request{Method: "GET", URL: "example.com"}}
	id := store.State.AddHistory(item)
	result := &httpclient.Response{Status: "200 OK", Duration: time.Millisecond, Headers: http.Header{"Content-Type": {"application/json"}}, Body: []byte(`{"ok":true}`)}
	store.State.CompleteHistory(id, result, "")
	view := newWorkspaceView(app.NewWindow("Relay"), &http.Client{}, store)
	view.historyList.Select(0)
	if view.active.url.Text != item.Request.URL || view.active.status.Text != "Status: 200 OK" || view.active.responseBody.Text() != "{\n  \"ok\": true\n}" {
		t.Fatal("history request and response did not reopen")
	}
	view.active.format.SetSelected("Raw")
	if view.active.responseBody.Text() != `{"ok":true}` {
		t.Fatal("raw response changed")
	}
	view.active.url.SetText("example.com/edited")
	if view.active.responseBody.Text() != "" {
		t.Fatal("editing request must clear stale response")
	}
	view.saveRequest()
	if store.State.Collections[0].Requests[0].Request.URL != "example.com/edited" {
		t.Fatal("edited history was not saved to collection")
	}
	if store.State.History[0].Request.URL != "example.com" {
		t.Fatal("editing history draft changed original snapshot")
	}
	reopened, err := workspace.Open(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.State.History[0].Response.Status != "200 OK" || !reopened.State.Draft.Raw {
		t.Fatal("history result or view preference was not persisted")
	}
}
