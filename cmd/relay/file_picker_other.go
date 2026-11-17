//go:build !linux || android

package main

import (
	"fyne.io/fyne/v2"
	"io"
)

func chooseUploadPath(window fyne.Window, callback func(string, error)) {
	chooseFyneUploadFile(window, callback)
}

func chooseResponseWriter(window fyne.Window, name string, callback func(io.WriteCloser, error)) {
	chooseFyneResponseWriter(window, name, callback)
}
