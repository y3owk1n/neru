package systray

import (
	"context"
	"sync/atomic"

	"github.com/atotto/clipboard"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/buildinfo"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/ports"
)

// AppInterface defines the interface that the systray component needs from the app.
type AppInterface interface {
	HintsEnabled() bool
	GridEnabled() bool
	RecursiveGridEnabled() bool
	IsEnabled() bool
	SetEnabled(enabled bool)
	ToggleEnabled()
	ActivateMode(mode domain.Mode)
	GetConfigPath() string
	ReloadConfig(ctx context.Context, configPath string) error
	// OnEnabledStateChanged is called when the enabled state changes externally
	// Returns a subscription ID that can be used to unsubscribe
	OnEnabledStateChanged(callback func(bool)) uint64
	// OffEnabledStateChanged unsubscribes a callback by ID
	OffEnabledStateChanged(id uint64)
	// Overlay screen share visibility
	IsOverlayHiddenForScreenShare() bool
	SetOverlayHiddenForScreenShare(hide bool)
	ToggleOverlayHiddenForScreenShare() bool
	OnScreenShareStateChanged(callback func(bool)) uint64
	OffScreenShareStateChanged(id uint64)
	// Scroll invert toggle
	IsScrollInverted() bool
	SetScrollInverted(inverted bool)
	ToggleScrollInvert() bool
	OnScrollInvertStateChanged(callback func(bool)) uint64
	OffScrollInvertStateChanged(id uint64)
	// Quit triggers a graceful shutdown of the application.
	Quit()
}

// Component encapsulates systray functionality.
type Component struct {
	app    AppInterface
	tray   ports.SystrayPort
	system ports.SystemPort
	logger *zap.Logger

	// Context for goroutine lifecycle management
	ctx    context.Context //nolint:containedctx // Used for proper goroutine cancellation
	cancel context.CancelFunc

	// Menu items
	mVersionCopy        ports.SystrayMenuItem
	mToggleDisable      ports.SystrayMenuItem
	mToggleEnable       ports.SystrayMenuItem
	mToggleScreenShare  ports.SystrayMenuItem
	mToggleScrollInvert ports.SystrayMenuItem
	mModes              ports.SystrayMenuItem
	mHints              ports.SystrayMenuItem
	mGrid               ports.SystrayMenuItem
	mRecursiveGrid      ports.SystrayMenuItem
	mConfig             ports.SystrayMenuItem
	mReloadConfig       ports.SystrayMenuItem
	mOpenConfig         ports.SystrayMenuItem
	mHelp               ports.SystrayMenuItem
	mSourceCode         ports.SystrayMenuItem
	mDocsConfig         ports.SystrayMenuItem
	mDocsCLI            ports.SystrayMenuItem
	mReportBug          ports.SystrayMenuItem
	mFeatureRequest     ports.SystrayMenuItem
	mDiscuss            ports.SystrayMenuItem
	mQuit               ports.SystrayMenuItem

	// State update signaling (thread-safe communication)
	stateUpdateSignal               chan struct{} // Signal that state changed
	latestState                     atomic.Bool   // Latest enabled state
	screenShareUpdateSignal         chan struct{} // Signal for screen share state changes
	latestScreenShareState          atomic.Bool   // Latest screen share hide state
	scrollInvertUpdateSignal        chan struct{} // Signal for scroll invert state changes
	latestScrollInvertState         atomic.Bool   // Latest scroll invert state
	chanClosed                      atomic.Bool
	enabledStateSubscriptionID      uint64 // ID for unsubscribing on cleanup
	screenShareStateSubscriptionID  uint64 // ID for screen share state unsubscription
	scrollInvertStateSubscriptionID uint64 // ID for scroll invert state unsubscription
}

