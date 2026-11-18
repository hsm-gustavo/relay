package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/hsm-gustavo/relay/internal/httpclient"
)

func (v *requestEditor) headerEditor() fyne.CanvasObject {
	v.headerRows = container.NewVBox()
	add := widget.NewButton("Add header", func() {
		v.headers = append(v.headers, httpclient.HeaderField{})
		v.changed(true)
		v.refreshHeaderRows()
	})
	return container.NewBorder(add, nil, nil, nil, container.NewVScroll(v.headerRows))
}

func (v *requestEditor) refreshHeaderRows() {
	v.headerRows.Objects = nil
	for i, row := range v.headers {
		name, value := widget.NewEntry(), widget.NewEntry()
		name.SetPlaceHolder("Header name")
		value.SetPlaceHolder("Value or {{variable}}")
		name.SetText(row.Name)
		value.SetText(row.Value)
		enabled := widget.NewCheck("Enabled", nil)
		enabled.SetChecked(!row.Disabled)
		name.OnChanged = func(text string) { v.headers[i].Name = text; v.changed(true) }
		value.OnChanged = func(text string) { v.headers[i].Value = text; v.changed(true) }
		enabled.OnChanged = func(on bool) { v.headers[i].Disabled = !on; v.changed(true) }
		remove := widget.NewButton("Remove", func() {
			v.headers = append(v.headers[:i], v.headers[i+1:]...)
			v.changed(true)
			v.refreshHeaderRows()
		})
		v.headerRows.Add(container.NewBorder(nil, nil, enabled, remove, container.NewGridWithColumns(2, name, value)))
	}
	v.headerRows.Refresh()
}
