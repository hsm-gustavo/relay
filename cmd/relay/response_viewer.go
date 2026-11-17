package main

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/hsm-gustavo/relay/internal/httpclient"
	"github.com/hsm-gustavo/relay/internal/responseview"
)

type responseViewer struct {
	adjustingSearch                              bool
	lastQuery                                    string
	navigation                                   *fyne.Container
	cancelPrepare                                context.CancelFunc
	owner                                        *requestEditor
	model                                        *responseview.Model
	response                                     *httpclient.Response
	generation, searchGeneration                 uint64
	updating                                     bool
	content                                      *fyne.Container
	tree                                         *widget.Tree
	search                                       *widget.Entry
	count, pageInfo                              *widget.Label
	previous, next, previousPage, nextPage, save *widget.Button
	searchBar                                    fyne.CanvasObject
	matches                                      responseview.SearchResult
	current                                      int
	page                                         responseview.Page
	offsets                                      []int
	pageIndex                                    int
}

func newResponseViewer(owner *requestEditor) fyne.CanvasObject {
	v := &responseViewer{owner: owner, current: -1, offsets: []int{0}}
	owner.viewer = v
	owner.responseBody = widget.NewTextGrid()
	owner.responseBody.Scroll = fyne.ScrollBoth
	owner.responseBody.TabWidth = 4
	owner.previewNote = widget.NewLabel("")
	owner.previewNote.Wrapping = fyne.TextWrapWord
	owner.previewNote.Hide()
	owner.contentType = widget.NewLabel("Content-Type: —")
	owner.contentType.Truncation = fyne.TextTruncateEllipsis
	owner.format = widget.NewSelect([]string{responseview.Pretty, responseview.Raw, responseview.Tree}, func(string) {
		if owner.loading || v.updating {
			return
		}
		owner.capture()
		v.offsets = []int{0}
		v.pageIndex = 0
		v.render()
		v.searchText()
		owner.persist()
	})
	owner.format.PlaceHolder = "Pretty"
	owner.copyBody = widget.NewButton("Copy response body", v.copyResponse)
	v.save = widget.NewButton("Save response…", v.saveResponse)
	v.search = widget.NewEntry()
	v.search.SetPlaceHolder("Find in response (Ctrl+F)")
	v.search.OnChanged = func(text string) {
		if v.adjustingSearch {
			return
		}
		if len(text) > responseview.MaxQueryRunes*utf8.UTFMax || utf8.RuneCountInString(text) > responseview.MaxQueryRunes {
			v.adjustingSearch = true
			v.search.SetText(v.lastQuery)
			v.adjustingSearch = false
			note := "Search terms are limited to 256 characters."
			if v.model != nil && v.model.Note != "" {
				note = v.model.Note + "\n" + note
			}
			v.owner.previewNote.SetText(note)
			v.owner.previewNote.Show()
			return
		}
		v.lastQuery = text
		v.searchText()
	}
	v.search.OnSubmitted = func(string) { v.moveMatch(1) }
	v.count = widget.NewLabel("")
	v.previous = widget.NewButton("Previous", func() { v.moveMatch(-1) })
	v.next = widget.NewButton("Next", func() { v.moveMatch(1) })
	v.searchBar = container.NewBorder(nil, nil, nil, container.NewHBox(v.previous, v.next, v.count), v.search)
	v.previousPage = widget.NewButton("Previous page", func() {
		if v.pageIndex > 0 {
			v.pageIndex--
			v.renderText()
		}
	})
	v.nextPage = widget.NewButton("Next page", func() {
		if !v.page.More {
			return
		}
		v.offsets = append(v.offsets[:v.pageIndex+1], v.page.End)
		v.pageIndex++
		v.renderText()
	})
	v.pageInfo = widget.NewLabel("")
	v.pageInfo.Truncation = fyne.TextTruncateEllipsis
	v.content = container.NewStack(widget.NewLabel("Send a request to view its response."))
	v.tree = widget.NewTree(func(id string) []string {
		if v.model == nil || v.model.Nodes == nil {
			return nil
		}
		if id == "" {
			return []string{"0"}
		}
		return v.model.Nodes[id].Children
	}, func(id string) bool {
		if v.model == nil {
			return false
		}
		if id == "" {
			return v.model.Nodes != nil
		}
		node := v.model.Nodes[id]
		return node.Kind == "array" || node.Kind == "object"
	}, func(bool) fyne.CanvasObject {
		label := widget.NewLabel("")
		label.TextStyle = fyne.TextStyle{Monospace: true}
		label.Truncation = fyne.TextTruncateEllipsis
		return container.NewThemeOverride(label, &textTheme{Theme: theme.DefaultTheme()})
	}, func(id string, _ bool, object fyne.CanvasObject) {
		if v.model == nil {
			return
		}
		node := v.model.Nodes[id]
		over := object.(*container.ThemeOverride)
		colors := over.Theme.(*textTheme)
		colors.role = neutral
		switch node.Kind {
		case "string":
			colors.role = green
		case "number":
			colors.role = amber
		case "boolean":
			colors.role = purple
		case "null":
			colors.role = rose
		case "object", "array":
			colors.role = blue
		}
		label := over.Content.(*widget.Label)
		label.SetText(shortLabel(node.Label(), 512))
		over.Refresh()
	})
	v.resetControls()
	tools := container.NewHBox(owner.format, owner.copyBody, v.save)
	v.navigation = container.NewBorder(nil, nil, container.NewHBox(v.previousPage, v.nextPage), nil, v.pageInfo)
	v.navigation.Hide()
	return container.NewBorder(container.NewVBox(tools, v.searchBar, owner.previewNote), v.navigation, nil, nil, v.content)
}

