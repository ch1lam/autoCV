package main

import (
	"context"
	"embed"
	"log"

	"github.com/ch1lam/autocv/internal/adapters/configfile"
	"github.com/ch1lam/autocv/internal/adapters/filesystem"
	"github.com/ch1lam/autocv/internal/adapters/sqlite"
	appservice "github.com/ch1lam/autocv/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	paths, err := filesystem.DefaultPaths()
	if err != nil {
		log.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		log.Fatal(err)
	}
	if _, err := configfile.New(paths.Config).LoadOrCreate(); err != nil {
		log.Fatal(err)
	}

	db, err := sqlite.Open(context.Background(), paths.Database)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	app := application.New(application.Options{
		Name:        "AutoCV",
		Description: "Local-first AI resume workbench",
		Services: []application.Service{
			application.NewService(appservice.NewHealthService()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "AutoCV",
		Width:     1487,
		Height:    1058,
		MinWidth:  1100,
		MinHeight: 720,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 52,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(247, 245, 240),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
