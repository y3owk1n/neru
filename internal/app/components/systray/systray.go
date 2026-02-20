package systray

import (
	"context"
	"os/exec"
	"strings"
	"sync/atomic"

	"github.com/atotto/clipboard"
	"github.com/y3owk1n/neru/internal/cli"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"github.com/y3owk1n/neru/internal/core/infra/systray"
	"go.uber.org/zap"
)

// AppInterface defines the interface that the systray component needs from the app.
type AppInterface interface {
	HintsEnabled() bool
	GridEnabled() bool
	RecursiveGridEnabled() bool
	IsEnabled() bool
	SetEnabled(enabled bool)
	ActivateMode(mode domain.Mode)
	GetConfigPath() string
	ReloadConfig(ctx context.Context, configPath string) error
	Cleanup()
	// OnEnabledStateChanged is called when the enabled state changes externally
	// Returns a subscription ID that can be used to unsubscribe
	OnEnabledStateChanged(callback func(bool)) uint64
	// OffEnabledStateChanged unsubscribes a callback by ID
	OffEnabledStateChanged(id uint64)
	// Overlay screen share visibility
	IsOverlayHiddenForScreenShare() bool
	SetOverlayHiddenForScreenShare(hide bool)
	OnScreenShareStateChanged(callback func(bool)) uint64
	OffScreenShareStateChanged(id uint64)
}

// Component encapsulates systray functionality.
type Component struct {
	app    AppInterface
	logger *zap.Logger

	// Context for goroutine lifecycle management
	ctx    context.Context //nolint:containedctx // Used for proper goroutine cancellation
	cancel context.CancelFunc

	// Menu items
	mVersionCopy       *systray.MenuItem
	mToggleDisable     *systray.MenuItem
	mToggleEnable      *systray.MenuItem
	mToggleScreenShare *systray.MenuItem
	mModes             *systray.MenuItem
	mHints             *systray.MenuItem
	mGrid              *systray.MenuItem
	mRecursiveGrid     *systray.MenuItem
	mReloadConfig      *systray.MenuItem
	mHelp              *systray.MenuItem
	mSourceCode        *systray.MenuItem
	mDocsConfig        *systray.MenuItem
	mDocsCLI           *systray.MenuItem
	mReportBug         *systray.MenuItem
	mFeatureRequest    *systray.MenuItem
	mDiscuss           *systray.MenuItem
	mQuit              *systray.MenuItem

	// State update signaling (thread-safe communication)
	stateUpdateSignal              chan struct{} // Signal that state changed
	latestState                    atomic.Bool   // Latest enabled state
	screenShareUpdateSignal        chan struct{} // Signal for screen share state changes
	latestScreenShareState         atomic.Bool   // Latest screen share hide state
	chanClosed                     atomic.Bool
	enabledStateSubscriptionID     uint64 // ID for unsubscribing on cleanup
	screenShareStateSubscriptionID uint64 // ID for screen share state unsubscription
}

// NewComponent creates a new systray component.
func NewComponent(app AppInterface, logger *zap.Logger) *Component {
	ctx, cancel := context.WithCancel(context.Background())
	component := &Component{
		app:                     app,
		logger:                  logger,
		ctx:                     ctx,
		cancel:                  cancel,
		stateUpdateSignal:       make(chan struct{}, 1),
		screenShareUpdateSignal: make(chan struct{}, 1),
	}

	// Register callback immediately for enabled state changes
	component.enabledStateSubscriptionID = app.OnEnabledStateChanged(func(enabled bool) {
		// Don't send if channel is closed
		if component.chanClosed.Load() {
			return
		}
		// Store latest state and signal update
		component.latestState.Store(enabled)

		select {
		case component.stateUpdateSignal <- struct{}{}:
		default:
			// Signal already pending, state will be read when processed
		}
	})

	// Register callback for screen share state changes
	component.screenShareStateSubscriptionID = app.OnScreenShareStateChanged(func(hidden bool) {
		// Don't send if channel is closed
		if component.chanClosed.Load() {
			return
		}
		// Store latest state and signal update
		component.latestScreenShareState.Store(hidden)

		select {
		case component.screenShareUpdateSignal <- struct{}{}:
		default:
			// Signal already pending, state will be read when processed
		}
	})

	return component
}

