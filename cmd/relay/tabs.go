package main

import (
	"crypto/rand"
	"net/http"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/hsm-gustavo/relay/internal/httpclient"
	"github.com/hsm-gustavo/relay/internal/workspace"
)

type workspaceView struct {
	window                                               fyne.Window
	client                                               *http.Client
	store                                                *workspace.Store
	content                                              fyne.CanvasObject
	collections, environments                            *widget.Select
	storageError                                         *widget.Label
	savedList, historyList                               *widget.List
	selectedCollection, selectedRequest, selectedHistory string
	loading                                              bool
	tabs                                                 *container.DocTabs
	editors                                              []*requestEditor
	active                                               *requestEditor
	dispatch                                             func(func())
	session                                              workspace.SessionVariables
}

func newWorkspaceView(w fyne.Window, client *http.Client, store *workspace.Store) *workspaceView {
	v := &workspaceView{window: w, client: client, store: store, loading: true, dispatch: fyne.Do}
	v.storageError = widget.NewLabel("")
	v.storageError.Wrapping = fyne.TextWrapWord
	v.storageError.Importance = widget.DangerImportance
	v.storageError.Hide()
	v.tabs = container.NewDocTabs()
	v.tabs.OnSelected = func(tab *container.TabItem) {
		for _, editor := range v.editors {
			if editor.tab == tab {
				v.active = editor
				break
			}
		}
		if !v.loading {
			v.persist()
		}
	}
	v.tabs.CreateTab = func() *container.TabItem {
		// DocTabs appends the returned tab itself.
		editor := v.makeRequestEditor(workspace.Draft{Item: workspace.Item{Request: httpclient.Request{Method: "GET"}}})
		return editor.tab
	}
	v.tabs.CloseIntercept = v.closeTab
	sidebar := v.makeLibrary()
	drafts := append([]workspace.Draft(nil), store.State.Tabs...)
	if len(drafts) == 0 {
		drafts = []workspace.Draft{store.State.Draft}
	}
	selected := store.State.ActiveTabID
	for _, draft := range drafts {
		v.appendDraft(draft)
	}
	for _, editor := range v.editors {
		if editor.draft.TabID == selected {
			v.tabs.Select(editor.tab)
		}
	}
	split := container.NewHSplit(sidebar, v.tabs)
	split.Offset = 0.25
	v.content = container.NewBorder(container.NewVBox(v.environmentBar(), v.storageError), nil, nil, nil, split)
	v.loading = false
	v.syncTabs()
	return v
}

func (v *workspaceView) syncTabs() {
	v.store.State.Tabs = make([]workspace.Draft, 0, len(v.editors))
	for _, editor := range v.editors {
		v.store.State.Tabs = append(v.store.State.Tabs, editor.draft)
	}
	if v.active != nil {
		v.store.State.Draft = v.active.draft // Migrate the old single-editor workspace without losing its draft.
		v.store.State.ActiveTabID = v.active.draft.TabID
	}
}

func (v *workspaceView) persist() bool {
	v.syncTabs()
	if err := v.store.Save(); err != nil {
		v.storageError.SetText("Changes could not be saved: " + err.Error())
		v.storageError.Show()
		return false
	}
	if v.store.Warning != "" {
		v.storageError.SetText(v.store.Warning)
		v.storageError.Show()
	} else {
		v.storageError.SetText("")
		v.storageError.Hide()
	}
	return true
}

func (v *workspaceView) makeRequestEditor(draft workspace.Draft) *requestEditor {
	if draft.TabID == "" {
		draft.TabID = rand.Text()
	}
	editor := &requestEditor{workspaceView: v, draft: draft, loading: true}
	editor.tab = container.NewTabItem("New request", editor.makeEditor())
	editor.loadDraft(draft)
	v.editors = append(v.editors, editor)
	return editor
}

func (v *workspaceView) appendDraft(draft workspace.Draft) {
	editor := v.makeRequestEditor(draft)
	v.active = editor
	v.tabs.Append(editor.tab)
	v.tabs.Select(editor.tab)
}

func (v *workspaceView) openDraft(draft workspace.Draft) {
	if draft.ID != "" {
		for _, editor := range v.editors {
			if editor.draft.ID == draft.ID {
				v.tabs.Select(editor.tab)
				v.persist()
				return
			}
		}
	}
	v.appendDraft(draft)
	v.persist()
}

func (v *workspaceView) closeTab(tab *container.TabItem) {
	for i, editor := range v.editors {
		if editor.tab != tab {
			continue
		}
		editor.closed = true
		if editor.cancelRun != nil {
			editor.cancelRun()
		}
		v.editors = append(v.editors[:i], v.editors[i+1:]...)
		break
	}
	v.tabs.Remove(tab)
	if len(v.editors) == 0 {
		v.appendDraft(workspace.Draft{Item: workspace.Item{Request: httpclient.Request{Method: "GET"}}})
	}
	for _, editor := range v.editors {
		if editor.tab == v.tabs.Selected() {
			v.active = editor
		}
	}
	v.persist()
}

func (e *requestEditor) updateTitle() {
	if e.tab == nil {
		return
	}
	title := strings.TrimSpace(e.draft.Name)
	if title == "" {
		title = strings.TrimSpace(e.draft.Request.URL)
	}
	if title == "" {
		title = "New request"
	}
	if text := []rune(title); len(text) > 28 {
		title = string(text[:28]) + "…"
	}
	if e.cancelRun != nil {
		title += " (sending)"
	}
	e.tab.Text = title
	e.tabs.Refresh()
}