// NewComponent creates a new systray component.
func NewComponent(
	app AppInterface,
	tray ports.SystrayPort,
	system ports.SystemPort,
	logger *zap.Logger,
) *Component {
	ctx, cancel := context.WithCancel(context.Background())
	component := &Component{
		app:                      app,
		tray:                     tray,
		system:                   system,
		logger:                   logger,
		ctx:                      ctx,
		cancel:                   cancel,
		stateUpdateSignal:        make(chan struct{}, 1),
		screenShareUpdateSignal:  make(chan struct{}, 1),
		scrollInvertUpdateSignal: make(chan struct{}, 1),
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

	// Register callback for scroll invert state changes
	component.scrollInvertStateSubscriptionID = app.OnScrollInvertStateChanged(func(inverted bool) {
		// Don't send if channel is closed
		if component.chanClosed.Load() {
			return
		}
		// Store latest state and signal update
		component.latestScrollInvertState.Store(inverted)

		select {
		case component.scrollInvertUpdateSignal <- struct{}{}:
		default:
			// Signal already pending, state will be read when processed
		}
	})

	return component
}

// OnReady sets up the systray menu when the systray is ready.
func (c *Component) OnReady() {
	c.mVersionCopy = c.tray.AddMenuItem("Version: " + buildinfo.Version)

	c.mHelp = c.tray.AddMenuItem("Help")
	c.mDocsConfig = c.mHelp.AddSubMenuItem("Config Docs")
	c.mDocsCLI = c.mHelp.AddSubMenuItem("CLI Docs")
	c.mHelp.AddSeparator()
	c.mSourceCode = c.mHelp.AddSubMenuItem("Source Code")
	c.mFeatureRequest = c.mHelp.AddSubMenuItem("Request Feature")
	c.mReportBug = c.mHelp.AddSubMenuItem("Report Bug")
	c.mDiscuss = c.mHelp.AddSubMenuItem("Community Discussion")

	c.tray.AddSeparator()

	c.mModes = c.tray.AddMenuItem("Activate Modes")

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

	c.tray.AddSeparator()

	c.mConfig = c.tray.AddMenuItem("Config")
	c.mReloadConfig = c.mConfig.AddSubMenuItem("Reload")
	c.mOpenConfig = c.mConfig.AddSubMenuItem("Open in Editor")

	c.mToggleDisable = c.tray.AddMenuItem("Pause Neru")
	c.mToggleEnable = c.tray.AddMenuItem("Resume Neru")
	c.mToggleEnable.Hide() // Initially hide the enable option

	c.tray.AddSeparator()

	c.mToggleScreenShare = c.tray.AddMenuItem("Screen Share: Visible")
	c.mToggleScrollInvert = c.tray.AddMenuItem("Scroll Invert: Off")

	c.tray.AddSeparator()

	c.mQuit = c.tray.AddMenuItem("Quit")

	// Clear text title once since we use an icon
	c.tray.SetTitle("")

	// Initialize all state-dependent UI elements
	c.updateMenuItems(c.app.IsEnabled())

	go c.handleEvents()
}

// OnExit handles systray exit.
//
// It tears down the component's own resources (goroutines, subscriptions) but
// does NOT call app.Cleanup(). Cleanup is owned by the daemon host so that
// waitForShutdown() and other post-systray code can still use the logger.
func (c *Component) OnExit() {
	// Order matters: chanClosed guard protects callback from sending during cleanup
	c.chanClosed.Store(true) // Prevent callback from sending to channel
	c.cancel()               // Signal event goroutine to stop
	c.app.OffEnabledStateChanged(c.enabledStateSubscriptionID)
	c.app.OffScreenShareStateChanged(c.screenShareStateSubscriptionID)
	c.app.OffScrollInvertStateChanged(c.scrollInvertStateSubscriptionID)
}

// Close cleans up systray component resources.
// This is used during initialization failure cleanup.
func (c *Component) Close() {
	// Order matters: chanClosed guard protects callback from sending during cleanup
	c.chanClosed.Store(true) // Prevent callback from sending to channel
	c.cancel()               // Signal event goroutine to stop
	c.app.OffEnabledStateChanged(c.enabledStateSubscriptionID)
	c.app.OffScreenShareStateChanged(c.screenShareStateSubscriptionID)
	c.app.OffScrollInvertStateChanged(c.scrollInvertStateSubscriptionID)
}

// updateMenuItems updates the systray menu items based on the current enabled state.
func (c *Component) updateMenuItems(enabled bool) {
	// Update icon, tooltip, and menu items to show current status
	iconBytes, isTemplate := trayIconFor(enabled)
	c.tray.SetIcon(iconBytes, isTemplate)

	if enabled {
		c.tray.SetTooltip("Neru - Running")
		c.mToggleDisable.Show()
		c.mToggleEnable.Hide()
	} else {
		c.tray.SetTooltip("Neru - Paused")
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
		case <-c.mVersionCopy.Clicked():
			c.handleVersionCopy()
		case <-c.mToggleDisable.Clicked():
			c.handleToggleEnable()
		case <-c.mToggleEnable.Clicked():
			c.handleToggleEnable()
		case <-c.mHints.Clicked():
			c.app.ActivateMode(domain.ModeHints)
		case <-c.mGrid.Clicked():
			c.app.ActivateMode(domain.ModeGrid)
		case <-c.mRecursiveGrid.Clicked():
			c.app.ActivateMode(domain.ModeRecursiveGrid)
		case <-c.mReloadConfig.Clicked():
			c.handleReloadConfig()
		case <-c.mOpenConfig.Clicked():
			go c.handleOpenConfig()
		case <-c.mSourceCode.Clicked():
			go func() {
				err := openExternal(c.ctx, "https://github.com/y3owk1n/neru")
				if err != nil {
					c.logger.Error("Failed to open repository", zap.Error(err))
				}
			}()
		case <-c.mDocsConfig.Clicked():
			go func() {
				err := openExternal(
					c.ctx,
					buildinfo.DocsURL("docs/CONFIGURATION.md", buildinfo.Version),
				)
				if err != nil {
					c.logger.Error("Failed to open configuration docs", zap.Error(err))
				}
			}()
		case <-c.mDocsCLI.Clicked():
			go func() {
				err := openExternal(c.ctx, buildinfo.DocsURL("docs/CLI.md", buildinfo.Version))
				if err != nil {
					c.logger.Error("Failed to open CLI docs", zap.Error(err))
				}
			}()
		case <-c.mFeatureRequest.Clicked():
			go func() {
				err := openExternal(
					c.ctx,
					"https://github.com/y3owk1n/neru/issues/new?template=feature_request.yml",
				)
				if err != nil {
					c.logger.Error("Failed to open feature request", zap.Error(err))
				}
			}()
		case <-c.mReportBug.Clicked():
			go func() {
				err := openExternal(
					c.ctx,
					"https://github.com/y3owk1n/neru/issues/new?template=bug_report.yml",
				)
				if err != nil {
					c.logger.Error("Failed to open bug report", zap.Error(err))
				}
			}()
		case <-c.mDiscuss.Clicked():
			go func() {
				err := openExternal(c.ctx, "https://github.com/y3owk1n/neru/discussions")
				if err != nil {
					c.logger.Error("Failed to open community discussion", zap.Error(err))
				}
			}()
		case <-c.mToggleScreenShare.Clicked():
			c.handleToggleScreenShare()
		case <-c.mToggleScrollInvert.Clicked():
			c.handleToggleScrollInvert()
		case <-c.mQuit.Clicked():
			c.app.Quit()

			return
		case <-c.stateUpdateSignal:
			c.updateMenuItems(c.latestState.Load())
		case <-c.screenShareUpdateSignal:
			c.updateScreenShareMenuItem(c.latestScreenShareState.Load())
		case <-c.scrollInvertUpdateSignal:
			c.updateScrollInvertMenuItem(c.latestScrollInvertState.Load())
		}
	}
}

