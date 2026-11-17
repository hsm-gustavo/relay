package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/hsm-gustavo/relay/internal/httpclient"
	"github.com/hsm-gustavo/relay/internal/workspace"
)

type requestEditor struct {
	*workspaceView
	draft                                                       workspace.Draft
	tab                                                         *container.TabItem
	name, url, params, body, username, password, token, timeout *widget.Entry
	method, format, auth                                        *widget.Select
	status, duration, size, requestError                        *widget.Label
	responseBody, responseHeaders, bodyPreview                  *widget.TextGrid
	methodColors, statusColors                                  *textTheme
	send, cancel, copyBody                                      *widget.Button
	loading, closed                                             bool
	revision                                                    uint64
	cancelRun                                                   context.CancelFunc
}

func (v *requestEditor) capture() {
	d := &v.draft
	d.Name = v.name.Text
	d.Request = httpclient.Request{Method: v.method.Selected, URL: v.url.Text, Params: v.params.Text, Body: v.body.Text, Auth: httpclient.Auth{Type: v.auth.Selected, Username: v.username.Text, Password: v.password.Text, Token: v.token.Text}, Timeout: v.timeout.Text}
	d.Raw = v.format.Selected == "Raw text"
}

func (v *requestEditor) changed(clearResponse bool) {
	if v.loading {
		return
	}
	v.capture()
	v.revision++
	if clearResponse {
		v.draft.Response = nil
		v.draft.Error = ""
		v.showResult()
	}
	v.updateTitle()
	v.persist()
}

func (v *requestEditor) loadDraft(d workspace.Draft) {
	v.loading = true
	v.revision++
	v.draft = d
	v.name.SetText(d.Name)
	method := d.Request.Method
	if method == "" {
		method = http.MethodGet
	}
	v.method.SetSelected(method)
	v.url.SetText(d.Request.URL)
	v.params.SetText(d.Request.Params)
	v.body.SetText(d.Request.Body)
	auth := d.Request.Auth.Type
	if auth == "" {
		auth = "None"
	}
	v.auth.SetSelected(auth)
	v.username.SetText(d.Request.Auth.Username)
	v.password.SetText(d.Request.Auth.Password)
	v.token.SetText(d.Request.Auth.Token)
	v.timeout.SetText(d.Request.Timeout)
	if d.Raw {
		v.format.SetSelected("Raw text")
	} else {
		v.format.SetSelected("Formatted JSON")
	}
	v.showResult()
	v.loading = false
	v.capture()
	v.updateTitle()
}

func (v *requestEditor) showResult() {
	d := v.draft
	v.statusColors.role = neutral
	defer v.status.Refresh()
	v.status.SetText("Status: —")
	if v.cancelRun != nil {
		v.statusColors.role = blue
		v.status.SetText("Sending…")
	}
	v.duration.SetText("Time: —")
	v.size.SetText("Body size: —")
	v.responseBody.SetText("")
	v.responseHeaders.SetText("")
	v.requestError.SetText(d.Error)
	if d.Error == "" {
		v.requestError.Hide()
	} else {
		v.requestError.Show()
	}
	v.copyBody.Disable()
	if d.Error != "" {
		v.statusColors.role = red
		v.status.SetText("Request failed")
	}
	if d.Response == nil {
		return
	}
	v.copyBody.Enable()
	result := d.Response
	v.statusColors.role = statusRole(result.Status)
	v.status.SetText("Status: " + result.Status)
	v.duration.SetText(fmt.Sprintf("Time: %.1f ms", float64(result.Duration)/float64(time.Millisecond)))
	v.size.SetText(fmt.Sprintf("Body size: %d bytes", len(result.Body)))
	text := httpclient.FormatBody(result.Body, !d.Raw)
	colorJSON(v.responseBody, text, !d.Raw)
	v.responseHeaders.SetText(httpclient.FormatHeaders(result.Headers))
	colorHeaderKeys(v.responseHeaders)
}

