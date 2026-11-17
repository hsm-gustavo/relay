package main

import "strings"

const previewRunes = 4096
const previewColumns = 120
const previewLines = 60

// Bound editable-body and import previews. Responses use responseview.Window
// instead, preserving source text without inserting synthetic line breaks.
func boundedPreview(text string) (string, bool) {
	var out strings.Builder
	count, column, lines := 0, 0, 1
	clipped := false
	for _, r := range text {
		if count >= previewRunes || lines > previewLines {
			clipped = true
			break
		}
		if column >= previewColumns && r != '\n' {
			out.WriteByte('\n')
			column = 0
			lines++
			if lines > previewLines {
				clipped = true
				break
			}
		}

		out.WriteRune(r)
		count++
		if r == '\n' {
			column = 0
			lines++
		} else if r == '\t' {
			column += 8 - column%8
		} else {
			column++
		}
	}
	return out.String(), clipped
}
