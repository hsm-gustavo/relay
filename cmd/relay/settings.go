package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (v *requestEditor) authSettings() fyne.CanvasObject {
	v.username = widget.NewEntry()
	v.password = widget.NewPasswordEntry()
	v.token = widget.NewPasswordEntry()
	basic := widget.NewForm(widget.NewFormItem("Username", v.username), widget.NewFormItem("Password", v.password))
	bearer := widget.NewForm(widget.NewFormItem("Token", v.token))
	v.auth = widget.NewSelect([]string{"None", "Basic", "Bearer"}, func(mode string) {
		basic.Hide()
		bearer.Hide()
		if mode == "Basic" {
			basic.Show()
		}
		if mode == "Bearer" {
			bearer.Show()
		}
		v.changed(true)
	})
	for _, entry := range []*widget.Entry{v.username, v.password, v.token} {
		entry.OnChanged = func(string) { v.changed(true) }
	}
	return container.NewVBox(widget.NewForm(widget.NewFormItem("Authentication", v.auth)), basic, bearer)
}

func (v *requestEditor) timeoutSettings() fyne.CanvasObject {
	v.timeout = widget.NewEntry()
	v.timeout.SetPlaceHolder("30s")
	v.timeout.OnChanged = func(string) { v.changed(true) }
	help := widget.NewLabel("Timeout includes connecting and reading the complete response. Leave blank for 30s; use 0 for no timeout.")
	help.Wrapping = fyne.TextWrapWord
	v.stopRedirect = widget.NewCheck("Stop at first redirect", func(bool) { v.changed(true) })
	return container.NewVBox(widget.NewForm(widget.NewFormItem("Timeout", v.timeout)), help, v.stopRedirect)
}