func shortLabel(text string, limit int) string {
	count := 0
	for offset := range text {
		if count == limit {
			return text[:offset] + "…"
		}
		count++
	}
	return text
}

func (v *responseViewer) resetControls() {
	if v.navigation != nil {
		v.navigation.Hide()
	}
	v.owner.copyBody.Disable()
	v.owner.format.Disable()
	v.save.Disable()
	v.search.Disable()
	v.previous.Disable()
	v.next.Disable()
	v.previousPage.Disable()
	v.nextPage.Disable()
	v.count.SetText("")
	v.pageInfo.SetText("")
}

func (v *responseViewer) setResponse(response *httpclient.Response) {
	if v.response == response && (response == nil || v.model != nil) {
		return
	}
	if v.response == response {
		return
	} // An asynchronous preparation is already in flight.
	if v.cancelPrepare != nil {
		v.cancelPrepare()
		v.cancelPrepare = nil
	}
	v.generation++
	v.searchGeneration++
	generation := v.generation
	v.response = response
	v.model = nil
	v.matches = responseview.SearchResult{}
	v.current = -1
	v.offsets = []int{0}
	v.pageIndex = 0
	v.resetControls()
	v.owner.previewNote.Hide()
	v.owner.responseBody.SetText("")
	v.owner.responseHeaders.SetText("")
	v.owner.headerNote.Hide()
	v.owner.contentType.SetText("Content-Type: —")
	v.tree.CloseAllBranches()
	if response == nil {
		v.content.Objects = []fyne.CanvasObject{widget.NewLabel("Send a request to view its response.")}
		v.content.Refresh()
		return
	}
	v.content.Objects = []fyne.CanvasObject{widget.NewLabel("Preparing response preview…")}
	v.content.Refresh()
	apply := func(model *responseview.Model) {
		if generation != v.generation || v.owner.closed || v.owner.draft.Response != response {
			return
		}
		v.model = model
		typeLabel := model.ContentType
		if model.Detected {
			typeLabel += " (detected)"
		}
		v.owner.contentType.SetText("Content-Type: " + typeLabel)
		v.owner.responseHeaders.SetText(model.HeaderPreview)
		colorHeaderKeys(v.owner.responseHeaders)
		if model.HeaderLimited {
			v.owner.headerNote.SetText("Header preview limited. Copy headers copies all response headers.")
			v.owner.headerNote.Show()
		}
		v.save.Enable()
		if model.Incomplete {
			v.save.SetText("Save partial response…")
		} else {
			v.save.SetText("Save response…")
		}
		v.updating = true
		options := []string{responseview.Raw}
		if model.SupportsPretty {
			options = []string{responseview.Pretty, responseview.Raw}
		}
		if model.Kind == responseview.JSON {
			options = []string{responseview.Pretty, responseview.Raw, responseview.Tree}
		}
		v.owner.format.Options = options
		allowed := false
		for _, option := range options {
			if option == v.owner.format.Selected {
				allowed = true
				break
			}
		}
		if !allowed {
			selected := responseview.Raw
			if model.SupportsPretty {
				selected = responseview.Pretty
			}
			v.owner.format.SetSelected(selected)
		}
		if model.Kind == responseview.JSON || model.Kind == responseview.Text {
			v.owner.format.Enable()
		}
		v.owner.format.Refresh()
		v.updating = false
		if model.Kind == responseview.JSON && model.Nodes != nil {
			v.tree.OpenBranch("0")
		}
		v.render()
		v.searchText()
	}
	if responseview.CanPrepareInline(response) {
		apply(responseview.Prepare(response))
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	v.cancelPrepare = cancel
	go func() {
		model := responseview.PrepareContext(ctx, response)
		if model == nil {
			return
		}
		v.owner.dispatch(func() { apply(model) })
	}()
}

func (v *responseViewer) render() {
	if v.model == nil {
		return
	}
	m := v.model
	v.owner.previewNote.SetText(m.Note)
	if m.Note == "" {
		v.owner.previewNote.Hide()
	} else {
		v.owner.previewNote.Show()
	}
	v.search.Disable()
	v.owner.copyBody.Disable()
	v.previousPage.Disable()
	v.nextPage.Disable()
	v.navigation.Hide()
	v.pageInfo.SetText("")
	v.previous.Disable()
	v.next.Disable()
	switch m.Kind {
	case responseview.Empty:
		v.content.Objects = []fyne.CanvasObject{widget.NewLabel("This response has no body.")}
		v.owner.copyBody.Enable()
	case responseview.Binary:
		info := widget.NewLabel(fmt.Sprintf("Binary response\nContent-Type: %s\nRetained body: %d bytes\nUse Save response to save the received bytes.", m.ContentType, len(m.Body)))
		info.Wrapping = fyne.TextWrapWord
		v.content.Objects = []fyne.CanvasObject{info}
	case responseview.Image:
		if m.Image != nil {
			preview := canvas.NewImageFromImage(m.Image)
			preview.FillMode = canvas.ImageFillContain
			v.content.Objects = []fyne.CanvasObject{preview}
		} else {
			v.content.Objects = []fyne.CanvasObject{widget.NewLabel("Image preview unavailable. Save the retained bytes to inspect this response.")}
		}
	default:
		v.owner.copyBody.Enable()
		if v.owner.format.Selected == responseview.Tree {
			if m.Nodes == nil {
				message := widget.NewLabel(m.TreeNote)
				message.Wrapping = fyne.TextWrapWord
				v.content.Objects = []fyne.CanvasObject{message}
			} else {
				v.tree.Refresh()
				v.content.Objects = []fyne.CanvasObject{v.tree}
			}
		} else {
			v.search.Enable()
			v.content.Objects = []fyne.CanvasObject{v.owner.responseBody}
			v.renderText()
		}
	}
	v.content.Refresh()
}

func (v *responseViewer) renderText() {
	if v.model == nil {
		return
	}
	text := v.model.TextFor(v.owner.format.Selected)
	start := v.offsets[v.pageIndex]
	v.page = responseview.Window(text, start)
	paintJSON(v.owner.responseBody, v.page.Text, v.model.Kind == responseview.JSON && v.owner.format.Selected == responseview.Pretty && !v.model.PrettyLimited)
	v.highlightMatch()
	if v.pageIndex > 0 {
		v.previousPage.Enable()
	} else {
		v.previousPage.Disable()
	}
	if v.page.More {
		v.nextPage.Enable()
	} else {
		v.nextPage.Disable()
	}
	if v.page.Start > 0 || v.page.More {
		v.navigation.Show()
		v.pageInfo.SetText(fmt.Sprintf("Characters %d–%d · bounded preview · Copy all / Save original", v.page.Start+1, v.page.End))
	} else {
		v.navigation.Hide()
		v.pageInfo.SetText("")
	}
	v.owner.responseBody.ScrollToTop()
}

func (v *responseViewer) searchText() {
	v.searchGeneration++
	generation := v.searchGeneration
	v.matches = responseview.SearchResult{}
	v.current = -1
	v.count.SetText("")
	v.previous.Disable()
	v.next.Disable()
	if v.model == nil || v.search.Disabled() {
		return
	}
	query := v.search.Text
	if query == "" {
		v.renderText()
		return
	}
	model := v.model
	mode := v.owner.format.Selected
	text := model.TextFor(mode)
	apply := func(result responseview.SearchResult) {
		if generation != v.searchGeneration || v.model != model || v.owner.closed {
			return
		}
		v.matches = result
		if len(result.Matches) > 0 {
			v.current = 0
			v.previous.Enable()
			v.next.Enable()
			v.showMatch()
		} else {
			v.count.SetText("0 matches")
			v.renderText()
		}
		if result.Note != "" {
			v.owner.previewNote.SetText(strings.TrimSpace(model.Note + "\n" + result.Note))
			v.owner.previewNote.Show()
		}
	}
	if len(text) <= 8<<10 {
		apply(responseview.Find(text, query))
		return
	}
	v.count.SetText("Searching…")
	go func() { result := responseview.Find(text, query); v.owner.dispatch(func() { apply(result) }) }()
}

func (v *responseViewer) moveMatch(delta int) {
	v.current = responseview.Navigate(v.current, delta, len(v.matches.Matches))
	if v.current >= 0 {
		v.showMatch()
	}
}
func (v *responseViewer) showMatch() {
	match := v.matches.Matches[v.current]
	v.offsets = []int{0}
	v.pageIndex = 0
	start := match.Start - 200
	if start < 0 {
		start = 0
	}
	if page := responseview.Window(v.model.TextFor(v.owner.format.Selected), start); page.End < match.End {
		start = match.Start
	}
	if start > 0 {
		v.offsets = append(v.offsets, start)
		v.pageIndex = 1
	}
	suffix := ""
	if v.matches.Limited || v.model.TextLimited {
		suffix = " (limited)"
	}
	v.count.SetText(fmt.Sprintf("%d/%d%s", v.current+1, len(v.matches.Matches), suffix))
	v.renderText()
}

func (v *responseViewer) focusSearch() {
	if v.model == nil {
		return
	}
	if v.model.Kind != responseview.JSON && v.model.Kind != responseview.Text {
		return
	}
	v.owner.responseTabs.SelectIndex(0)
	if v.owner.format.Selected == responseview.Tree {
		v.owner.format.SetSelected(responseview.Raw)
	}
	v.owner.window.Canvas().Focus(v.search)
}

func (v *responseViewer) copyResponse() {
	if v.model == nil {
		return
	}
	model := v.model
	mode := v.owner.format.Selected
	copyNow := func() {
		text, err := model.CopyText(mode)
		v.owner.dispatch(func() {
			if v.owner.closed {
				return
			}
			if err == nil {
				fyne.CurrentApp().Clipboard().SetContent(text)
			}
		})
	}
	if len(model.Body) <= 8<<10 {
		text, err := model.CopyText(mode)
		if err == nil {
			fyne.CurrentApp().Clipboard().SetContent(text)
		}
		return
	}
	go copyNow()
}
