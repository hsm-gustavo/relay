package main

import (
	"net/http"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/hsm-gustavo/relay/internal/workspace"
)

func TestCurlImportOpensPersistentTabWithoutSending(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { t.Error("import sent a request"); return nil, nil })}
	view := newWorkspaceView(app.NewWindow("Relay"), client, store)
	original := view.active
	original.url.SetText("example.com/original")
	err := view.importCurl(`curl --json '{"id":{{id}}}' -H 'Authorization: Bearer {{token}}' '{{base_url}}/users'`, "Create user")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.editors) != 2 || view.active == original || view.active.method.Selected != "POST" || view.active.url.Text != "{{base_url}}/users" || view.active.body.Text != `{"id":{{id}}}` || view.active.token.Text != "{{token}}" {
		t.Fatal("imported editor did not match request")
	}
	if len(store.State.History) != 0 {
		t.Fatal("import should not create sent history")
	}
	reopened, err := workspace.Open(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.State.Draft.Name != "Create user" || reopened.State.Draft.Request.URL != "{{base_url}}/users" {
		t.Fatal("import was not persisted")
	}
	if err := view.importCurl(`curl -k https://example.com`, "Invalid"); err == nil || len(view.editors) != 2 {
		t.Fatal("invalid import modified tabs")
	}
}
