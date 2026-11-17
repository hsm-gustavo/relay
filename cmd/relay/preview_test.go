package main

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/hsm-gustavo/relay/internal/httpclient"
	"github.com/hsm-gustavo/relay/internal/workspace"
)

func TestLargeBodyPreviewAndFullCopy(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	store, _ := workspace.Open(filepath.Join(t.TempDir(), "workspace.json"))
	view := newWorkspaceView(app.NewWindow("Relay"), &http.Client{}, store)
	body := strings.Repeat("<script>minified html</script>", 100000)
	view.active.draft.Response = &httpclient.Response{Status: "200 OK", Body: []byte(body)}
	view.active.showResult()
	if len([]rune(view.active.responseBody.Text())) > previewRunes+previewLines {
		t.Fatal("preview was not bounded")
	}
	if !view.active.previewNote.Visible() {
		t.Fatal("preview limit was not explained")
	}
	for _, row := range view.active.responseBody.Rows {
		if len(row.Cells) > previewColumns+8 {
			t.Fatal("minified line is unbounded")
		}
	}
	view.active.copyBody.OnTapped()
	if app.Clipboard().Content() != body {
		t.Fatal("copy returned only the preview")
	}
	view.active.draft.Error = "The connection closed before the full response was received."
	view.active.draft.Response.Incomplete = true
	view.active.showResult()
	if !strings.Contains(view.active.status.Text, "incomplete") || !view.active.requestError.Visible() {
		t.Fatal("partial response concealed its failure")
	}
}

func TestPreviewLimits(t *testing.T) {
	for _, source := range []string{strings.Repeat("a", 1<<20), strings.Repeat("line\n", 100000), strings.Repeat("\t", 10000), strings.Repeat("🙂", 10000)} {
		preview, clipped := boundedPreview(source)
		if !clipped || len([]rune(preview)) > previewRunes+previewLines || strings.Count(preview, "\n") > previewLines {
			t.Fatal("unbounded preview")
		}
	}
	_, note, _ := responsePreview([]byte{0xff, 0, 1}, false)
	if !strings.Contains(note, "binary") {
		t.Fatal("binary response was not explained")
	}
}

func BenchmarkLargeHTMLPreview(b *testing.B) {
	body := []byte(strings.Repeat("<html>minified content</html>", 100000))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		responsePreview(body, false)
	}
}
