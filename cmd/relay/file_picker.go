package main

import (
	"fmt"
	"io"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

func chooseFyneUploadFile(window fyne.Window, callback func(string, error)) {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			callback("", err)
			return
		}
		if reader == nil {
			return
		}
		uri := reader.URI()
		if err := reader.Close(); err != nil {
			callback("", err)
			return
		}
		if uri.Scheme() != "file" {
			callback("", fmt.Errorf("choose a local file for upload"))
			return
		}
		callback(uri.Path(), nil)
	}, window)
}

func chooseFyneResponseWriter(window fyne.Window, name string, callback func(io.WriteCloser, error)) {
	picker := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if writer == nil {
			callback(nil, err)
		} else {
			callback(writer, err)
		}
	}, window)
	picker.SetFileName(name)
	picker.Show()
}
