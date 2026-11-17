package main

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/hsm-gustavo/relay/internal/workspace"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func awaitUI(t *testing.T, queue <-chan func()) {
	t.Helper()
	select {
	case callback := <-queue:
		callback()
	case <-time.After(3 * time.Second):
		t.Fatal("request did not complete")
	}
}

func TestTabsRestoreAuthTimeoutAndSelection(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	view := newWorkspaceView(app.NewWindow("Relay"), &http.Client{}, store)
	first := view.active
	first.name.SetText("First")
	first.auth.SetSelected("Basic")
	first.username.SetText("ada")
	first.password.SetText("secret")
	first.timeout.SetText("2m")
	view.newRequest()
	second := view.active
	second.name.SetText("Second")
	second.auth.SetSelected("Bearer")
	second.token.SetText("sample-token")
	second.timeout.SetText("0")
	view.tabs.Select(first.tab)
	reopened, err := workspace.Open(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	restored := newWorkspaceView(app.NewWindow("Restored"), &http.Client{}, reopened)
	if len(restored.editors) != 2 || restored.active.name.Text != "First" {
		t.Fatal("tabs or selected tab were not restored")
	}
	if restored.active.password.Text != "secret" || restored.active.timeout.Text != "2m" {
		t.Fatal("basic auth or timeout was not restored")
	}
	restored.tabs.Select(restored.editors[1].tab)
	if restored.active.token.Text != "sample-token" || restored.active.timeout.Text != "0" {
		t.Fatal("bearer auth or disabled timeout was not restored")
	}
	restored.closeTab(restored.active.tab)
	restored.closeTab(restored.active.tab)
	if len(restored.editors) != 1 || restored.active.name.Text != "" {
		t.Fatal("closing the last tab must create an empty tab")
	}
}

func TestConcurrentTabsCancelAndCopyResponse(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	started := make(chan *http.Request, 2)
	release := make(chan struct{})
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		started <- r
		if r.URL.Path == "/cancel" {
			<-r.Context().Done()
			return nil, r.Context().Err()
		}
		select {
		case <-release:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
		return &http.Response{Status: "200 OK", StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
	})}
	view := newWorkspaceView(app.NewWindow("Relay"), client, store)
	queue := make(chan func(), 2)
	view.dispatch = func(fn func()) { queue <- fn }
	first := view.active
	first.url.SetText("http://example.com/cancel")
	first.auth.SetSelected("Basic")
	first.username.SetText("ada")
	first.password.SetText("secret")
	first.execute()
	req := <-started
	user, pass, ok := req.BasicAuth()
	if !ok || user != "ada" || pass != "secret" {
		t.Fatal("basic auth was not sent")
	}
	view.newRequest()
	second := view.active
	second.url.SetText("http://example.com/ok")
	second.auth.SetSelected("Bearer")
	second.token.SetText("token")
	second.execute()
	if req = <-started; req.Header.Get("Authorization") != "Bearer token" {
		t.Fatal("bearer auth was not sent")
	}
	if first.cancel.Disabled() || second.cancel.Disabled() {
		t.Fatal("both tabs must be independently cancellable")
	}
	first.cancel.OnTapped()
	awaitUI(t, queue)
	if first.draft.Error != "Request cancelled." || second.cancelRun == nil {
		t.Fatal("cancelling one tab affected another")
	}
	close(release)
	awaitUI(t, queue)
	if second.draft.Response == nil || first.draft.Response != nil {
		t.Fatal("response reached the wrong tab")
	}
	second.copyBody.OnTapped()
	if got := app.Clipboard().Content(); got != "{\n  \"ok\": true\n}" {
		t.Fatalf("formatted clipboard = %q", got)
	}
	second.format.SetSelected("Raw text")
	second.copyBody.OnTapped()
	if got := app.Clipboard().Content(); got != `{"ok":true}` {
		t.Fatalf("raw clipboard = %q", got)
	}
	if store.State.History[1].Error != "Request cancelled." || store.State.History[0].Response == nil {
		t.Fatal("results were not recorded in history")
	}
}

func TestTimeoutAndClosingRunningTab(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	view := newWorkspaceView(app.NewWindow("Relay"), client, store)
	queue := make(chan func(), 2)
	view.dispatch = func(fn func()) { queue <- fn }
	editor := view.active
	editor.url.SetText("example.com")
	editor.timeout.SetText("20ms")
	editor.execute()
	awaitUI(t, queue)
	if !strings.Contains(editor.draft.Error, "timed out") || editor.send.Disabled() || !editor.cancel.Disabled() {
		t.Fatal("timeout was not reported or controls were not reset")
	}
	editor.timeout.SetText("0")
	editor.execute()
	if editor.cancelRun == nil {
		t.Fatal("request did not start")
	}
	view.closeTab(editor.tab)
	awaitUI(t, queue)
	if len(view.editors) != 1 || view.active == editor || store.State.History[0].Error != "Request cancelled." {
		t.Fatal("closed tab was resurrected or its request was not cancelled")
	}
}

func TestValidationErrorsDoNotSend(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Error("invalid request was sent")
		return nil, context.Canceled
	})}
	view := newWorkspaceView(app.NewWindow("Relay"), client, store)
	editor := view.active
	editor.url.SetText("example.com")
	editor.timeout.SetText("bad")
	editor.execute()
	if !strings.Contains(editor.draft.Error, "timeout") || len(store.State.History) != 0 {
		t.Fatal("invalid timeout did not produce a validation error")
	}
	editor.timeout.SetText("30s")
	editor.auth.SetSelected("Bearer")
	editor.execute()
	if !strings.Contains(editor.draft.Error, "bearer token") || editor.cancelRun != nil {
		t.Fatal("empty token did not produce a validation error")
	}
}

func TestLegacyDraftMigratesToTab(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	store.State.Draft.Name = "Legacy draft"
	store.State.Draft.Request.URL = "localhost:8080/legacy"
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	reopened, err := workspace.Open(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	view := newWorkspaceView(app.NewWindow("Migrated"), &http.Client{}, reopened)
	if len(view.editors) != 1 || view.active.name.Text != "Legacy draft" || view.active.url.Text != "localhost:8080/legacy" {
		t.Fatal("legacy draft was lost")
	}
	if len(reopened.State.Tabs) != 1 || reopened.State.ActiveTabID == "" {
		t.Fatal("legacy draft was not migrated to a tab")
	}
}

func TestEditedRequestDoesNotReceiveStaleResult(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	release := make(chan struct{})
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		select {
		case <-release:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
		return &http.Response{Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader("original response"))}, nil
	})}
	view := newWorkspaceView(app.NewWindow("Relay"), client, store)
	queue := make(chan func(), 1)
	view.dispatch = func(fn func()) { queue <- fn }
	editor := view.active
	editor.url.SetText("example.com/original")
	editor.execute()
	editor.url.SetText("example.com/edited")
	close(release)
	awaitUI(t, queue)
	if editor.draft.Response != nil || editor.status.Text == "Sending…" {
		t.Fatal("edited request received a stale response or is stuck sending")
	}
	if store.State.History[0].Response == nil || store.State.History[0].Request.URL != "example.com/original" {
		t.Fatal("original response was not retained in history")
	}
}
