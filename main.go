package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"senda/internal/app"
	"senda/internal/store"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	backend := app.NewApp()
	wailsApp := application.New(application.Options{
		Name:        "Senda",
		Description: "API Development Tool",
		Services: []application.Service{
			application.NewService(backend),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})
	backend.SetApp(wailsApp) // native dialogs need the running app handle

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Senda",
		Width:     1280,
		Height:    820,
		MinWidth:  900,
		MinHeight: 600,
	})

	err := wailsApp.Run()

	// Fold any archive-backed collection edits back into their .zip on exit.
	if perr := store.PackOpen(); perr != nil {
		log.Printf("pack archives on shutdown: %v", perr)
	}

	if err != nil {
		log.Fatal(err)
	}
}
