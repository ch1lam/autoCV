package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/ch1lam/autocv/internal/adapters/configfile"
	"github.com/ch1lam/autocv/internal/adapters/fakeprovider"
	"github.com/ch1lam/autocv/internal/adapters/filesystem"
	"github.com/ch1lam/autocv/internal/adapters/keychain"
	"github.com/ch1lam/autocv/internal/adapters/logging"
	markdownparser "github.com/ch1lam/autocv/internal/adapters/markdown"
	"github.com/ch1lam/autocv/internal/adapters/openaiprovider"
	"github.com/ch1lam/autocv/internal/adapters/providerrouter"
	"github.com/ch1lam/autocv/internal/adapters/sqlite"
	"github.com/ch1lam/autocv/internal/adapters/systemclock"
	typstrenderer "github.com/ch1lam/autocv/internal/adapters/typst"
	"github.com/ch1lam/autocv/internal/adapters/wailsdialog"
	appservice "github.com/ch1lam/autocv/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "AutoCV failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	paths, err := filesystem.DefaultPaths()
	if err != nil {
		return err
	}
	if err := paths.Ensure(); err != nil {
		return err
	}

	logger, err := logging.NewFile(
		filepath.Join(paths.Logs, "autocv.log"),
		slog.LevelInfo,
	)
	if err != nil {
		return err
	}
	defer logger.Close()
	slog.SetDefault(logger.Logger)
	slog.Info("application.start")

	if _, err := configfile.New(paths.Config).LoadOrCreate(); err != nil {
		slog.Error("config.load.failed", slog.Any("error", err))
		return err
	}

	db, err := sqlite.Open(context.Background(), paths.Database)
	if err != nil {
		slog.Error("database.open.failed", slog.Any("error", err))
		return err
	}
	defer db.Close()

	managedFiles, err := filesystem.NewManagedFiles(paths.Root)
	if err != nil {
		slog.Error("managed.files.open.failed", slog.Any("error", err))
		return err
	}

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
	profileRepository := sqlite.NewProfileRepository(db)
	jdRepository := sqlite.NewJDRepository(db)
	matchRepository := sqlite.NewMatchRepository(db)
	providerConfigRepository := sqlite.NewProviderConfigRepository(db)
	providerCallRepository := sqlite.NewProviderCallRepository(db)
	secretStore := keychain.New("io.github.ch1lam.autocv")
	openAIProvider, err := openaiprovider.NewDynamicProvider(
		providerConfigRepository,
		secretStore,
		providerCallRepository,
		60*time.Second,
		1,
	)
	if err != nil {
		return err
	}
	exportPicker := wailsdialog.NewExportPicker(app)
	provider := providerrouter.New(
		providerConfigRepository,
		fakeprovider.New(),
		openAIProvider,
	)
	app.RegisterService(application.NewService(
		appservice.NewProviderControlService(provider),
	))
	app.RegisterService(application.NewService(appservice.NewSettingsService(
		providerConfigRepository,
		secretStore,
		systemclock.Clock{},
	)))
	app.RegisterService(application.NewService(appservice.NewProfileService(
		profileRepository,
		sqlite.NewProfileSearch(db),
		markdownparser.New(),
		provider,
		managedFiles,
		wailsdialog.NewMarkdownPicker(app),
		exportPicker,
		systemclock.Clock{},
	)))
	app.RegisterService(application.NewService(appservice.NewJDService(
		jdRepository,
		provider,
		systemclock.Clock{},
	)))
	resumeRepository := sqlite.NewResumeRepository(db)
	stageResultRepository := sqlite.NewStageResultRepository(db)
	clarificationRepository := sqlite.NewClarificationRepository(db)
	confirmationRepository := sqlite.NewRunConfirmationRepository(db)
	app.RegisterService(application.NewService(appservice.NewMatchService(
		matchRepository,
		resumeRepository,
		resumeRepository,
		stageResultRepository,
		clarificationRepository,
		confirmationRepository,
		profileRepository,
		jdRepository,
		provider,
		systemclock.Clock{},
	)))
	app.RegisterService(application.NewService(appservice.NewWorkflowService(
		resumeRepository,
		stageResultRepository,
	)))
	resumeService := appservice.NewResumeService(
		resumeRepository,
		stageResultRepository,
		confirmationRepository,
		matchRepository,
		profileRepository,
		jdRepository,
		provider,
		systemclock.Clock{},
	)
	app.RegisterService(application.NewService(resumeService))
	app.RegisterService(application.NewService(appservice.NewPDFService(
		resumeService,
		sqlite.NewArtifactRepository(db),
		managedFiles,
		typstrenderer.NewRenderer(
			os.Getenv("AUTOCV_TYPST_BIN"),
			20*time.Second,
		),
		exportPicker,
		systemclock.Clock{},
	)))

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
		slog.Error("application.run.failed", slog.Any("error", err))
		return err
	}
	slog.Info("application.stop")
	return nil
}
