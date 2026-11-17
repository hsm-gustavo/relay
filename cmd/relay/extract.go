package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/hsm-gustavo/relay/internal/extraction"
)

func (v *requestEditor) extractionSettings() fyne.CanvasObject {
	v.extractionSummary = widget.NewLabel("No extraction has run in this tab yet.")
	v.rulesBox = container.NewVBox()
	v.extractionLog = container.NewVBox(widget.NewLabel("No extraction has run in this tab yet."))
	help := widget.NewLabel("Complete 2xx JSON → environment session variables. Primitive values only.")
	help.Wrapping = fyne.TextWrapWord
	add := widget.NewButton("Add rule", func() {
		v.extractRules = append(v.extractRules, extraction.Rule{})
		v.changed(false)
		v.refreshExtractionRules()
	})
	rules := container.NewBorder(container.NewBorder(nil, nil, nil, add, help), nil, nil, nil, container.NewVScroll(v.rulesBox))
	return container.NewBorder(v.extractionSummary, nil, nil, nil, container.NewAppTabs(container.NewTabItem("Rules", rules), container.NewTabItem("Last run", container.NewVScroll(v.extractionLog))))
}

func (v *requestEditor) refreshExtractionRules() {
	if v.rulesBox == nil {
		return
	}
	v.rulesBox.Objects = nil
	for i, rule := range v.extractRules {
		path := widget.NewEntry()
		path.SetPlaceHolder("$.access_token")
		path.SetText(rule.Path)
		variable := widget.NewEntry()
		variable.SetPlaceHolder("token")
		variable.SetText(rule.Variable)
		enabled := widget.NewCheck("Enabled", nil)
		enabled.SetChecked(!rule.Disabled)
		path.OnChanged = func(text string) { v.extractRules[i].Path = text; v.changed(false) }
		variable.OnChanged = func(text string) { v.extractRules[i].Variable = text; v.changed(false) }
		enabled.OnChanged = func(checked bool) { v.extractRules[i].Disabled = !checked; v.changed(false) }
		remove := widget.NewButton("Remove", func() {
			v.extractRules = append(v.extractRules[:i], v.extractRules[i+1:]...)
			v.changed(false)
			v.refreshExtractionRules()
		})
		fields := container.NewGridWithColumns(2, widget.NewForm(widget.NewFormItem("JSON path", path)), widget.NewForm(widget.NewFormItem("Variable", variable)))
		v.rulesBox.Add(container.NewBorder(nil, nil, enabled, remove, fields))
	}
	if len(v.extractRules) == 0 {
		v.rulesBox.Add(widget.NewLabel("No rules. Add one to extract a response value."))
	}
	v.rulesBox.Refresh()
}

func (v *requestEditor) recordExtractionResults(results []extraction.Result) {
	v.extractResults = results
	successful, failed, skipped := 0, 0, 0
	for _, result := range results {
		switch result.Status {
		case extraction.Success:
			successful++
		case extraction.Failed:
			failed++
		default:
			skipped++
		}
	}
	v.extractionSummary.Importance = widget.MediumImportance
	if successful > 0 {
		v.extractionSummary.Importance = widget.SuccessImportance
	}
	if failed > 0 {
		v.extractionSummary.Importance = widget.DangerImportance
	}
	v.extractionSummary.SetText(fmt.Sprintf("Last extraction: %d succeeded · %d failed · %d skipped/disabled", successful, failed, skipped))
	v.extractionLog.Objects = nil
	if len(results) == 0 {
		v.extractionLog.Add(widget.NewLabel("No extraction rules configured for this run."))
	}
	for _, result := range results {
		label := widget.NewLabel(fmt.Sprintf("%s → %s: %s\n%s", result.Rule.Path, result.Rule.Variable, result.Status, result.Message))
		label.Wrapping = fyne.TextWrapWord
		switch result.Status {
		case extraction.Success:
			label.Importance = widget.SuccessImportance
		case extraction.Failed:
			label.Importance = widget.DangerImportance
		default:
			label.Importance = widget.MediumImportance
		}
		v.extractionLog.Add(label)
	}
	v.extractionLog.Refresh()
}
