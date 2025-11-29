package main

import (
	"github.com/atotto/clipboard"
	"github.com/getlantern/systray"
	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/cli"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"go.uber.org/zap"
)

func onReady() {
	systray.SetTitle("⌨️")
	systray.SetTooltip("Neru - Keyboard Navigation")

	mVersion := systray.AddMenuItem("Version "+cli.Version, "Show version")
	mVersion.Disable()

	mVersionCopy := systray.AddMenuItem("Copy version", "Copy version to clipboard")

	systray.AddSeparator()

	mStatus := systray.AddMenuItem("Status: Running", "Show current status")
	mStatus.Disable()

	mToggle := systray.AddMenuItem("Disable", "Disable/Enable Neru without quitting")

	systray.AddSeparator()

	mHints := systray.AddMenuItem("Hints", "Hint mode actions")
	if globalApp != nil && !globalApp.HintsEnabled() {
		mHints.Hide()
	}

	mGrid := systray.AddMenuItem("Grid", "Grid mode actions")
	if globalApp != nil && !globalApp.GridEnabled() {
		mGrid.Hide()
	}

	mAction := systray.AddMenuItem("Action", "Action mode for mouse operations")

	mScroll := systray.AddMenuItem("Scroll", "Scroll mode for vim-style scrolling")

	systray.AddSeparator()

	mReloadConfig := systray.AddMenuItem("Reload Config", "Reload configuration from disk")

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit Neru", "Exit the application")

	go handleSystrayEvents(
		mVersionCopy, mStatus, mToggle,
		mHints, mGrid, mAction, mScroll, mReloadConfig,
		mQuit,
	)
}

func handleSystrayEvents(
	mVersionCopy, mStatus, mToggle *systray.MenuItem,
	mHints, mGrid, mAction, mScroll, mReloadConfig *systray.MenuItem,
	mQuit *systray.MenuItem,
) {
	for {
		select {
		case <-mVersionCopy.ClickedCh:
			handleVersionCopy()
		case <-mToggle.ClickedCh:
			handleToggleEnable(mStatus, mToggle)
		case <-mHints.ClickedCh:
			activateModeFromSystray(app.ModeHints)
		case <-mGrid.ClickedCh:
			activateModeFromSystray(app.ModeGrid)
		case <-mAction.ClickedCh:
			activateModeFromSystray(app.ModeAction)
		case <-mScroll.ClickedCh:
			activateModeFromSystray(app.ModeScroll)
		case <-mReloadConfig.ClickedCh:
			handleReloadConfig()
		case <-mQuit.ClickedCh:
			systray.Quit()

			return
		}
	}
}

func handleVersionCopy() {
	writeToClipboardErr := clipboard.WriteAll(cli.Version)
	if writeToClipboardErr != nil {
		logger.Error("Error copying version to clipboard", zap.Error(writeToClipboardErr))
	}
}

func handleToggleEnable(mStatus, mToggle *systray.MenuItem) {
	if globalApp == nil {
		return
	}

	if globalApp.IsEnabled() {
		globalApp.SetEnabled(false)
		mStatus.SetTitle("Status: Disabled")
		mToggle.SetTitle("Enable")
	} else {
		globalApp.SetEnabled(true)
		mStatus.SetTitle("Status: Enabled")
		mToggle.SetTitle("Disable")
	}
}

func activateModeFromSystray(mode app.Mode) {
	if globalApp != nil {
		globalApp.ActivateMode(mode)
	}
}

func handleReloadConfig() {
	if globalApp == nil {
		return
	}

	configPath := globalApp.GetConfigPath()

	reloadConfigErr := globalApp.ReloadConfig(configPath)
	if reloadConfigErr != nil {
		logger.Error("Failed to reload config from systray", zap.Error(reloadConfigErr))
	} else {
		logger.Info("Configuration reloaded successfully from systray")
	}
}

func onExit() {
	if globalApp != nil {
		globalApp.Cleanup()
	}
}
