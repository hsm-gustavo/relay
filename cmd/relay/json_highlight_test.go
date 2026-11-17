package main

import (
	"image/color"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func TestJSONHighlighting(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	grid := widget.NewTextGrid()
	text := `{"key": "value", "number": -1.2e+3, "bool": true, "null": null}`
	colorJSON(grid, text, true)
	roles := map[string]colorRole{`"key"`: blue, `"value"`: green, `-1.2e+3`: amber, `true`: purple, `null}`: rose}
	for token, want := range roles {
		start := -1
		runes := []rune(text)
		for i := 0; i+len([]rune(token)) <= len(runes); i++ {
			if string(runes[i:i+len([]rune(token))]) == token {
				start = i
				break
			}
		}
		style, ok := grid.Rows[0].Cells[start].Style.(syntaxStyle)
		if !ok || style.role != want {
			t.Errorf("%s style = %+v, want %s", token, style, want)
		}
	}
	if grid.Text() != text {
		t.Fatal("highlighting changed text")
	}
	// Escaped quotes, Unicode, nested arrays, and tab indentation must not confuse the lexer.
	for _, source := range []string{`{"a\"b":"true","é":[false,null,12345678901234567890]}`, "{\n\t\"key\": [1, 2]\n}"} {
		colorJSON(grid, source, true)
		if grid.Text() != source {
			t.Errorf("text changed: %q", grid.Text())
		}
	}
	for _, tt := range []struct {
		text    string
		enabled bool
	}{{text, false}, {"plain text", true}, {`{"invalid":}`, true}, {"", true}} {
		colorJSON(grid, tt.text, tt.enabled)
		for _, row := range grid.Rows {
			for _, cell := range row.Cells {
				if cell.Style != nil {
					t.Error("raw, invalid, or non-JSON text was styled")
				}
			}
		}
	}
}

func TestSemanticColorsAndContrast(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	if methodRole("GET") != green || methodRole("DELETE") != red || statusRole("201 Created") != green || statusRole("404 Not Found") != amber || statusRole("500 Internal Server Error") != red {
		t.Fatal("incorrect semantic colors")
	}
	for _, variant := range []fyne.ThemeVariant{theme.VariantDark, theme.VariantLight} {
		for _, role := range []colorRole{green, blue, amber, purple, red, cyan, rose} {
			for _, surface := range []fyne.ThemeColorName{theme.ColorNameBackground, theme.ColorNameInputBackground} {
				foreground := luminance(roleColor(role, variant))
				background := luminance(theme.DefaultTheme().Color(surface, variant))
				ratio := (math.Max(foreground, background) + 0.05) / (math.Min(foreground, background) + 0.05)
				if ratio < 4.5 {
					t.Errorf("%s contrast on %s variant %d = %.2f", role, surface, variant, ratio)
				}
			}
		}
	}
}

func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	linear := func(v uint32) float64 {
		x := float64(v) / 65535
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b)
}
