package main

import (
	"image/color"
	"io"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"github.com/hsm-gustavo/relay/internal/responseview"
	"github.com/mattn/go-runewidth"
)

// matchStyle is applied only to the current match, on top of JSON token colors.
type matchStyle struct{ syntaxStyle }

func (matchStyle) TextColor() color.Color       { return theme.Color(theme.ColorNameForeground) }
func (matchStyle) BackgroundColor() color.Color { return theme.Color(theme.ColorNameSelection) }

func (v *responseViewer) highlightMatch() {
	if v.current < 0 || v.current >= len(v.matches.Matches) {
		return
	}
	match := v.matches.Matches[v.current]
	position := v.page.Start
	width := v.owner.responseBody.TabWidth
	if width == 0 {
		width = 4
	}
	for row := range v.owner.responseBody.Rows {
		cells := v.owner.responseBody.Rows[row].Cells
		for col := 0; col < len(cells); {
			next := col + 1
			if cells[col].Rune == '\t' {
				next = col + width - col%width
			} else if size := runewidth.RuneWidth(cells[col].Rune); size > 1 {
				next = col + size
			}
			if position >= match.Start && position < match.End {
				for i := col; i < next && i < len(cells); i++ {
					cells[i].Style = matchStyle{}
				}
			}
			position++
			col = next
		}
		position++
	}
	v.owner.responseBody.Refresh()
}

func (v *responseViewer) saveResponse() {
	if v.model == nil {
		return
	}
	model := v.model
	chooseResponseWriter(v.owner.window, model.Filename(), func(writer io.WriteCloser, err error) {
		if err != nil {
			if !v.owner.closed {
				dialog.ShowError(err, v.owner.window)
			}
			return
		}
		if writer == nil {
			return
		}
		if v.model == model {
			v.save.Disable()
		}
		go func() {
			err := responseview.WriteBody(writer, model.Body)
			closeErr := writer.Close()
			if err == nil {
				err = closeErr
			}
			v.owner.dispatch(func() {
				if v.owner.closed {
					return
				}
				if v.model == model {
					v.save.Enable()
				}
				if err != nil {
					dialog.ShowError(err, v.owner.window)
					return
				}
				if v.model == model {
					message := "Saved original retained bytes."
					if model.Incomplete {
						message = "Saved partial response: retained bytes only, not a complete file."
					}
					if model.Note != "" {
						message = model.Note + "\n" + message
					}
					v.owner.previewNote.SetText(message)
					v.owner.previewNote.Show()
				}
			})
		}()
	})
}
