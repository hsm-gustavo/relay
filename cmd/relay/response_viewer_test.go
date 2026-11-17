package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"github.com/hsm-gustavo/relay/internal/httpclient"
	"github.com/hsm-gustavo/relay/internal/responseview"
	"github.com/hsm-gustavo/relay/internal/workspace"
)

func TestViewerTreeSearchCopyAndMetrics(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	view := newWorkspaceView(app.NewWindow("Relay"), &http.Client{}, store)
	view.window.SetContent(view.content)
	editor := view.active
	body := `{"user":{"id":9007199254740993,"name":"Ada"},"roles":["admin","editor"],"other":"ADA"}`
	editor.draft.Response = &httpclient.Response{Status: "201 Created", Duration: 23 * time.Millisecond, Headers: http.Header{"Content-Type": {"application/problem+json"}}, Body: []byte(body)}
	editor.showResult()
	viewer := editor.viewer
	if viewer.model.Kind != responseview.JSON || !viewer.tree.IsBranchOpen("0") || !viewer.tree.IsBranch(viewer.tree.Root) || len(viewer.tree.ChildUIDs(viewer.tree.Root)) != 1 {
		t.Fatal("JSON tree was not prepared")
	}
	if editor.size.Text != fmt.Sprintf("Body size: %d bytes", len(body)) {
		t.Fatal("body size semantics changed")
	}
	if editor.duration.Text != "Time: 23.0 ms" || editor.status.Text != "Status: 201 Created" || !strings.Contains(editor.contentType.Text, "application/problem+json") {
		t.Fatal("response metrics changed")
	}
	userID := viewer.tree.ChildUIDs("0")[0]
	viewer.tree.OpenBranch(userID)
	if !viewer.tree.IsBranchOpen(userID) || len(viewer.tree.ChildUIDs(userID)) != 2 {
		t.Fatal("nested object did not expand")
	}
	viewer.tree.CloseBranch(userID)
	if viewer.tree.IsBranchOpen(userID) {
		t.Fatal("nested object did not collapse")
	}
	editor.format.SetSelected(responseview.Tree)
	if viewer.search.Disabled() == false || !viewer.tree.Visible() {
		t.Fatal("Tree did not disable text search")
	}
	viewer.focusSearch()
	if editor.format.Selected != responseview.Raw || view.window.Canvas().Focused() != viewer.search {
		t.Fatalf("Find mode=%s focused=%T disabled=%v", editor.format.Selected, view.window.Canvas().Focused(), viewer.search.Disabled())
	}
	viewer.search.SetText("ada")
	if viewer.count.Text != "1/2" || viewer.current != 0 {
		t.Fatalf("search count = %s", viewer.count.Text)
	}
	viewer.next.OnTapped()
	if viewer.current != 1 {
		t.Fatal("next match did not advance")
	}
	viewer.previous.OnTapped()
	if viewer.current != 0 {
		t.Fatal("previous match did not move")
	}
	viewer.search.SetText(strings.Repeat("x", responseview.MaxQueryRunes+1))
	if viewer.search.Text != "ada" || !strings.Contains(editor.previewNote.Text, "256") {
		t.Fatal("oversized search input was not rejected")
	}
	highlighted := false
	for _, row := range editor.responseBody.Rows {
		for _, cell := range row.Cells {
			if _, ok := cell.Style.(matchStyle); ok {
				highlighted = true
			}
		}
	}
	if !highlighted {
		t.Fatal("current match was not highlighted")
	}
	editor.copyBody.OnTapped()
	if app.Clipboard().Content() != body {
		t.Fatal("Raw copy changed original text")
	}
	editor.format.SetSelected(responseview.Pretty)
	editor.copyBody.OnTapped()
	if app.Clipboard().Content() != viewer.model.PrettyText {
		t.Fatal("Pretty copy returned wrong text")
	}
	if string(editor.draft.Response.Body) != body {
		t.Fatal("viewer changed response data")
	}
}

func TestStaleResponsePreparationCannotReplaceNewResponse(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	view := newWorkspaceView(app.NewWindow("Relay"), &http.Client{}, store)
	queue := make(chan func(), 2)
	view.dispatch = func(fn func()) { queue <- fn }
	editor := view.active
	editor.draft.Response = &httpclient.Response{Status: "200 OK", Headers: http.Header{"Content-Type": {"text/plain"}}, Body: []byte(strings.Repeat("old ", 10000))}
	editor.showResult()
	var oldCallback func()
	select {
	case oldCallback = <-queue:
	case <-time.After(3 * time.Second):
		t.Fatal("preparation did not finish")
	}
	editor.draft.Response = &httpclient.Response{Status: "200 OK", Headers: http.Header{"Content-Type": {"text/plain"}}, Body: []byte("new response")}
	editor.showResult()
	oldCallback()
	if editor.responseBody.Text() != "new response" || string(editor.viewer.model.Body) != "new response" {
		t.Fatal("old preparation replaced new data")
	}
}

func TestBinaryImageEmptyAndPartialViewerStates(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	view := newWorkspaceView(app.NewWindow("Relay"), &http.Client{}, store)
	editor := view.active
	editor.draft.Response = &httpclient.Response{Status: "200 OK", Headers: http.Header{"Content-Type": {"application/pdf"}}, Body: []byte("%PDF-binary\x00\xff")}
	editor.showResult()
	if editor.viewer.model.Kind != responseview.Binary || editor.responseBody.Text() != "" || !editor.copyBody.Disabled() || editor.viewer.save.Disabled() {
		t.Fatal("binary response was displayed as text or could not be saved")
	}
	editor.draft.Response = &httpclient.Response{Status: "204 No Content"}
	editor.showResult()
	if editor.viewer.model.Kind != responseview.Empty {
		t.Fatal("empty body state missing")
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewNRGBA(image.Rect(0, 0, 32, 16))); err != nil {
		t.Fatal(err)
	}
	queue := make(chan func(), 1)
	view.dispatch = func(fn func()) { queue <- fn }
	editor.draft.Response = &httpclient.Response{Status: "200 OK", Headers: http.Header{"Content-Type": {"image/png"}}, Body: data.Bytes()}
	editor.showResult()
	awaitUI(t, queue)
	if editor.viewer.model.Image == nil || !editor.copyBody.Disabled() {
		t.Fatal("image preview failed or binary copy was enabled")
	}
	editor.draft.Response = &httpclient.Response{Status: "200 OK", Incomplete: true, Body: []byte("partial")}
	editor.showResult()
	if editor.viewer.save.Text != "Save partial response…" || !strings.Contains(editor.previewNote.Text, "Incomplete capture") {
		t.Fatal("partial bytes were presented as a complete file")
	}
}