func (v *requestEditor) makeEditor() fyne.CanvasObject {
	v.name = widget.NewEntry()
	v.name.SetPlaceHolder("Request name")
	v.url = widget.NewEntry()
	v.url.SetPlaceHolder("https://example.com/api or localhost:8080")
	v.methodColors = &textTheme{Theme: theme.DefaultTheme()}
	v.method = widget.NewSelect(httpclient.Methods, func(method string) {
		v.methodColors.role = methodRole(method)
		v.method.Refresh()
		v.changed(true)
	})
	v.params = widget.NewMultiLineEntry()
	v.params.SetPlaceHolder("search=hello world\ntag=go\ntag=fyne")
	v.body = widget.NewMultiLineEntry()
	v.body.TextStyle = fyne.TextStyle{Monospace: true}
	v.bodyPreview = widget.NewTextGrid()
	v.bodyPreview.Scroll = fyne.ScrollBoth
	v.body.SetPlaceHolder("{\n  \"message\": \"Hello\"\n}")
	v.name.OnChanged = func(string) { v.changed(false) }
	for _, entry := range []*widget.Entry{v.url, v.params, v.body} {
		entry.OnChanged = func(string) { v.changed(true) }
	}
	v.body.OnChanged = func(text string) { colorJSON(v.bodyPreview, text, true); v.changed(true) }
	paramsHelp := widget.NewLabel("One key=value per line. Values are encoded automatically; duplicate keys are allowed.")
	paramsHelp.Wrapping = fyne.TextWrapWord
	bodyHelp := widget.NewLabel("Optional JSON body, sent with Content-Type: application/json.")
	bodyHelp.Wrapping = fyne.TextWrapWord
	requestTabs := container.NewAppTabs(
		container.NewTabItem("Query params", container.NewBorder(paramsHelp, nil, nil, nil, v.params)),
		container.NewTabItem("JSON body", container.NewBorder(bodyHelp, nil, nil, nil, container.NewAppTabs(
			container.NewTabItem("Edit", v.body),
			container.NewTabItem("Highlighted preview", v.bodyPreview)))),
		container.NewTabItem("Authentication", v.authSettings()),
		container.NewTabItem("Settings", v.timeoutSettings()),
	)
	v.status = widget.NewLabel("")
	v.statusColors = &textTheme{Theme: theme.DefaultTheme()}
	v.duration = widget.NewLabel("")
	v.size = widget.NewLabel("")
	v.requestError = widget.NewLabel("")
	v.requestError.Wrapping = fyne.TextWrapWord
	v.requestError.Importance = widget.DangerImportance
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
	v.copyBody = widget.NewButton("Copy response body", func() { fyne.CurrentApp().Clipboard().SetContent(v.responseBody.Text()) })
	responseTabs := container.NewAppTabs(
		container.NewTabItem("Response", container.NewBorder(container.NewHBox(v.format, v.copyBody), nil, nil, nil, v.responseBody)),
		container.NewTabItem("Headers", container.NewBorder(widget.NewButton("Copy headers", func() {
			fyne.CurrentApp().Clipboard().SetContent(v.responseHeaders.Text())
		}), nil, nil, nil, v.responseHeaders)),
	)
	v.send = widget.NewButton("Send", v.execute)
	v.send.Importance = widget.HighImportance
	v.cancel = widget.NewButton("Cancel", func() {
		if v.cancelRun != nil {
			v.cancelRun()
		}
	})
	v.cancel.Disable()
	v.url.OnSubmitted = func(string) {
		if !v.send.Disabled() {
			v.execute()
		}
	}
	nameBar := container.NewBorder(nil, nil, nil, container.NewHBox(
		widget.NewButton("New tab", v.newRequest), widget.NewButton("Save request", v.saveRequest)), v.name)
	requestBar := container.NewBorder(nil, nil, container.NewThemeOverride(v.method, v.methodColors), container.NewHBox(v.send, v.cancel), v.url)
	panels := container.NewVSplit(requestTabs, container.NewBorder(container.NewHBox(container.NewThemeOverride(v.status, v.statusColors), v.duration, v.size), nil, nil, nil, responseTabs))
	panels.Offset = 0.4
	return container.NewBorder(container.NewVBox(nameBar, requestBar, v.requestError), nil, nil, nil, panels)
}

func (v *requestEditor) execute() {
	if v.cancelRun != nil {
		return
	}
	v.capture()
	snapshot := v.draft.Item
	timeout, err := snapshot.Request.TimeoutDuration()
	if err != nil {
		v.failValidation(err)
		return
	}
	var ctx context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	req, err := snapshot.Request.Build(ctx)
	if err != nil {
		cancel()
		v.failValidation(err)
		return
	}
	v.selectedHistory = ""
	v.historyList.UnselectAll()
	id := v.store.State.AddHistory(snapshot)
	v.historyList.Refresh()
	if !v.persist() {
		cancel()
		v.store.State.DeleteHistory(id)
		v.historyList.Refresh()
		return
	}
	revision := v.revision
	v.cancelRun = cancel
	v.draft.Response, v.draft.Error = nil, ""
	v.showResult()
	v.updateTitle()
	v.send.Disable()
	v.cancel.Enable()
	// The context applies the per-request setting without a shared client timeout capping it.
	client := *v.client
	client.Timeout = 0
	go func() {
		defer cancel()
		result, err := httpclient.Execute(&client, req)
		v.dispatch(func() {
			v.cancelRun = nil
			message := httpclient.ErrorMessage(err)
			v.store.State.CompleteHistory(id, result, message)
			if !v.closed {
				v.send.Enable()
				v.cancel.Disable()
				if revision == v.revision {
					v.draft.Response, v.draft.Error = result, message
				}
				v.showResult()
				v.updateTitle()
			}
			v.historyList.Refresh()
			v.persist()
		})
	}()
}

func (v *requestEditor) failValidation(err error) {
	v.draft.Response = nil
	v.draft.Error = err.Error()
	v.showResult()
	v.persist()
}
