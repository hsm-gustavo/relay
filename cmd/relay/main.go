package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/hsm-gustavo/relay/internal/httpclient"
)

func main() {
	a := app.New()
	w := a.NewWindow("Relay")
	w.Resize(fyne.NewSize(1000, 720))
	w.SetContent(requestView(w, &http.Client{Timeout: 30 * time.Second}))
	w.ShowAndRun()
}

func requestView(w fyne.Window, client *http.Client) fyne.CanvasObject {
	url := widget.NewEntry()
	url.SetPlaceHolder("https://example.com/api")
	method := widget.NewSelect(httpclient.Methods, nil)
	method.SetSelected(http.MethodGet)

	params := widget.NewMultiLineEntry()
	params.SetPlaceHolder("search=hello world\ntag=go\ntag=fyne")
	body := widget.NewMultiLineEntry()
	body.TextStyle = fyne.TextStyle{Monospace: true}
	body.SetPlaceHolder("{\n  \"message\": \"Hello\"\n}")
	requestTabs := container.NewAppTabs(
		container.NewTabItem("Query params", container.NewBorder(
			widget.NewLabel("One key=value per line; values are encoded automatically and added to the URL query."),
			nil, nil, nil, params)),
		container.NewTabItem("JSON body", container.NewBorder(
			widget.NewLabel("Optional JSON body. Content-Type: application/json is set when a body is provided."),
			nil, nil, nil, body)),
	)

	status := widget.NewLabel("Status: —")
	duration := widget.NewLabel("Time: —")
	size := widget.NewLabel("Body size: —")
	errorMessage := widget.NewLabel("")
	errorMessage.Wrapping = fyne.TextWrapWord
	responseBody := widget.NewTextGrid()
	responseBody.Scroll = fyne.ScrollBoth
	responseHeaders := widget.NewTextGrid()
	responseHeaders.Scroll = fyne.ScrollBoth
	var response *httpclient.Response
	format := widget.NewSelect([]string{"Formatted JSON", "Raw text"}, func(selected string) {
		if response != nil {
			responseBody.SetText(httpclient.FormatBody(response.Body, selected == "Formatted JSON"))
		}
	})
	format.PlaceHolder = "Formatted JSON"
	format.SetSelected("Formatted JSON")
	copyBody := widget.NewButton("Copy body", func() {
		w.Clipboard().SetContent(responseBody.Text())
	})
	copyHeaders := widget.NewButton("Copy headers", func() {
		w.Clipboard().SetContent(responseHeaders.Text())
	})
	responseTabs := container.NewAppTabs(
		container.NewTabItem("Response", container.NewBorder(
			container.NewHBox(format, copyBody), nil, nil, nil, responseBody)),
		container.NewTabItem("Headers", container.NewBorder(
			container.NewHBox(copyHeaders), nil, nil, nil, responseHeaders)),
	)
	var send *widget.Button
	send = widget.NewButton("Send", func() {
		req, err := (httpclient.Request{
			Method: method.Selected, URL: url.Text, Params: params.Text, Body: body.Text,
		}).Build(context.Background())
		response = nil
		responseBody.SetText("")
		responseHeaders.SetText("")
		status.SetText("Status: —")
		duration.SetText("Time: —")
		size.SetText("Body size: —")
		if err != nil {
			errorMessage.SetText(err.Error())
			return
		}
		errorMessage.SetText("")
		status.SetText("Sending…")
		send.Disable()
		go func() {
			result, err := httpclient.Execute(client, req)
			fyne.Do(func() {
				send.Enable()
				if err != nil {
					status.SetText("Request failed")
					errorMessage.SetText(err.Error())
					return
				}
				response = result
				status.SetText("Status: " + result.Status)
				duration.SetText(fmt.Sprintf("Time: %.1f ms", float64(result.Duration)/float64(time.Millisecond)))
				size.SetText(fmt.Sprintf("Body size: %d bytes", len(result.Body)))
				responseHeaders.SetText(httpclient.FormatHeaders(result.Headers))
				responseBody.SetText(httpclient.FormatBody(result.Body, format.Selected == "Formatted JSON"))
			})
		}()
	})
	send.Importance = widget.HighImportance
	url.OnSubmitted = func(string) {
		if !send.Disabled() {
			send.OnTapped()
		}
	}
	requestBar := container.NewBorder(nil, nil, method, send, url)
	responsePanel := container.NewBorder(container.NewHBox(status, duration, size), nil, nil, nil, responseTabs)
	panels := container.NewVSplit(requestTabs, responsePanel)
	panels.Offset = 0.4
	return container.NewBorder(container.NewVBox(requestBar, errorMessage), nil, nil, nil, panels)
}
