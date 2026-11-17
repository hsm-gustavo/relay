package main

import (
	"encoding/json"
	"unicode"

	"fyne.io/fyne/v2/widget"
)

// colorJSON changes cell styles only: displayed text and clipboard contents stay intact.
// Raw mode, non-JSON bodies, and incomplete JSON in the request preview remain plain text.
func colorJSON(grid *widget.TextGrid, text string, enabled bool) {
	text, clipped := boundedPreview(text)
	grid.SetText(text)
	if clipped || !enabled || !json.Valid([]byte(text)) {
		return
	}
	// Scan the rendered cells so tabs and wide Unicode characters cannot misalign styles.
	var source []rune
	for _, row := range grid.Rows {
		for _, cell := range row.Cells {
			source = append(source, cell.Rune)
		}
		source = append(source, '\n')
	}
	styles := make([]*syntaxStyle, len(source))
	for i := 0; i < len(source); {
		start := i
		var style *syntaxStyle
		switch c := source[i]; {
		case c == '"':
			i++
			for i < len(source) {
				if source[i] == '\\' {
					i += 2
					continue
				}
				if source[i] == '"' {
					i++
					break
				}
				i++
			}
			next := i
			for next < len(source) && unicode.IsSpace(source[next]) {
				next++
			}
			if next < len(source) && source[next] == ':' {
				style = &syntaxStyle{role: blue, key: true}
			} else {
				style = &syntaxStyle{role: green}
			}
		case c == '-' || (c >= '0' && c <= '9'):
			for i < len(source) && isNumberRune(source[i]) {
				i++
			}
			style = &syntaxStyle{role: amber}
		case c == 't' || c == 'f' || c == 'n':
			for i < len(source) && source[i] >= 'a' && source[i] <= 'z' {
				i++
			}
			role := purple
			if c == 'n' {
				role = rose
			}
			style = &syntaxStyle{role: role}
		default:
			i++
		}
		for j := start; j < i && j < len(styles); j++ {
			styles[j] = style
		}
	}
	offset := 0
	for row := range grid.Rows {
		for col := range grid.Rows[row].Cells {
			if style := styles[offset]; style != nil {
				grid.Rows[row].Cells[col].Style = *style
			}
			offset++
		}
		offset++
	}
	grid.Refresh()
}

func isNumberRune(r rune) bool {
	return (r >= '0' && r <= '9') || r == '-' || r == '+' || r == '.' || r == 'e' || r == 'E'
}
