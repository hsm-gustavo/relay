package main

import (
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/hsm-gustavo/relay/internal/httpclient"
	"github.com/hsm-gustavo/relay/internal/workspace"
)

func TestFormBodiesPersistAndResolveAcrossEditor(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	env, _ := store.State.SaveEnvironment("", "Local", map[string]string{"description": "hello upload"})
	store.State.ActiveEnvironmentID = env
	collection, _ := store.State.AddCollection("Uploads")
	filePath := filepath.Join(t.TempDir(), "document.txt")
	if err := os.WriteFile(filePath, []byte("document contents"), 0600); err != nil {
		t.Fatal(err)
	}
	sent := make(chan *http.Request, 1)
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
		if err != nil {
			return nil, err
		}
		form, err := multipart.NewReader(req.Body, params["boundary"]).ReadForm(32 << 10)
		if err != nil {
			return nil, err
		}
		defer form.RemoveAll()
		defer req.Body.Close()
		if form.Value["description"][0] != "hello upload" || form.File["file"][0].Filename != "document.txt" {
			t.Error("form fields were not sent")
		}
		sent <- req
		return &http.Response{Status: "201 Created", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
	})}
	view := newWorkspaceView(app.NewWindow("Relay"), client, store)
	queue := make(chan func(), 1)
	view.dispatch = func(fn func()) { queue <- fn }
	editor := view.active
	editor.name.SetText("Upload document")
	editor.url.SetText("example.com/upload")
	editor.body.SetText("{{inactive_json}}")
	editor.formFields = []httpclient.FormField{{ID: "description", Name: "description", Value: "{{description}}"}, {ID: "file", Name: "file", Type: "file"}}
	editor.bodyType.SetSelected(httpclient.BodyMultipart)
	editor.setUploadFile("file", filePath)
	if editor.formFields[1].FilePath != filePath {
		t.Fatal("file selection was not applied")
	}
	view.selectedCollection = collection
	view.saveRequest()
	editor.execute()
	awaitUI(t, queue)
	<-sent
	if editor.draft.Response == nil || editor.draft.Response.Status != "201 Created" {
		t.Fatal("upload response missing")
	}
	reopened, err := workspace.Open(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	restored := newWorkspaceView(app.NewWindow("Restored"), client, reopened)
	if restored.active.bodyType.Selected != httpclient.BodyMultipart || restored.active.formFields[1].FilePath != filePath || reopened.State.Collections[0].Requests[0].Request.FormFields[0].Value != "{{description}}" {
		t.Fatal("body type, file path or saved template was not restored")
	}
	restored.active.setUploadFile("deleted-row", "wrong")
	if restored.active.formFields[1].FilePath != filePath {
		t.Fatal("stale file callback changed a different field")
	}
	restored.active.bodyType.SetSelected(httpclient.BodyURLEncoded)
	restored.active.formFields[1].Disabled = true
	restored.active.changed(true)
	if !restored.active.draft.Request.FormFields[1].Disabled {
		t.Fatal("disabled form field was not saved")
	}
}
