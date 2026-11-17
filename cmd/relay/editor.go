package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

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
	name, url, params, body                              *widget.Entry
	method, format, collections                          *widget.Select
	status, duration, size, requestError, storageError   *widget.Label
	responseBody, responseHeaders                        *widget.TextGrid
	savedList, historyList                               *widget.List
	selectedCollection, selectedRequest, selectedHistory string
	loading                                              bool
	revision                                             uint64
	send                                                 *widget.Button
}

func newWorkspaceView(w fyne.Window, client *http.Client, store *workspace.Store) *workspaceView {
	v := &workspaceView{window: w, client: client, store: store, loading: true}
	v.storageError = widget.NewLabel("")
	v.storageError.Wrapping = fyne.TextWrapWord
	editor := v.makeEditor()
	sidebar := v.makeLibrary()
	split := container.NewHSplit(sidebar, editor)
	split.Offset = 0.25
	v.content = container.NewBorder(v.storageError, nil, nil, nil, split)
	v.loadDraft(store.State.Draft)
	return v
}

func (v *workspaceView) persist() bool {
	if err := v.store.Save(); err != nil {
		v.storageError.SetText("Changes could not be saved: " + err.Error())
		return false
	}
	v.storageError.SetText("")
	return true
}

func (v *workspaceView) capture() {
	d := &v.store.State.Draft
	d.Name = v.name.Text
	d.Request = httpclient.Request{Method: v.method.Selected, URL: v.url.Text, Params: v.params.Text, Body: v.body.Text}
	d.Raw = v.format.Selected == "Raw text"
}

func (v *workspaceView) changed(clearResponse bool) {
	if v.loading {
		return
	}
	v.capture()
	v.revision++
	if clearResponse {
		v.store.State.Draft.Response = nil
		v.store.State.Draft.Error = ""
		v.showResult()
	}
	v.persist()
}

func (v *workspaceView) loadDraft(d workspace.Draft) {
	v.loading = true
	v.revision++
	v.store.State.Draft = d
	v.name.SetText(d.Name)
	method := d.Request.Method
	if method == "" {
		method = http.MethodGet
	}
	v.method.SetSelected(method)
	v.url.SetText(d.Request.URL)
	v.params.SetText(d.Request.Params)
	v.body.SetText(d.Request.Body)
	if d.Raw {
		v.format.SetSelected("Raw text")
	} else {
		v.format.SetSelected("Formatted JSON")
	}
	v.showResult()
	v.loading = false
	v.capture()
}

func (v *workspaceView) showResult() {
	d := v.store.State.Draft
	v.status.SetText("Status: —")
	v.duration.SetText("Time: —")
	v.size.SetText("Body size: —")
	v.responseBody.SetText("")
	v.responseHeaders.SetText("")
	v.requestError.SetText(d.Error)
	if d.Error != "" {
		v.status.SetText("Request failed")
	}
	if d.Response == nil {
		return
	}
	result := d.Response
	v.status.SetText("Status: " + result.Status)
	v.duration.SetText(fmt.Sprintf("Time: %.1f ms", float64(result.Duration)/float64(time.Millisecond)))
	v.size.SetText(fmt.Sprintf("Body size: %d bytes", len(result.Body)))
	v.responseBody.SetText(httpclient.FormatBody(result.Body, !d.Raw))
	v.responseHeaders.SetText(httpclient.FormatHeaders(result.Headers))
}

