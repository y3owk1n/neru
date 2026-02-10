// Package main is the entry point for the Neru application.
package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof" // Register pprof handlers
	"os"
	"path/filepath"
	"runtime"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/cli"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"github.com/y3owk1n/neru/internal/core/infra/systray"
	"go.uber.org/zap"
)

func main() {
	// Lock to main thread for macOS Cocoa - must be called before any goroutines
	runtime.LockOSThread()

	cli.LaunchFunc = LaunchDaemon

	cli.Execute()
}

// LaunchDaemon is called by the CLI to launch the daemon.
func LaunchDaemon(configPath string) {
	// Start pprof server if enabled via environment variable
	// Usage: NERU_PPROF=:6060 neru launch
	if pprofAddr := os.Getenv("NERU_PPROF"); pprofAddr != "" {
		go func() {
			fmt.Fprintf(os.Stderr, "Starting pprof server on %s\\n", pprofAddr)

			pprofServerErr := http.ListenAndServe(pprofAddr, nil)
			if pprofServerErr != nil {
				fmt.Fprintf(os.Stderr, "pprof server error: %v\\n", pprofServerErr)
			}
		}()
	}

	service := config.NewService(config.DefaultConfig(), configPath, zap.NewNop())
	configResult := service.LoadWithValidation(configPath)

	// If there's a validation error, show alert and continue with default config
	if configResult.ValidationError != nil {
		fmt.Fprintf(
			os.Stderr,
			"⚠️  Configuration validation failed: %v\\n",
			configResult.ValidationError,
		)
		fmt.Fprintf(os.Stderr, "Config file: %s\\n", configResult.ConfigPath)
		fmt.Fprintf(os.Stderr, "Continuing with default configuration...\\n\\n")

		// Show native macOS alert dialog asynchronously
		// We use a goroutine and delay to ensure the main run loop has started
		// otherwise the alert might hang or not show up
		go func() {
			absPath, _ := filepath.Abs(configResult.ConfigPath)
			showConfigErrorAlert(configResult.ValidationError.Error(), absPath)
		}()
	}

	app, appErr := app.New(
		app.WithConfig(configResult.Config),
		app.WithConfigPath(configResult.ConfigPath),
	)
	if appErr != nil {
		fmt.Fprintf(os.Stderr, "Error creating app: %v\\n", appErr)
		os.Exit(1)
	}

	go func() {
		err := app.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running app: %v\\n", err)
		}
	}()

	systrayComponent := app.GetSystrayComponent()
	if systrayComponent != nil {
		systray.Run(systrayComponent.OnReady, systrayComponent.OnExit)
	} else {
		// Run in headless mode (no status icon) if systray is disabled
		systray.RunHeadless(func() {}, func() {
			app.Cleanup()
		})
	}
}

// showConfigErrorAlert displays a native macOS alert for config validation errors.
func showConfigErrorAlert(errorMessage, configPath string) {
	bridge.ShowConfigValidationError(errorMessage, configPath)
}
