package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type requestRow struct {
	widget.BaseWidget
	method, details *widget.Label
	colors          *textTheme
}

func newRequestRow(details string) *requestRow {
	row := &requestRow{method: widget.NewLabel("OPTIONS"), details: widget.NewLabel(details), colors: &textTheme{Theme: theme.DefaultTheme()}}
	row.method.TextStyle.Bold = true
	row.details.Truncation = fyne.TextTruncateEllipsis
	row.ExtendBaseWidget(row)
	return row
}

func (r *requestRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewBorder(nil, nil, container.NewThemeOverride(r.method, r.colors), nil, r.details))
}

func (r *requestRow) set(method, details string) {
	r.colors.role = methodRole(method)
	r.method.SetText(method)
	r.method.Refresh()
	r.details.SetText(details)
}