// OnReady sets up the systray menu when the systray is ready.
func (c *Component) OnReady() {
	c.mVersionCopy = systray.AddMenuItem("Version: " + cli.Version)

	c.mHelp = systray.AddMenuItem("Help")
	c.mDocsConfig = c.mHelp.AddSubMenuItem("Config Docs")
	c.mDocsCLI = c.mHelp.AddSubMenuItem("CLI Docs")
	c.mHelp.AddSeparator()
	c.mSourceCode = c.mHelp.AddSubMenuItem("Source Code")
	c.mFeatureRequest = c.mHelp.AddSubMenuItem("Request Feature")
	c.mReportBug = c.mHelp.AddSubMenuItem("Report Bug")
	c.mDiscuss = c.mHelp.AddSubMenuItem("Community Discussion")

	systray.AddSeparator()

	c.mModes = systray.AddMenuItem("Activate Modes")

	c.mHints = c.mModes.AddSubMenuItem("Hints")
	if !c.app.HintsEnabled() {
		c.mHints.SetTitle("Hints: Disabled")
		c.mHints.Disable()
	}

	c.mGrid = c.mModes.AddSubMenuItem("Grid")
	if !c.app.GridEnabled() {
		c.mGrid.SetTitle("Grid: Disabled")
		c.mGrid.Disable()
	}

	c.mRecursiveGrid = c.mModes.AddSubMenuItem("Recursive Grid")
	if !c.app.RecursiveGridEnabled() {
		c.mRecursiveGrid.SetTitle("Recursive Grid: Disabled")
		c.mRecursiveGrid.Disable()
	}

	systray.AddSeparator()

	c.mReloadConfig = systray.AddMenuItem("Reload Config")

	c.mToggleDisable = systray.AddMenuItem("Pause Neru")
	c.mToggleEnable = systray.AddMenuItem("Resume Neru")
	c.mToggleEnable.Hide() // Initially hide the enable option

	systray.AddSeparator()

	c.mToggleScreenShare = systray.AddMenuItem("Screen Share: Visible")

	c.mQuit = systray.AddMenuItem("Quit")

	// Initialize all state-dependent UI elements
	c.updateMenuItems(c.app.IsEnabled())

	go c.handleEvents()
}

// OnExit handles systray exit.
func (c *Component) OnExit() {
	// Order matters: chanClosed guard protects callback from sending during cleanup
	c.chanClosed.Store(true) // Prevent callback from sending to channel
	c.cancel()               // Signal event goroutine to stop
	c.app.OffEnabledStateChanged(c.enabledStateSubscriptionID)
	c.app.OffScreenShareStateChanged(c.screenShareStateSubscriptionID)
	c.app.Cleanup()
}

// Close cleans up systray component resources without triggering app cleanup.
// This is used during initialization failure cleanup to avoid double cleanup.
func (c *Component) Close() {
	// Order matters: chanClosed guard protects callback from sending during cleanup
	c.chanClosed.Store(true) // Prevent callback from sending to channel
	c.cancel()               // Signal event goroutine to stop
	c.app.OffEnabledStateChanged(c.enabledStateSubscriptionID)
	c.app.OffScreenShareStateChanged(c.screenShareStateSubscriptionID)
	// Note: We don't call c.app.Cleanup() here to avoid double cleanup during init failure
}

// updateMenuItems updates the systray menu items based on the current enabled state.
func (c *Component) updateMenuItems(enabled bool) {
	// Update icon, tooltip, and menu items to show current status
	if enabled {
		systray.SetTitle("⌨️")
		systray.SetTooltip("Neru - Running")
		c.mToggleDisable.Show()
		c.mToggleEnable.Hide()
	} else {
		systray.SetTitle("⏸️")
		systray.SetTooltip("Neru - Paused")
		c.mToggleDisable.Hide()
		c.mToggleEnable.Show()
	}
}

