package main

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/hsm-gustavo/relay/internal/httpclient"
)

const previewRunes = 4096
const previewColumns = 120
const previewLines = 60

// Limit both total characters and row width: a minified HTML line otherwise
// causes TextGrid to allocate thousands of off-screen cells for every row.
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

func responsePreview(body []byte, raw bool) (string, string, bool) {
	sample := body
	large := len(body) > 64<<10
	if large && len(sample) > previewRunes*utf8.UTFMax+1 {
		sample = sample[:previewRunes*utf8.UTFMax+1]
	}
	text := httpclient.FormatBody(sample, !raw && !large)
	preview, clipped := boundedPreview(text)
	note := ""
	if clipped {
		note = fmt.Sprintf("Preview limited to %d characters. Copy response body copies the full retained body.", previewRunes)
	}
	if !raw && len(body) > 64<<10 {
		note = "Large body shown as a bounded raw preview. Copy response body copies the full retained body."
	}
	if !utf8.Valid(sample) {
		note = "Response contains binary or invalid UTF-8 data; the preview may show replacement characters."
	}
	return preview, note, !raw && !clipped
}
