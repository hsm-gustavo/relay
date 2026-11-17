//go:build linux && !android

package main

import (
	"fmt"
	"io"
	"os"

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

func chooseResponseWriter(window fyne.Window, name string, callback func(io.WriteCloser, error)) {
	go func() {
		uris, err := filechooser.SaveFile("", "Save response", &filechooser.SaveFileOptions{CurrentName: name, AcceptLabel: "Save"})
		if err != nil {
			fyne.Do(func() { chooseFyneResponseWriter(window, name, callback) })
			return
		}
		if len(uris) == 0 {
			fyne.Do(func() { callback(nil, nil) })
			return
		}
		uri, err := storage.ParseURI(uris[0])
		if err == nil && uri.Scheme() != "file" {
			err = fmt.Errorf("choose a local file for the response")
		}
		var writer *os.File
		if err == nil {
			writer, err = os.OpenFile(uri.Path(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		}
		fyne.Do(func() {
			if writer == nil {
				callback(nil, err)
			} else {
				callback(writer, err)
			}
		})
	}()
}
