package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/hsm-gustavo/relay/internal/httpclient"
	"github.com/hsm-gustavo/relay/internal/workspace"
)

func (v *workspaceView) currentCollection() *workspace.Collection {
	for i := range v.store.State.Collections {
		if v.store.State.Collections[i].ID == v.selectedCollection {
			return &v.store.State.Collections[i]
		}
	}
	return nil
}

func (v *workspaceView) refreshCollections() {
	names := make([]string, 0, len(v.store.State.Collections))
	selectedName := ""
	for _, c := range v.store.State.Collections {
		names = append(names, c.Name)
		if c.ID == v.selectedCollection {
			selectedName = c.Name
		}
	}
	if selectedName == "" && len(names) > 0 {
		selectedName = names[0]
		v.selectedCollection = v.store.State.Collections[0].ID
	}
	if len(names) == 0 {
		v.selectedCollection = ""
	}
	v.collections.Options = names
	v.collections.SetSelected(selectedName)
	v.collections.Refresh()
	v.store.State.SelectedCollectionID = v.selectedCollection
	v.savedList.Refresh()
}

func (v *workspaceView) namedDialog(title, initial string, action func(string) error) {
	name := widget.NewEntry()
	name.SetText(initial)
	name.Validator = func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("enter a name")
		}
		return nil
	}
	dialog.ShowForm(title, "Save", "Cancel", []*widget.FormItem{widget.NewFormItem("Name", name)}, func(ok bool) {
		if !ok {
			return
		}
		if err := action(name.Text); err != nil {
			dialog.ShowError(err, v.window)
			return
		}
		v.persist()
	}, v.window)
}