func (v *workspaceView) makeEditor() fyne.CanvasObject {
	v.name = widget.NewEntry()
	v.name.SetPlaceHolder("Request name")
	v.url = widget.NewEntry()
	v.url.SetPlaceHolder("https://example.com/api or localhost:8080")
	v.method = widget.NewSelect(httpclient.Methods, func(string) { v.changed(true) })
	v.params = widget.NewMultiLineEntry()
	v.params.SetPlaceHolder("search=hello world\ntag=go\ntag=fyne")
	v.body = widget.NewMultiLineEntry()
	v.body.TextStyle = fyne.TextStyle{Monospace: true}
	v.body.SetPlaceHolder("{\n  \"message\": \"Hello\"\n}")
	v.name.OnChanged = func(string) { v.changed(false) }
	for _, entry := range []*widget.Entry{v.url, v.params, v.body} {
		entry.OnChanged = func(string) { v.changed(true) }
	}
	paramsHelp := widget.NewLabel("One key=value per line. Values are encoded automatically; duplicate keys are allowed.")
	paramsHelp.Wrapping = fyne.TextWrapWord
	bodyHelp := widget.NewLabel("Optional JSON body, sent with Content-Type: application/json.")
	bodyHelp.Wrapping = fyne.TextWrapWord
	requestTabs := container.NewAppTabs(
		container.NewTabItem("Query params", container.NewBorder(paramsHelp, nil, nil, nil, v.params)),
		container.NewTabItem("JSON body", container.NewBorder(bodyHelp, nil, nil, nil, v.body)),
	)
	v.status = widget.NewLabel("")
	v.duration = widget.NewLabel("")
	v.size = widget.NewLabel("")
	v.requestError = widget.NewLabel("")
	v.requestError.Wrapping = fyne.TextWrapWord
	v.responseBody = widget.NewTextGrid()
	v.responseBody.Scroll = fyne.ScrollBoth
	v.responseHeaders = widget.NewTextGrid()
	v.responseHeaders.Scroll = fyne.ScrollBoth
	v.format = widget.NewSelect([]string{"Formatted JSON", "Raw text"}, func(string) {
		if v.loading {
			return
		}
		v.capture()
		v.showResult()
		v.persist()
	})
	v.format.PlaceHolder = "Formatted JSON"
	responseTabs := container.NewAppTabs(
		container.NewTabItem("Response", container.NewBorder(container.NewHBox(v.format, widget.NewButton("Copy body", func() {
			v.window.Clipboard().SetContent(v.responseBody.Text())
		})), nil, nil, nil, v.responseBody)),
		container.NewTabItem("Headers", container.NewBorder(widget.NewButton("Copy headers", func() {
			v.window.Clipboard().SetContent(v.responseHeaders.Text())
		}), nil, nil, nil, v.responseHeaders)),
	)
	v.send = widget.NewButton("Send", v.execute)
	v.send.Importance = widget.HighImportance
	v.url.OnSubmitted = func(string) {
		if !v.send.Disabled() {
			v.execute()
		}
	}
	nameBar := container.NewBorder(nil, nil, nil, container.NewHBox(
		widget.NewButton("New request", v.newRequest), widget.NewButton("Save request", v.saveRequest)), v.name)
	requestBar := container.NewBorder(nil, nil, v.method, v.send, v.url)
	panels := container.NewVSplit(requestTabs, container.NewBorder(container.NewHBox(v.status, v.duration, v.size), nil, nil, nil, responseTabs))
	panels.Offset = 0.4
	return container.NewBorder(container.NewVBox(nameBar, requestBar, v.requestError), nil, nil, nil, panels)
}

func (v *workspaceView) execute() {
	v.capture()
	snapshot := v.store.State.Draft.Item
	req, err := snapshot.Request.Build(context.Background())
	v.store.State.Draft.Response = nil
	v.store.State.Draft.Error = ""
	if err != nil {
		v.store.State.Draft.Error = err.Error()
		v.showResult()
		v.persist()
		return
	}
	v.selectedHistory = ""
	v.historyList.UnselectAll()
	id := v.store.State.AddHistory(snapshot)
	v.historyList.Refresh()
	if !v.persist() {
		return
	}
	revision := v.revision
	v.showResult()
	v.status.SetText("Sending…")
	v.send.Disable()
	go func() {
		result, err := httpclient.Execute(v.client, req)
		fyne.Do(func() {
			v.send.Enable()
			message := ""
			if err != nil {
				message = err.Error()
			}
			v.store.State.CompleteHistory(id, result, message)
			if revision == v.revision {
				v.store.State.Draft.Response = result
				v.store.State.Draft.Error = message
				v.showResult()
			}
			v.historyList.Refresh()
			v.persist()
		})
	}()
}
