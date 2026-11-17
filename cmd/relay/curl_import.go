package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/hsm-gustavo/relay/internal/curlimport"
	"github.com/hsm-gustavo/relay/internal/httpclient"
	"github.com/hsm-gustavo/relay/internal/workspace"
)

func (v *workspaceView) importCurl(command, name string) error {
	result, err := curlimport.Parse(command)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Imported cURL request"
	}
	v.openDraft(workspace.Draft{Item: workspace.Item{Name: name, Request: result.Request}})
	return nil
}

func (v *workspaceView) showCurlImport() {
	name := widget.NewEntry()
	name.SetText("Imported cURL request")
	command := widget.NewMultiLineEntry()
	command.TextStyle = fyne.TextStyle{Monospace: true}
	command.SetMinRowsVisible(8)
	command.SetPlaceHolder("curl 'https://example.com/users' -H 'Authorization: Bearer token'")
	preview := widget.NewTextGrid()
	preview.Scroll = fyne.ScrollBoth
	command.Validator = func(text string) error {
		result, err := curlimport.Parse(text)
		if err != nil {
			preview.SetText("")
			return err
		}
		auth := result.Request.Auth.Type
		if auth == "" {
			auth = "None"
		}
		timeout := result.Request.Timeout
		if timeout == "" {
			timeout = "30s (Relay default)"
		}
		body := fmt.Sprintf("%s, %d bytes", result.Request.EffectiveBodyType(), len(result.Request.Body))
		if result.Request.EffectiveBodyType() != httpclient.BodyJSON {
			body = fmt.Sprintf("%s, %d fields", result.Request.EffectiveBodyType(), len(result.Request.FormFields))
		}
		summary := fmt.Sprintf("%s %s\nBody: %s · Auth: %s · Timeout: %s\n\nReview before importing:\n", result.Request.Method, result.Request.URL, body, auth, timeout)
		for i, warning := range result.Warnings {
			if i >= 12 {
				summary += "Additional unsupported headers were omitted.\n"
				break
			}
			summary += "• " + warning + "\n"
		}
		text, _ = boundedPreview(summary)
		preview.SetText(text)
		return nil
	}
	help := widget.NewLabel("Paste one cURL command using POSIX-style quotes. Import opens an editable tab without sending a request. Review omitted headers below.")
	help.Wrapping = fyne.TextWrapWord
	form := dialog.NewForm("Import cURL", "Import", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Name", name), widget.NewFormItem("Command", command), widget.NewFormItem("Review", container.NewGridWrap(fyne.NewSize(580, 150), preview)), widget.NewFormItem("", help),
	}, func(ok bool) {
		if !ok {
			return
		}
		if err := v.importCurl(command.Text, name.Text); err != nil {
			dialog.ShowError(err, v.window)
		}
	}, v.window)
	form.Resize(fyne.NewSize(760, 620))
	form.Show()
}
