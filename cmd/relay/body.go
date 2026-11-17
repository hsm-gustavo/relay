package main

import (
	"crypto/rand"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/hsm-gustavo/relay/internal/httpclient"
)

func (v *requestEditor) bodyEditor() fyne.CanvasObject {
	jsonHelp := widget.NewLabel("Optional JSON body. Use {{variables}} in any active body field.")
	jsonHelp.Wrapping = fyne.TextWrapWord
	jsonEditor := container.NewBorder(jsonHelp, nil, nil, nil, container.NewAppTabs(container.NewTabItem("Edit", v.body), container.NewTabItem("Highlighted preview", v.bodyPreview)))
	v.formBox = container.NewVBox()
	add := widget.NewButton("Add field", func() {
		v.formFields = append(v.formFields, httpclient.FormField{ID: rand.Text()})
		v.changed(true)
		v.refreshFormFields()
	})
	help := widget.NewLabel("Text values are encoded automatically. File contents are read only when sending. Repeated names are allowed.")
	help.Wrapping = fyne.TextWrapWord
	formEditor := container.NewBorder(container.NewBorder(nil, nil, nil, add, help), nil, nil, nil, container.NewVScroll(v.formBox))
	v.bodyType = widget.NewSelect([]string{httpclient.BodyJSON, httpclient.BodyURLEncoded, httpclient.BodyMultipart}, func(mode string) {
		jsonEditor.Hide()
		formEditor.Hide()
		if mode == httpclient.BodyJSON {
			jsonEditor.Show()
		} else {
			formEditor.Show()
		}
		v.refreshFormFields()
		v.changed(true)
	})
	v.bodyType.PlaceHolder = httpclient.BodyURLEncoded
	return container.NewBorder(container.NewHBox(widget.NewLabel("Content-Type"), v.bodyType), nil, nil, nil, container.NewStack(jsonEditor, formEditor))
}

func (v *requestEditor) refreshFormFields() {
	if v.formBox == nil {
		return
	}
	v.formBox.Objects = nil
	for i := range v.formFields {
		if v.formFields[i].ID == "" {
			v.formFields[i].ID = rand.Text()
		}
		field := v.formFields[i]
		enabled := widget.NewCheck("Enabled", nil)
		enabled.SetChecked(!field.Disabled)
		name := widget.NewEntry()
		name.SetPlaceHolder("Field name")
		name.SetText(field.Name)
		value := widget.NewEntry()
		value.SetPlaceHolder("Value or {{variable}}")
		if field.Type == "file" {
			value.SetPlaceHolder("File path or {{variable}}")
			value.SetText(field.FilePath)
		} else {
			value.SetText(field.Value)
		}
		kind := widget.NewSelect([]string{"text", "file"}, nil)
		selected := field.Type
		if selected == "" {
			selected = "text"
		}
		kind.SetSelected(selected)
		kind.OnChanged = func(text string) { v.formFields[i].Type = text; v.changed(true); v.refreshFormFields() }
		name.OnChanged = func(text string) { v.formFields[i].Name = text; v.changed(true) }
		value.OnChanged = func(text string) {
			if v.formFields[i].Type == "file" {
				v.formFields[i].FilePath = text
			} else {
				v.formFields[i].Value = text
			}
			v.changed(true)
		}
		enabled.OnChanged = func(checked bool) { v.formFields[i].Disabled = !checked; v.changed(true) }
		id := field.ID
		browse := widget.NewButton("Choose file…", func() { v.chooseUploadFile(id) })
		if field.Type != "file" || v.bodyType.Selected != httpclient.BodyMultipart {
			browse.Disable()
		}
		remove := widget.NewButton("Remove", func() {
			v.formFields = append(v.formFields[:i], v.formFields[i+1:]...)
			v.changed(true)
			v.refreshFormFields()
		})
		row := container.NewBorder(nil, nil, container.NewHBox(enabled, kind), container.NewHBox(browse, remove), container.NewGridWithColumns(2, name, value))
		v.formBox.Add(row)
	}
	if len(v.formFields) == 0 {
		v.formBox.Add(widget.NewLabel("No form fields. Click Add field to create one."))
	}
	v.formBox.Refresh()
}

func (v *requestEditor) chooseUploadFile(id string) {
	chooseUploadPath(v.window, func(path string, err error) {
		if v.closed {
			return
		}
		if err != nil {
			dialog.ShowError(err, v.window)
			return
		}
		if path != "" {
			v.setUploadFile(id, path)
		}
	})
}

func (v *requestEditor) setUploadFile(id, path string) {
	if v.closed {
		return
	}
	for i := range v.formFields {
		if v.formFields[i].ID == id && v.formFields[i].Type == "file" {
			v.formFields[i].FilePath = path
			v.changed(true)
			v.refreshFormFields()
			return
		}
	}
}
