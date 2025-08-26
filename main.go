package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"

	"execp2p/internal/app"
	"execp2p/internal/config"
	"execp2p/internal/logger"
	"execp2p/internal/platform"
	"execp2p/internal/wailsbridge"

	"github.com/spf13/cobra"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

var (
	version = "1.0.4-e2e"

	rootCmd = &cobra.Command{
		Use:     "execp2p",
		Short:   "A GUI-based post-quantum end-to-end encrypted chat application.",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApp()
		},
	}

	// CLI global flags
	logLevelFlag string
)

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevelFlag, "log-level", "", "Set log level (debug, info, warn, error). Overrides $EXECP2P_LOG_LEVEL")

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if logLevelFlag != "" {
			// apply user-provided level
			lvl := logger.ParseLevel(logLevelFlag)
			logger.SetLevel(lvl)
			logger.L().Info("Log level set via CLI flag", "level", logLevelFlag)
		}
	}
}

func main() {
	// silence all logging to keep chat interface clean
	log.SetOutput(io.Discard)
	log.SetFlags(0)

	// Webview must run on the main OS thread (wymóg Wails)
	runtime.LockOSThread()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runApp() error {
	cfg := config.DefaultConfig()

	// Inicjalizacja back-endu ExecP2P
	entApp, err := app.NewExecP2P(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize ExecP2P: %w", err)
	}
	defer entApp.Close()

	// Tworzenie mostu Wails-ExecP2P
	bridge := wailsbridge.NewBridge(entApp)

	// Uruchomienie Wails
	// Inicjalizacja ustawień specyficznych dla platformy
	if err := platform.InitPlatform(); err != nil {
		logger.L().Warn("Failed to initialize platform-specific settings", "err", err)
	}

	err = wails.Run(&options.App{
		Title:  "ExecP2P",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 18, A: 1},
		OnStartup: func(ctx context.Context) {
			logger.L().Info("Application starting", "os", platform.GetOSName(), "arch", runtime.GOARCH)
			bridge.SetContext(ctx)
		},
		Bind: []interface{}{
			bridge,
		},
	})

	if err != nil {
		return fmt.Errorf("failed to run Wails: %w", err)
	}

	return nil
}
