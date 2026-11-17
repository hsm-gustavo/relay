//go:build !linux || android

package main

import "fyne.io/fyne/v2"

func chooseUploadPath(window fyne.Window, callback func(string, error)) {
	chooseFyneUploadFile(window, callback)
}
