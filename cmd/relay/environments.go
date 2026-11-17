package main

import (
	"fmt"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/hsm-gustavo/relay/internal/workspace"
)

const noEnvironment = "No environment"

func (v *workspaceView) refreshEnvironments() {
	options := []string{noEnvironment}
	selected := noEnvironment
	for _, env := range v.store.State.Environments {
		options = append(options, env.Name)
		if env.ID == v.store.State.ActiveEnvironmentID {
			selected = env.Name
		}
	}
	if selected == noEnvironment {
		v.store.State.ActiveEnvironmentID = ""
	}
	v.environments.Options = options
	v.environments.SetSelected(selected)
	v.environments.Refresh()
}

func (v *workspaceView) environmentBar() fyne.CanvasObject {
	edit := widget.NewButton("Edit environment", func() {
		if env := v.store.State.ActiveEnvironment(); env != nil {
			v.editEnvironment(env)
		}
	})
	remove := widget.NewButton("Delete environment", func() {
		env := v.store.State.ActiveEnvironment()
		if env == nil {
			return
		}
		id := env.ID
		dialog.ShowConfirm("Delete environment", "Delete “"+env.Name+"” and its variables? Saved request templates will be kept.", func(ok bool) {
			if !ok {
				return
			}
			v.store.State.DeleteEnvironment(id)
			v.refreshEnvironments()
			v.persist()
		}, v.window)
	})
	v.environments = widget.NewSelect(nil, func(name string) {
		v.store.State.ActiveEnvironmentID = ""
		for _, env := range v.store.State.Environments {
			if env.Name == name {
				v.store.State.ActiveEnvironmentID = env.ID
				break
			}
		}
		if v.store.State.ActiveEnvironment() != nil {
			edit.Enable()
			remove.Enable()
		} else {
			edit.Disable()
			remove.Disable()
		}
		if !v.loading {
			v.persist()
		}
	})
	v.environments.PlaceHolder = "Select environment"
	v.refreshEnvironments()
	return container.NewHBox(widget.NewLabel("Environment"), v.environments, widget.NewButton("New environment", func() { v.editEnvironment(nil) }), edit, remove, widget.NewButton("Import cURL", v.showCurlImport))
}

func (v *workspaceView) editEnvironment(env *workspace.Environment) {
	id := ""
	name := widget.NewEntry()
	variables := widget.NewMultiLineEntry()
	variables.SetMinRowsVisible(10)
	variables.TextStyle = fyne.TextStyle{Monospace: true}
	variables.SetPlaceHolder("base_url=http://localhost:8080\ntoken=your-token")
	if env != nil {
		id = env.ID
		name.SetText(env.Name)
		keys := make([]string, 0, len(env.Variables))
		for key := range env.Variables {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var lines []string
		for _, key := range keys {
			lines = append(lines, key+"="+env.Variables[key])
		}
		variables.SetText(strings.Join(lines, "\n"))
	}
	name.Validator = func(text string) error {
		text = strings.TrimSpace(text)
		if text == "" || strings.EqualFold(text, noEnvironment) {
			return fmt.Errorf("enter an environment name other than No environment")
		}
		for _, other := range v.store.State.Environments {
			if other.ID != id && strings.EqualFold(other.Name, text) {
				return fmt.Errorf("an environment with this name already exists")
			}
		}
		return nil
	}
	variables.Validator = func(text string) error { _, err := workspace.ParseVariables(text); return err }
	help := widget.NewLabel("One name=value per line. Use {{name}} in URLs, query params, JSON bodies, auth fields, or timeouts. Values are literal and case-sensitive.")
	help.Wrapping = fyne.TextWrapWord
	form := dialog.NewForm("Environment", "Save", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Name", name), widget.NewFormItem("Variables", variables), widget.NewFormItem("", help),
	}, func(ok bool) {
		if !ok {
			return
		}
		values, err := workspace.ParseVariables(variables.Text)
		if err != nil {
			dialog.ShowError(err, v.window)
			return
		}
		savedID, err := v.store.State.SaveEnvironment(id, name.Text, values)
		if err != nil {
			dialog.ShowError(err, v.window)
			return
		}
		v.store.State.ActiveEnvironmentID = savedID
		v.refreshEnvironments()
		v.persist()
	}, v.window)
	form.Resize(fyne.NewSize(660, 460))
	form.Show()
}
