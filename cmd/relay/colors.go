package main

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type colorRole string

const (
	neutral colorRole = ""
	green   colorRole = "green"
	blue    colorRole = "blue"
	amber   colorRole = "amber"
	purple  colorRole = "purple"
	red     colorRole = "red"
	cyan    colorRole = "cyan"
	rose    colorRole = "rose"
)

// Both palettes keep text legible on Fyne's corresponding default surfaces.
var darkPalette = map[colorRole]color.NRGBA{
	green: {0x73, 0xd4, 0x9b, 255}, blue: {0x82, 0xbb, 0xff, 255}, amber: {0xf2, 0xc6, 0x6d, 255},
	purple: {0xd4, 0xa3, 0xff, 255}, red: {0xff, 0x9a, 0x9a, 255}, cyan: {0x80, 0xd5, 0xe5, 255}, rose: {0xed, 0xaf, 0xc9, 255},
}
var lightPalette = map[colorRole]color.NRGBA{
	green: {0x16, 0x65, 0x34, 255}, blue: {0x1d, 0x4e, 0xd8, 255}, amber: {0x92, 0x40, 0x0e, 255},
	purple: {0x7e, 0x22, 0xce, 255}, red: {0xb9, 0x1c, 0x1c, 255}, cyan: {0x0e, 0x66, 0x75, 255}, rose: {0x9d, 0x17, 0x4d, 255},
}

func roleColor(role colorRole, variant fyne.ThemeVariant) color.Color {
	palette := darkPalette
	if variant == theme.VariantLight {
		palette = lightPalette
	}
	if c, ok := palette[role]; ok {
		return c
	}
	return theme.Current().Color(theme.ColorNameForeground, variant)
}

type textTheme struct {
	fyne.Theme
	role colorRole
}

func (t *textTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameForeground {
		return roleColor(t.role, variant)
	}
	return t.Theme.Color(name, variant)
}

func methodRole(method string) colorRole {
	switch method {
	case "GET":
		return green
	case "POST":
		return amber
	case "PUT":
		return blue
	case "PATCH":
		return purple
	case "DELETE":
		return red
	case "HEAD", "OPTIONS":
		return cyan
	default:
		return neutral
	}
}

func statusRole(status string) colorRole {
	switch {
	case strings.HasPrefix(status, "1"):
		return blue
	case strings.HasPrefix(status, "2"):
		return green
	case strings.HasPrefix(status, "3"):
		return cyan
	case strings.HasPrefix(status, "4"):
		return amber
	case strings.HasPrefix(status, "5"):
		return red
	default:
		return neutral
	}
}

type syntaxStyle struct {
	role colorRole
	key  bool
}

func (s syntaxStyle) Style() fyne.TextStyle { return fyne.TextStyle{Monospace: true, Bold: s.key} }
func (s syntaxStyle) TextColor() color.Color {
	return roleColor(s.role, fyne.CurrentApp().Settings().ThemeVariant())
}
func (s syntaxStyle) BackgroundColor() color.Color { return nil }

func colorHeaderKeys(grid *widget.TextGrid) {
	for row := range grid.Rows {
		end := -1
		for col, cell := range grid.Rows[row].Cells {
			if cell.Rune == ':' {
				end = col
				break
			}
		}
		for col := 0; col < end; col++ {
			grid.Rows[row].Cells[col].Style = syntaxStyle{role: cyan, key: true}
		}
	}
	grid.Refresh()
}