// handleEvents handles systray menu item events.
func (c *Component) handleEvents() {
	for {
		select {
		case <-c.ctx.Done():
			return // Context canceled, exit goroutine
		case <-c.mVersionCopy.ClickedCh:
			c.handleVersionCopy()
		case <-c.mToggleDisable.ClickedCh:
			c.handleToggleEnable()
		case <-c.mToggleEnable.ClickedCh:
			c.handleToggleEnable()
		case <-c.mHints.ClickedCh:
			c.app.ActivateMode(domain.ModeHints)
		case <-c.mGrid.ClickedCh:
			c.app.ActivateMode(domain.ModeGrid)
		case <-c.mRecursiveGrid.ClickedCh:
			c.app.ActivateMode(domain.ModeRecursiveGrid)
		case <-c.mReloadConfig.ClickedCh:
			c.handleReloadConfig()
		case <-c.mSourceCode.ClickedCh:
			go func() {
				err := exec.CommandContext(c.ctx, "/usr/bin/open", "https://github.com/y3owk1n/neru").
					Run()
				if err != nil {
					c.logger.Error("Failed to open repository", zap.Error(err))
				}
			}()
		case <-c.mDocsConfig.ClickedCh:
			go func() {
				err := exec.CommandContext(c.ctx, "/usr/bin/open", DocsURL("docs/CONFIGURATION.md", cli.Version)).
					Run()
				if err != nil {
					c.logger.Error("Failed to open configuration docs", zap.Error(err))
				}
			}()
		case <-c.mDocsCLI.ClickedCh:
			go func() {
				err := exec.CommandContext(c.ctx, "/usr/bin/open", DocsURL("docs/CLI.md", cli.Version)).
					Run()
				if err != nil {
					c.logger.Error("Failed to open CLI docs", zap.Error(err))
				}
			}()
		case <-c.mFeatureRequest.ClickedCh:
			go func() {
				err := exec.CommandContext(c.ctx, "/usr/bin/open", "https://github.com/y3owk1n/neru/issues/new?template=feature_request.yml").
					Run()
				if err != nil {
					c.logger.Error("Failed to open feature request", zap.Error(err))
				}
			}()
		case <-c.mReportBug.ClickedCh:
			go func() {
				err := exec.CommandContext(c.ctx, "/usr/bin/open", "https://github.com/y3owk1n/neru/issues/new?template=bug_report.yml").
					Run()
				if err != nil {
					c.logger.Error("Failed to open bug report", zap.Error(err))
				}
			}()
		case <-c.mDiscuss.ClickedCh:
			go func() {
				err := exec.CommandContext(c.ctx, "/usr/bin/open", "https://github.com/y3owk1n/neru/discussions").
					Run()
				if err != nil {
					c.logger.Error("Failed to open community discussion", zap.Error(err))
				}
			}()
		case <-c.mToggleScreenShare.ClickedCh:
			c.handleToggleScreenShare()
		case <-c.mQuit.ClickedCh:
			systray.Quit()

			return
		case <-c.stateUpdateSignal:
			c.updateMenuItems(c.latestState.Load())
		case <-c.screenShareUpdateSignal:
			c.updateScreenShareMenuItem(c.latestScreenShareState.Load())
		}
	}
}

const docsVersionSegments = 3

// DocsURL returns the documentation URL for a given path and version.
func DocsURL(path, version string) string {
	tag := extractDocsTag(version)
	if tag == "" {
		tag = "main"
	}

	return "https://github.com/y3owk1n/neru/blob/" + tag + "/" + path
}

func extractDocsTag(version string) string {
	if version == "" {
		return ""
	}

	if !strings.HasPrefix(version, "v") {
		return ""
	}

	if idx := strings.Index(version, "-"); idx != -1 {
		version = version[:idx]
	}

	parts := strings.Split(version[1:], ".")
	if len(parts) != docsVersionSegments {
		return ""
	}

	for _, part := range parts {
		if part == "" {
			return ""
		}

		for _, ch := range part {
			if ch < '0' || ch > '9' {
				return ""
			}
		}
	}

	return version
}

// handleVersionCopy copies the version to clipboard.
func (c *Component) handleVersionCopy() {
	writeToClipboardErr := clipboard.WriteAll(cli.Version)
	if writeToClipboardErr != nil {
		c.logger.Error("Error copying version to clipboard", zap.Error(writeToClipboardErr))
	} else {
		bridge.ShowNotification("Neru", "Version copied to clipboard")
	}
}

// handleToggleEnable toggles the enabled state of the application.
func (c *Component) handleToggleEnable() {
	// Toggle the enabled state - the callback will update the menu items
	c.app.SetEnabled(!c.app.IsEnabled())
}

// handleReloadConfig reloads the configuration from disk.
func (c *Component) handleReloadConfig() {
	configPath := c.app.GetConfigPath()

	reloadConfigErr := c.app.ReloadConfig(c.ctx, configPath)
	if reloadConfigErr != nil {
		c.logger.Error("Failed to reload config from systray", zap.Error(reloadConfigErr))
	} else {
		c.logger.Info("Configuration reloaded successfully from systray")
	}
}

// handleToggleScreenShare toggles the overlay visibility in screen sharing.
func (c *Component) handleToggleScreenShare() {
	currentState := c.app.IsOverlayHiddenForScreenShare()
	newState := !currentState
	c.app.SetOverlayHiddenForScreenShare(newState)
	// Note: updateScreenShareMenuItem will be called via the state change callback

	status := "visible"
	if newState {
		status = "hidden"
	}

	bridge.ShowNotification("Neru", "Screen share visibility: "+status)
}

// updateScreenShareMenuItem updates the screen share menu item text based on state.
func (c *Component) updateScreenShareMenuItem(hidden bool) {
	if hidden {
		c.mToggleScreenShare.SetTitle("Screen Share: Hidden")
	} else {
		c.mToggleScreenShare.SetTitle("Screen Share: Visible")
	}
}
