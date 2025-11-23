package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof" // Register pprof handlers
	"os"
	"path/filepath"

	"github.com/getlantern/systray"
	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/cli"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/infra/bridge"
)

var globalApp *app.App

func main() {
	cli.LaunchFunc = LaunchDaemon
	cli.Execute()
}

// LaunchDaemon is called by the CLI to launch the daemon.
func LaunchDaemon(configPath string) {
	// Start pprof server if enabled via environment variable
	// Usage: NERU_PPROF=:6060 neru daemon
	if pprofAddr := os.Getenv("NERU_PPROF"); pprofAddr != "" {
		go func() {
			fmt.Fprintf(os.Stderr, "Starting pprof server on %s\\n", pprofAddr)
			if err := http.ListenAndServe(pprofAddr, nil); err != nil {
				fmt.Fprintf(os.Stderr, "pprof server error: %v\\n", err)
			}
		}()
	}

	result := config.LoadWithValidation(configPath)

	// If there's a validation error, show alert and continue with default config
	if result.ValidationError != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Configuration validation failed: %v\\n", result.ValidationError)
		fmt.Fprintf(os.Stderr, "Config file: %s\\n", result.ConfigPath)
		fmt.Fprintf(os.Stderr, "Continuing with default configuration...\\n\\n")

		// Show native macOS alert dialog asynchronously
		// We use a goroutine and delay to ensure the main run loop has started
		// otherwise the alert might hang or not show up
		go func() {
			absPath, _ := filepath.Abs(result.ConfigPath)
			showConfigErrorAlert(result.ValidationError.Error(), absPath)
		}()
	}

	a, err := app.New(result.Config, result.ConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating app: %v\\n", err)
		os.Exit(1)
	}

	globalApp = a

	go func() {
		err := a.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running app: %v\\n", err)
		}
	}()

	systray.Run(onReady, onExit)
}

// showConfigErrorAlert displays a native macOS alert for config validation errors.
func showConfigErrorAlert(errorMessage, configPath string) {
	bridge.ShowConfigValidationError(errorMessage, configPath)
}
