package systray

import (
	"context"
	"sync/atomic"

	"github.com/atotto/clipboard"
	"github.com/getlantern/systray"
	"github.com/y3owk1n/neru/internal/cli"
	"github.com/y3owk1n/neru/internal/core/domain"
	"go.uber.org/zap"
)

// AppInterface defines the interface that the systray component needs from the app.
type AppInterface interface {
	HintsEnabled() bool
	GridEnabled() bool
	IsEnabled() bool
	SetEnabled(enabled bool)
	ActivateMode(mode domain.Mode)
	GetConfigPath() string
	ReloadConfig(ctx context.Context, configPath string) error
	Cleanup()
	// OnEnabledStateChanged is called when the enabled state changes externally
	OnEnabledStateChanged(callback func(bool))
}

// Component encapsulates systray functionality.
type Component struct {
	app    AppInterface
	logger *zap.Logger

	// Context for goroutine lifecycle management
	ctx    context.Context //nolint:containedctx // Used for proper goroutine cancellation
	cancel context.CancelFunc

	// Menu items
	mVersion       *systray.MenuItem
	mVersionCopy   *systray.MenuItem
	mStatus        *systray.MenuItem
	mToggleDisable *systray.MenuItem
	mToggleEnable  *systray.MenuItem
	mHints         *systray.MenuItem
	mGrid          *systray.MenuItem
	mReloadConfig  *systray.MenuItem
	mQuit          *systray.MenuItem

	// Channel for state updates (thread-safe communication)
	stateUpdateChan chan bool
	chanClosed      atomic.Bool
}

// NewComponent creates a new systray component.
func NewComponent(app AppInterface, logger *zap.Logger) *Component {
	ctx, cancel := context.WithCancel(context.Background())
	component := &Component{
		app:             app,
		logger:          logger,
		ctx:             ctx,
		cancel:          cancel,
		stateUpdateChan: make(chan bool, 1), // Buffered channel to avoid blocking
	}

	// Register callback immediately for state changes
	app.OnEnabledStateChanged(func(enabled bool) {
		// Don't send if channel is closed
		if component.chanClosed.Load() {
			return
		}
		// Send to channel (non-blocking)
		select {
		case component.stateUpdateChan <- enabled:
		default:
			// Channel is full, skip this update
		}
	})

	return component
}

// OnReady sets up the systray menu when the systray is ready.
func (c *Component) OnReady() {
	systray.SetTitle("⌨️")
	// Tooltip will be updated by updateMenuItems

	c.mVersion = systray.AddMenuItem("Version "+cli.Version, "Show version")
	c.mVersion.Disable()

	c.mVersionCopy = systray.AddMenuItem("Copy version", "Copy version to clipboard")

	systray.AddSeparator()

	c.mStatus = systray.AddMenuItem("Status: Running", "Show current status")
	c.mStatus.Disable()

	c.mToggleDisable = systray.AddMenuItem("Disable", "Disable Neru without quitting")
	c.mToggleEnable = systray.AddMenuItem("Enable", "Enable Neru")
	c.mToggleEnable.Hide() // Initially hide the enable option

	systray.AddSeparator()

	c.mHints = systray.AddMenuItem("Hints", "Hint mode actions")
	if !c.app.HintsEnabled() {
		c.mHints.Hide()
	}

	c.mGrid = systray.AddMenuItem("Grid", "Grid mode actions")
	if !c.app.GridEnabled() {
		c.mGrid.Hide()
	}

	systray.AddSeparator()

	c.mReloadConfig = systray.AddMenuItem("Reload Config", "Reload configuration from disk")

	systray.AddSeparator()

	c.mQuit = systray.AddMenuItem("Quit Neru", "Exit the application")

	// Initialize menu items with current state
	c.updateMenuItems(c.app.IsEnabled())

	go c.handleEvents()
}

// OnExit handles systray exit.
func (c *Component) OnExit() {
	c.chanClosed.Store(true) // Prevent further sends to channel
	c.cancel()               // Signal goroutine to stop
	c.app.Cleanup()
}

// Close cleans up systray component resources without triggering app cleanup.
// This is used during initialization failure cleanup to avoid double cleanup.
func (c *Component) Close() {
	c.chanClosed.Store(true) // Prevent further sends to channel
	c.cancel()               // Signal goroutine to stop
	// Note: We don't call c.app.Cleanup() here to avoid double cleanup during init failure
}

// updateMenuItems updates the systray menu items based on the current enabled state.
func (c *Component) updateMenuItems(enabled bool) {
	// Update tooltip, icon, and status menu item to show current status
	if enabled {
		systray.SetTitle("⌨️")
		systray.SetTooltip("Neru - Running")
		c.mStatus.SetTitle("Status: Running")
		c.mToggleDisable.Show()
		c.mToggleEnable.Hide()
	} else {
		systray.SetTitle("⏸️")
		systray.SetTooltip("Neru - Disabled")
		c.mStatus.SetTitle("Status: Disabled")
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
		case <-c.mReloadConfig.ClickedCh:
			c.handleReloadConfig()
		case <-c.mQuit.ClickedCh:
			systray.Quit()

			return
		case enabled := <-c.stateUpdateChan:
			c.updateMenuItems(enabled)
		}
	}
}

// handleVersionCopy copies the version to clipboard.
func (c *Component) handleVersionCopy() {
	writeToClipboardErr := clipboard.WriteAll(cli.Version)
	if writeToClipboardErr != nil {
		c.logger.Error("Error copying version to clipboard", zap.Error(writeToClipboardErr))
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

	reloadConfigErr := c.app.ReloadConfig(context.Background(), configPath)
	if reloadConfigErr != nil {
		c.logger.Error("Failed to reload config from systray", zap.Error(reloadConfigErr))
	} else {
		c.logger.Info("Configuration reloaded successfully from systray")
	}
}