// handleVersionCopy copies the version to clipboard.
func (c *Component) handleVersionCopy() {
	writeToClipboardErr := clipboard.WriteAll(buildinfo.Version)
	if writeToClipboardErr != nil {
		c.logger.Error("Error copying version to clipboard", zap.Error(writeToClipboardErr))
	} else {
		c.notify("Version copied to clipboard")
	}
}

// notify puts a short message in front of the user, and reports it when the
// platform could not. A tray toggle whose only feedback is a notification is
// silent twice over if the notification is dropped without a word — which is
// what a Linux session with no notification daemon does.
//
// It runs off the menu loop. Showing a notification is a session-bus round
// trip on Linux, and the loop it would park is the same one that carries every
// later menu click, Quit included; nothing here reads the result, so waiting
// for it buys the user nothing and costs them the menu.
//
// Only the failure is logged: the message is UI text, which never goes to a
// log (root AGENTS.md, Conventions).
func (c *Component) notify(message string) {
	if c.system == nil {
		return
	}

	go func() {
		err := c.system.ShowNotification(c.ctx, "Neru", message)
		if err != nil {
			c.logger.Warn("Could not show a tray notification", zap.Error(err))
		}
	}()
}

// handleToggleEnable toggles the enabled state of the application.
func (c *Component) handleToggleEnable() {
	// Atomically toggle the enabled state - the callback will update the menu items
	c.app.ToggleEnabled()
}

// handleOpenConfig opens the configuration file in the default editor.
func (c *Component) handleOpenConfig() {
	configPath := c.app.GetConfigPath()
	if configPath == "" {
		return
	}

	err := openExternal(c.ctx, configPath)
	if err != nil {
		c.logger.Error("Failed to open config file", zap.Error(err))
	}
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
	// Atomically toggle - the callback will update the menu item
	newState := c.app.ToggleOverlayHiddenForScreenShare()

	status := "visible"
	if newState {
		status = "hidden"
	}

	c.notify("Screen share visibility: " + status)
}

// updateScreenShareMenuItem updates the screen share menu item text based on state.
func (c *Component) updateScreenShareMenuItem(hidden bool) {
	if hidden {
		c.mToggleScreenShare.SetTitle("Screen Share: Hidden")
	} else {
		c.mToggleScreenShare.SetTitle("Screen Share: Visible")
	}
}

// handleToggleScrollInvert toggles the scroll direction inversion.
func (c *Component) handleToggleScrollInvert() {
	// Atomically toggle - the callback will update the menu item
	newState := c.app.ToggleScrollInvert()

	status := "off"
	if newState {
		status = "on"
	}

	c.notify("Scroll invert: " + status)
}

// updateScrollInvertMenuItem updates the scroll invert menu item text based on state.
func (c *Component) updateScrollInvertMenuItem(inverted bool) {
	if inverted {
		c.mToggleScrollInvert.SetTitle("Scroll Invert: On")
	} else {
		c.mToggleScrollInvert.SetTitle("Scroll Invert: Off")
	}
}
