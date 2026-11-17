package main

import (
	"context"
	"fmt"
	"github.com/hsm-gustavo/relay/internal/extraction"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/hsm-gustavo/relay/internal/httpclient"
	"github.com/hsm-gustavo/relay/internal/workspace"
)

type requestEditor struct {
	bodyType                *widget.Select
	formFields              []httpclient.FormField
	formBox                 *fyne.Container
	extractRules            []extraction.Rule
	extractResults          []extraction.Result
	rulesBox, extractionLog *fyne.Container
	extractionSummary       *widget.Label
	*workspaceView
	draft                                                                          workspace.Draft
	tab                                                                            *container.TabItem
	name, url, params, body, username, password, token, timeout                    *widget.Entry
	method, format, auth                                                           *widget.Select
	status, duration, size, requestError, previewNote, headerNote, environmentInfo *widget.Label
	responseBody, responseHeaders, bodyPreview                                     *widget.TextGrid
	methodColors, statusColors                                                     *textTheme
	send, cancel, copyBody                                                         *widget.Button
	loading, closed                                                                bool
	revision                                                                       uint64
	cancelRun                                                                      context.CancelFunc
}

func (v *requestEditor) capture() {
	d := &v.draft
	d.Name = v.name.Text
	d.Request = httpclient.Request{Method: v.method.Selected, URL: v.url.Text, Params: v.params.Text, Body: v.body.Text, Auth: httpclient.Auth{Type: v.auth.Selected, Username: v.username.Text, Password: v.password.Text, Token: v.token.Text}, Timeout: v.timeout.Text, BodyType: v.bodyType.Selected, FormFields: append([]httpclient.FormField(nil), v.formFields...), Extract: append([]extraction.Rule(nil), v.extractRules...)}
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
		v.draft.SessionOnly = false
		v.showResult()
	}
	v.updateTitle()
	v.persist()
}

