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

	// If there's a validation error, show alert and exit
	if configResult.ValidationError != nil {
		handleConfigValidationError(configResult)
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

	err := newDaemonHost().Run(app)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
		os.Exit(1)
	}
}

// handleConfigValidationError shows a validation error and exits.
// From an app bundle it displays a native alert; from a terminal it prints to stderr.
func handleConfigValidationError(result *config.LoadResult) {
	errMsg := result.ValidationError.Error()
	cfgPath := result.ConfigPath
	fmt.Fprintf(os.Stderr, "⚠️  Configuration validation failed: %v\n", result.ValidationError)
	fmt.Fprintf(os.Stderr, "Config file: %s\n", cfgPath)
	fmt.Fprintf(os.Stderr, "Please fix the configuration and relaunch Neru.\n")

	if cli.IsRunningFromAppBundle() {
		absPath, _ := filepath.Abs(cfgPath)
		_ = platform.ShowConfigValidationErrorAlert(errMsg, absPath)
	}

	os.Exit(1)
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
		default:
			fmt.Fprintf(
				os.Stderr,
				"Unexpected onboarding alert response (%d), continuing with defaults\n",
				choice,
			)

			return false
		}
	}

	fmt.Fprintf(os.Stderr, "No config file found. Create one with: neru config init\n")

	os.Exit(1)

	return false
}