func (v *workspaceView) makeLibrary() fyne.CanvasObject {
	v.selectedCollection = v.store.State.SelectedCollectionID
	v.collections = widget.NewSelect(nil, func(name string) {
		for _, c := range v.store.State.Collections {
			if c.Name == name {
				v.selectedCollection = c.ID
				break
			}
		}
		v.store.State.SelectedCollectionID = v.selectedCollection
		v.selectedRequest = ""
		v.savedList.UnselectAll()
		v.savedList.Refresh()
		if !v.loading {
			v.persist()
		}
	})
	v.collections.PlaceHolder = "Create a collection"
	v.savedList = widget.NewList(func() int {
		if c := v.currentCollection(); c != nil {
			return len(c.Requests)
		}
		return 0
	}, func() fyne.CanvasObject { return newRequestRow("Saved request") }, func(i widget.ListItemID, object fyne.CanvasObject) {
		c := v.currentCollection()
		item := c.Requests[i]
		object.(*requestRow).set(item.Request.Method, item.Name)
	})
	v.savedList.OnSelected = func(i widget.ListItemID) {
		c := v.currentCollection()
		if c == nil || i >= len(c.Requests) {
			return
		}
		item := c.Requests[i]
		v.selectedRequest = item.ID
		v.openDraft(workspace.Draft{Item: item, CollectionID: c.ID, Raw: v.active.draft.Raw})
		v.persist()
	}
	newCollection := widget.NewButton("New collection", func() {
		v.namedDialog("New collection", "", func(name string) error {
			id, err := v.store.State.AddCollection(name)
			if err != nil {
				return err
			}
			v.selectedCollection = id
			v.refreshCollections()
			return nil
		})
	})
	renameCollection := widget.NewButton("Rename", func() {
		c := v.currentCollection()
		if c == nil {
			return
		}
		id := c.ID
		v.namedDialog("Rename collection", c.Name, func(name string) error {
			if err := v.store.State.RenameCollection(id, name); err != nil {
				return err
			}
			v.refreshCollections()
			return nil
		})
	})
	deleteCollection := widget.NewButton("Delete", func() {
		c := v.currentCollection()
		if c == nil {
			return
		}
		id := c.ID
		dialog.ShowConfirm("Delete collection", "Delete “"+c.Name+"” and all its saved requests?", func(ok bool) {
			if !ok {
				return
			}
			v.store.State.DeleteCollection(id)
			for _, editor := range v.editors {
				if editor.draft.CollectionID == id {
					editor.draft.CollectionID = ""
					editor.draft.ID = ""
				}
			}
			v.selectedRequest = ""
			v.savedList.UnselectAll()
			v.refreshCollections()
			v.persist()
		}, v.window)
	})
	deleteRequest := widget.NewButton("Delete request", func() {
		if v.selectedRequest == "" {
			return
		}
		collectionID, id := v.selectedCollection, v.selectedRequest
		dialog.ShowConfirm("Delete request", "Delete this saved request? Its history will be kept.", func(ok bool) {
			if !ok {
				return
			}
			v.store.State.DeleteRequest(collectionID, id)
			for _, editor := range v.editors {
				if editor.draft.ID == id {
					editor.draft.ID = ""
				}
			}
			v.selectedRequest = ""
			v.savedList.UnselectAll()
			v.savedList.Refresh()
			v.persist()
		}, v.window)
	})
	collectionHelp := widget.NewLabel("Save request writes to the selected collection. Open an item to edit it.")
	collectionHelp.Wrapping = fyne.TextWrapWord
	collectionsPanel := container.NewBorder(container.NewVBox(newCollection, v.collections,
		container.NewGridWithColumns(2, renameCollection, deleteCollection), collectionHelp), deleteRequest, nil, nil, v.savedList)

	v.historyList = widget.NewList(func() int { return len(v.store.State.History) }, func() fyne.CanvasObject {
		return newRequestRow("Request\nTimestamp")
	}, func(i widget.ListItemID, object fyne.CanvasObject) {
		item := v.store.State.History[i]
		result := ""
		if item.Pending {
			result = " · Sending…"
		}
		if item.Response != nil {
			result = " · " + item.Response.Status
		}
		if item.Error != "" {
			result = " · Failed"
		}
		object.(*requestRow).set(item.Request.Method, item.Name+"\n"+item.SentAt.Local().Format("Jan 02 15:04:05")+result)
	})
	v.historyList.OnSelected = func(i widget.ListItemID) {
		item := v.store.State.History[i]
		v.selectedHistory = item.ID
		draft := workspace.Draft{Item: item.Item, Response: item.Response, Error: item.Error, Raw: v.active.draft.Raw}
		draft.ID = "" // Saving history creates a collection request rather than modifying another saved item.
		v.openDraft(draft)
		v.persist()
	}
	renameHistory := widget.NewButton("Rename", func() {
		id := v.selectedHistory
		for _, item := range v.store.State.History {
			if item.ID != id {
				continue
			}
			v.namedDialog("Rename history item", item.Name, func(name string) error {
				for i := range v.store.State.History {
					if v.store.State.History[i].ID == id {
						v.store.State.History[i].Name = strings.TrimSpace(name)
						break
					}
				}
				v.historyList.Refresh()
				return nil
			})
			break
		}
	})
	deleteHistory := widget.NewButton("Delete", func() {
		id := v.selectedHistory
		if id == "" {
			return
		}
		dialog.ShowConfirm("Delete history item", "Delete this request and response from history?", func(ok bool) {
			if !ok {
				return
			}
			v.store.State.DeleteHistory(id)
			v.selectedHistory = ""
			v.historyList.UnselectAll()
			v.historyList.Refresh()
			v.persist()
		}, v.window)
	})
	historyHelp := widget.NewLabel("Open a sent request to inspect its response, edit, resend, or save it to a collection.")
	historyHelp.Wrapping = fyne.TextWrapWord
	historyPanel := container.NewBorder(historyHelp, container.NewGridWithColumns(2, renameHistory, deleteHistory), nil, nil, v.historyList)
	v.refreshCollections()
	return container.NewAppTabs(container.NewTabItem("Collections", collectionsPanel), container.NewTabItem("History", historyPanel))
}

func (v *workspaceView) newRequest() {
	v.savedList.UnselectAll()
	v.historyList.UnselectAll()
	v.selectedRequest, v.selectedHistory = "", ""
	v.openDraft(workspace.Draft{Item: workspace.Item{Request: httpclient.Request{Method: "GET"}}, Raw: v.active.draft.Raw})
	v.persist()
}

func (v *workspaceView) saveRequest() {
	v.active.capture()
	d := &v.active.draft
	item, err := v.store.State.SaveRequest(v.selectedCollection, d.ID, d.Name, d.Request)
	if err != nil {
		dialog.ShowError(err, v.window)
		return
	}
	d.Item = item
	d.CollectionID = v.selectedCollection
	v.selectedRequest = item.ID
	v.savedList.Refresh()
	v.persist()
}
