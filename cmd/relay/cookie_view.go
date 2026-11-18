package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

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
	var target *url.URL
	var scope string
	if v.active != nil {
		// Invalid settings elsewhere in the editor should not prevent cookie management.
		template := httpclient.Request{Method: http.MethodGet, URL: v.active.url.Text}
		resolved, sensitive, err := v.session.Resolve(env, template)
		if err == nil {
			if req, err := resolved.Build(context.Background()); err == nil {
				target = req.URL
				scope = httpclient.DisplayURL(target, sensitive || strings.Contains(template.URL, "{{"))
			}
		}
	}
	update := func() {
		if target == nil {
			label.SetText("Enter a valid HTTP or HTTPS request URL to inspect or add cookies.")
			return
		}
		names := v.cookies.Names(id, target)
		if len(names) == 0 {
			label.SetText("No session cookies apply to " + scope)
			return
		}
		var text strings.Builder
		text.WriteString(scope + "\n\n")
		for _, name := range names {
			fmt.Fprintf(&text, "%s: [masked]\n", name)
		}
		preview, _ := boundedPreview(text.String())
		label.SetText(preview)
	}
	update()
	notice := widget.NewLabel("")
	notice.Wrapping = fyne.TextWrapWord
	notice.Hide()
	add := widget.NewButton("Add cookie", func() {
		v.addCookie(id, target, func() {
			update()
			notice.SetText("Cookie saved for this session. The list shows cookies that match this URL.")
			notice.Show()
		})
	})
	if target == nil {
		add.Disable()
	}
	clear := widget.NewButton("Clear this environment's cookies", func() {
		v.cookies.Clear(id)
		notice.Hide()
		update()
	})
	controls := container.NewHBox(add, clear)
	content := container.NewBorder(widget.NewLabel("Applicable session cookies for the current request URL"), container.NewVBox(notice, controls), nil, nil, container.NewVScroll(label))
	d := dialog.NewCustom("Cookies · "+name, "Close", content, v.window)
	d.Resize(fyne.NewSize(570, 320))
	d.Show()
}

func (v *workspaceView) addCookie(id string, target *url.URL, saved func()) {
	name, value, path, expires := widget.NewEntry(), widget.NewPasswordEntry(), widget.NewEntry(), widget.NewEntry()
	path.SetText("/")
	expires.SetPlaceHolder("RFC3339, or blank for this session")
	secure := widget.NewCheck("Send over HTTPS only", nil)
	secure.SetChecked(target.Scheme == "https")
	status := widget.NewLabel("")
	status.Importance = widget.DangerImportance
	status.Wrapping = fyne.TextWrapWord
	status.Hide()
	form := widget.NewForm(
		widget.NewFormItem("Host", widget.NewLabel(target.Hostname())),
		widget.NewFormItem("Name", name),
		widget.NewFormItem("Value", value),
		widget.NewFormItem("Path", path),
		widget.NewFormItem("Expires", expires),
		widget.NewFormItem("Secure", secure),
	)
	var d *dialog.CustomDialog
	submit := widget.NewButton("Save cookie", func() {
		cookie := http.Cookie{Name: strings.TrimSpace(name.Text), Value: value.Text, Path: strings.TrimSpace(path.Text), Secure: secure.Checked}
		if raw := strings.TrimSpace(expires.Text); raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				status.SetText("Enter an RFC3339 expiry such as 2027-01-01T00:00:00Z.")
				status.Show()
				return
			}
			cookie.Expires = parsed
		}
		if err := v.cookies.Add(id, target, cookie); err != nil {
			status.SetText(err.Error())
			status.Show()
			return
		}
		d.Hide()
		saved()
	})
	submit.Importance = widget.HighImportance
	d = dialog.NewCustom("Add cookie", "Cancel", container.NewVBox(form, status, submit), v.window)
	d.Resize(fyne.NewSize(530, 400))
	d.Show()
}
