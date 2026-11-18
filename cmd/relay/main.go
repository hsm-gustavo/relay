package main

import (
	_ "embed"
	"fmt"
	"net/http"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"

	"github.com/hsm-gustavo/relay/internal/workspace"
)

//go:embed Icon.png
var iconPNG []byte

func main() {
	a := app.NewWithID("com.github.hsm-gustavo.relay")
	a.SetIcon(fyne.NewStaticResource("Icon.png", iconPNG))
	w := a.NewWindow("Relay")
	w.Resize(fyne.NewSize(1280, 800))
	path, err := workspace.DefaultPath()
	if err == nil {
		var store *workspace.Store
		store, err = workspace.Open(path)
		if err == nil {
			view := newWorkspaceView(w, &http.Client{}, store)
			w.SetContent(view.content)
			w.SetCloseIntercept(func() {
				if view.persist() {
					for _, editor := range view.editors {
						if editor.cancelRun != nil {
							editor.cancelRun()
						}
					}
					w.SetCloseIntercept(nil)
					w.Close()
				}
			})
		}
	}
	if err != nil {
		message := widget.NewLabel(fmt.Sprintf("Could not open local workspace: %v\n\nYour existing data has not been changed.\nFile: %s", err, path))
		message.Wrapping = fyne.TextWrapWord
		w.SetContent(message)
	}
	w.ShowAndRun()
}
