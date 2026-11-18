package main

import (
	"context"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/hsm-gustavo/relay/internal/httpclient"
)

func (v *workspaceView) showCookies() {
	env := v.store.State.ActiveEnvironment()
	id, name := "", noEnvironment
	if env != nil {
		id, name = env.ID, env.Name
	}
	label := widget.NewLabel("")
	label.Wrapping = fyne.TextWrapWord
	update := func() {
		if v.active == nil {
			label.SetText("Open a request to inspect its cookie scope.")
			return
		}
		// Use the active URL only. Invalid body/auth fields do not prevent inspection.
		template := httpclient.Request{Method: "GET", URL: v.active.url.Text}
		resolved, sensitive, err := v.session.Resolve(env, template)
		if err != nil {
			label.SetText("Resolve the request URL to inspect applicable cookies.")
			return
		}
		req, err := resolved.Build(context.Background())
		if err != nil {
			label.SetText("Enter a valid HTTP or HTTPS URL to inspect applicable cookies.")
			return
		}
		scope := httpclient.DisplayURL(req.URL, sensitive || strings.Contains(template.URL, "{{"))
		names := v.cookies.Names(id, req.URL)
		if len(names) == 0 {
			label.SetText("No session cookies apply to " + scope)
			return
		}
		text := scope + "\n\n"
		for _, name := range names {
			text += name + ": [masked]\n"
		}
		preview, _ := boundedPreview(text)
		label.SetText(preview)
	}
	update()
	clear := widget.NewButton("Clear this environment's cookies", func() {
		v.cookies.Clear(id)
		update()
	})
	content := container.NewBorder(widget.NewLabel("Applicable session cookies for the current request URL"), clear, nil, nil, container.NewVScroll(label))
	d := dialog.NewCustom("Cookies · "+name, "Close", content, v.window)
	d.Resize(fyne.NewSize(570, 320))
	d.Show()
}
