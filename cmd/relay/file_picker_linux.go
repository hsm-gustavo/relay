//go:build linux && !android

package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"github.com/rymdport/portal/filechooser"
)

// Prefer the desktop's native file chooser through XDG Desktop Portal. On
// desktops without a portal backend, retain Fyne's file chooser as a fallback.
func chooseUploadPath(window fyne.Window, callback func(string, error)) {
	go func() {
		uris, err := filechooser.OpenFile("", "Select file to upload", &filechooser.OpenFileOptions{AcceptLabel: "Select file"})
		fyne.Do(func() {
			if err != nil {
				chooseFyneUploadFile(window, callback)
				return
			}
			if len(uris) == 0 {
				return
			}
			uri, err := storage.ParseURI(uris[0])
			if err != nil {
				callback("", err)
				return
			}
			if uri.Scheme() != "file" {
				callback("", fmt.Errorf("choose a local file for upload"))
				return
			}
			callback(uri.Path(), nil)
		})
	}()
}
