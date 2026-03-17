// Package main is the entry point for the Neru application.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/cli"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/infra/platform"
	"github.com/y3owk1n/neru/internal/core/infra/systray"
)

// main is defined in main_os.go files (main_darwin.go / main_other.go)
// so that platform-specific thread locking can be applied before any goroutines start.
// This file contains only the shared daemon logic.

type alertProvider struct {
	system config.AlertProvider
}

func newAlertProvider() *alertProvider {
	sp, err := platform.NewSystemPort()
	if err != nil {
		return &alertProvider{}
	}

	return &alertProvider{system: sp}
}

func (p *alertProvider) ShowAlert(ctx context.Context, title, message string) error {
	if p.system != nil {
		return p.system.ShowAlert(ctx, title, message)
	}

	return nil
}

// LaunchDaemon is called by the CLI to launch the daemon.
func LaunchDaemon(configPath string) {
	if !platform.IsDarwin() {
		fmt.Fprintf(
			os.Stderr,
			"⚠️  WARNING: Neru is running on %s, which is not yet fully supported.\n",
			platform.CurrentOS(),
		)
		fmt.Fprintf(
			os.Stderr,
			"   Most features (hotkeys, overlays, accessibility, notifications) are stubs\n",
		)
		fmt.Fprintf(os.Stderr, "   and will not function. Only macOS is currently supported.\n")
		fmt.Fprintf(
			os.Stderr,
			"   See docs/ARCHITECTURE.md for the contribution guide.\n\n",
		)
	}

	service := config.NewService(
		config.DefaultConfig(),
		configPath,
		zap.NewNop(),
		newAlertProvider(),
	)
	configResult := service.LoadWithValidation(configPath)

	// If there's a validation error, show alert and continue with default config
	if configResult.ValidationError != nil {
		fmt.Fprintf(
			os.Stderr,
			"⚠️  Configuration validation failed: %v\n",
			configResult.ValidationError,
		)
		fmt.Fprintf(os.Stderr, "Config file: %s\n", configResult.ConfigPath)
		fmt.Fprintf(os.Stderr, "Continuing with default configuration...\n\n")

		// Show native macOS alert dialog asynchronously
		// We use a goroutine and delay to ensure the main run loop has started
		// otherwise the alert might hang or not show up
		go func() {
			absPath, _ := filepath.Abs(configResult.ConfigPath)
			showConfigErrorAlert(configResult.ValidationError.Error(), absPath)
		}()
	}

	if configResult.ConfigPath == "" && configPath == "" {
		configResult = handleConfigOnboarding(service, configResult)
	}

	app, appErr := app.New(
		app.WithConfig(configResult.Config),
		app.WithConfigPath(configResult.ConfigPath),
	)
	if appErr != nil {
		fmt.Fprintf(os.Stderr, "Error creating app: %v\n", appErr)
		os.Exit(1)
	}

	go func() {
		err := app.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
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

// showConfigErrorAlert displays a native system alert for config validation errors.
func showConfigErrorAlert(errorMessage, configPath string) {
	provider := newAlertProvider()
	_ = provider.ShowAlert(context.Background(), errorMessage, configPath)
}

func handleConfigOnboarding(
	service *config.Service,
	configResult *config.LoadResult,
) *config.LoadResult {
	defaultPath, err := config.DefaultConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to determine default config path: %v\n", err)

		return configResult
	}

	if !promptConfigInit(defaultPath) {
		return configResult
	}

	err = config.WriteDefaultConfig(defaultPath, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create config file: %v\n", err)

		return configResult
	}

	return service.LoadWithValidation(defaultPath)
}

func promptConfigInit(configPath string) bool {
	if cli.IsRunningFromAppBundle() {
		absPath, _ := filepath.Abs(configPath)

		choice := platform.ShowConfigOnboardingAlert(absPath)
		switch choice {
		case platform.ConfigOnboardingCreate:
			return true
		case platform.ConfigOnboardingQuit:
			os.Exit(0)
		case platform.ConfigOnboardingDefaults:
			return false
		}

		return false
	}

	fmt.Fprintf(os.Stderr, "No config file found. Create one with: neru config init\n")

	os.Exit(1)

	return false
}