func (v *requestEditor) loadDraft(d workspace.Draft) {
	v.loading = true
	v.revision++
	v.draft = d
	v.formFields = append([]httpclient.FormField(nil), d.Request.FormFields...)
	v.bodyType.SetSelected(d.Request.EffectiveBodyType())
	v.refreshFormFields()
	v.extractRules = append([]extraction.Rule(nil), d.Request.Extract...)
	v.refreshExtractionRules()
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
	v.previewNote.SetText("")
	v.previewNote.Hide()
	v.headerNote.SetText("")
	v.headerNote.Hide()
	v.responseHeaders.SetText("")
	v.requestError.SetText(d.Error)
	if d.Error == "" {
		v.requestError.Hide()
	} else {
		v.requestError.Show()
	}
	v.environmentInfo.Hide()
	if d.Response != nil || d.Error != "" {
		name := d.EnvironmentName
		if name == "" {
			name = "No environment"
		}
		v.environmentInfo.SetText("Sent with: " + name)
		v.environmentInfo.Show()
	}
	if d.SessionOnly && d.Response == nil && d.Error == "" && v.cancelRun == nil {
		v.previewNote.SetText("Session-only response was not retained. Send again to obtain fresh values.")
		v.previewNote.Show()
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
	text, note, highlight := responsePreview(result.Body, d.Raw)
	colorJSON(v.responseBody, text, highlight)
	if note != "" {
		v.previewNote.SetText(note)
		v.previewNote.Show()
	}
	if result.Incomplete {
		v.statusColors.role = red
		v.status.SetText("Status: " + result.Status + " (incomplete)")
	}
	headers, limited := boundedPreview(httpclient.FormatHeaders(result.Headers))
	v.responseHeaders.SetText(headers)
	if limited {
		v.headerNote.SetText("Header preview limited. Copy headers copies all response headers.")
		v.headerNote.Show()
	}
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
	requestTabs := container.NewAppTabs(
		container.NewTabItem("Query params", container.NewBorder(paramsHelp, nil, nil, nil, v.params)),
		container.NewTabItem("Body", v.bodyEditor()),
		container.NewTabItem("Authentication", v.authSettings()),
		container.NewTabItem("Settings", v.timeoutSettings()),
		container.NewTabItem("Extract", v.extractionSettings()),
	)
	v.environmentInfo = widget.NewLabel("")
	v.environmentInfo.Hide()
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
	v.copyBody = widget.NewButton("Copy response body", func() {
		if result := v.draft.Response; result != nil {
			fyne.CurrentApp().Clipboard().SetContent(httpclient.FormatBody(result.Body, !v.draft.Raw))
		}
	})
	v.headerNote = widget.NewLabel("")
	v.headerNote.Wrapping = fyne.TextWrapWord
	v.headerNote.Hide()
	v.previewNote = widget.NewLabel("")
	v.previewNote.Wrapping = fyne.TextWrapWord
	v.previewNote.Hide()
	responseTabs := container.NewAppTabs(
		container.NewTabItem("Response", container.NewBorder(container.NewVBox(container.NewHBox(v.format, v.copyBody), v.previewNote), nil, nil, nil, v.responseBody)),
		container.NewTabItem("Headers", container.NewBorder(container.NewVBox(widget.NewButton("Copy headers", func() {
			if result := v.draft.Response; result != nil {
				fyne.CurrentApp().Clipboard().SetContent(httpclient.FormatHeaders(result.Headers))
			}
		}), v.headerNote), nil, nil, nil, v.responseHeaders)),
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
	panels := container.NewVSplit(requestTabs, container.NewBorder(container.NewVBox(container.NewHBox(container.NewThemeOverride(v.status, v.statusColors), v.duration, v.size), v.environmentInfo), nil, nil, nil, responseTabs))
	panels.Offset = 0.4
	return container.NewBorder(container.NewVBox(nameBar, requestBar, v.requestError), nil, nil, nil, panels)
}

func (v *requestEditor) execute() {
	if v.cancelRun != nil {
		return
	}
	v.capture()
	snapshot := v.draft.Item
	environmentName := "No environment"
	environmentID := ""
	env := v.store.State.ActiveEnvironment()
	if env != nil {
		environmentName = env.Name
		environmentID = env.ID
	}
	resolved, sensitive, err := v.session.Resolve(env, snapshot.Request)
	sensitive = sensitive || snapshot.Request.HasEnabledExtractionRules()
	if err != nil {
		v.failValidation(err, sensitive)
		return
	}
	timeout, err := resolved.TimeoutDuration()
	if err != nil {
		v.failValidation(err, sensitive)
		return
	}
	var ctx context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	req, err := resolved.Build(ctx)
	if err != nil {
		cancel()
		v.failValidation(err, sensitive)
		return
	}
	v.selectedHistory = ""
	v.historyList.UnselectAll()
	id := v.store.State.AddHistory(snapshot)
	v.store.State.History[0].EnvironmentName = environmentName
	v.store.State.History[0].SessionOnly = sensitive
	v.store.State.History[0].ResolvedRequest = &resolved
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
	v.draft.SessionOnly = sensitive
	v.draft.EnvironmentName = environmentName
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
		code := 0
		var body []byte
		if result != nil {
			body = result.Body
			parts := strings.Fields(result.Status)
			if len(parts) > 0 {
				code, _ = strconv.Atoi(parts[0])
			}
		}
		results := extraction.Evaluate(code, body, err == nil && result != nil && !result.Incomplete, snapshot.Request.Extract)
		v.dispatch(func() {
			targetID := environmentID
			if targetID != "" {
				exists := false
				for _, environment := range v.store.State.Environments {
					if environment.ID == targetID {
						exists = true
						break
					}
				}
				if !exists {
					targetID = ""
				}
			}
			results = v.session.Apply(targetID, results)
			v.cancelRun = nil
			message := httpclient.ErrorMessage(err)
			v.store.State.CompleteHistory(id, result, message)
			if !v.closed {
				v.send.Enable()
				v.recordExtractionResults(results)
				v.cancel.Disable()
				if revision == v.revision {
					v.draft.Response, v.draft.Error = result, message
					v.draft.SessionOnly = sensitive
					v.draft.EnvironmentName = environmentName
				}
				v.showResult()
				v.updateTitle()
			}
			v.historyList.Refresh()
			v.persist()
		})
	}()
}

func (v *requestEditor) failValidation(err error, sensitive bool) {
	v.draft.SessionOnly = sensitive
	v.draft.Response = nil
	v.draft.Error = err.Error()
	v.draft.EnvironmentName = ""
	v.showResult()
	v.persist()
}
