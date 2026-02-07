package systray

import (
	"context"
	"os/exec"
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
	QuadGridEnabled() bool
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
	mVersionCopy   *systray.MenuItem
	mToggleDisable *systray.MenuItem
	mToggleEnable  *systray.MenuItem
	mModes         *systray.MenuItem
	mHints         *systray.MenuItem
	mGrid          *systray.MenuItem
	mQuadGrid      *systray.MenuItem
	mReloadConfig  *systray.MenuItem
	mHelp          *systray.MenuItem
	mSourceCode    *systray.MenuItem
	mDocsConfig    *systray.MenuItem
	mDocsCLI       *systray.MenuItem
	mReportIssue   *systray.MenuItem
	mDiscuss       *systray.MenuItem
	mQuit          *systray.MenuItem

	// State update signaling (thread-safe communication)
	stateUpdateSignal chan struct{} // Signal that state changed
	latestState       atomic.Bool   // Latest enabled state
	chanClosed        atomic.Bool
}

// NewComponent creates a new systray component.
func NewComponent(app AppInterface, logger *zap.Logger) *Component {
	ctx, cancel := context.WithCancel(context.Background())
	component := &Component{
		app:               app,
		logger:            logger,
		ctx:               ctx,
		cancel:            cancel,
		stateUpdateSignal: make(chan struct{}, 1),
	}

	// Register callback immediately for state changes
	app.OnEnabledStateChanged(func(enabled bool) {
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

	return component
}

// OnReady sets up the systray menu when the systray is ready.
func (c *Component) OnReady() {
	c.mVersionCopy = systray.AddMenuItem("Version: "+cli.Version, "Copy version to clipboard")

	c.mHelp = systray.AddMenuItem("Help", "Help menu")
	c.mDocsConfig = c.mHelp.AddSubMenuItem("Config Docs", "Open configuration documentation")
	c.mDocsCLI = c.mHelp.AddSubMenuItem("CLI Docs", "Open CLI documentation")
	c.mHelp.AddSeparator()
	c.mSourceCode = c.mHelp.AddSubMenuItem("Source Code", "View the source code")
	c.mReportIssue = c.mHelp.AddSubMenuItem("Report Issue", "Report an issue")
	c.mDiscuss = c.mHelp.AddSubMenuItem("Community Discussion", "Create a discussion")

	systray.AddSeparator()

	c.mModes = systray.AddMenuItem("Activate Modes", "Modes title")

	c.mHints = c.mModes.AddSubMenuItem("Hints", "Hint mode actions")
	if !c.app.HintsEnabled() {
		c.mHints.SetTitle("Hints: Disabled")
		c.mHints.Disable()
	}

	c.mGrid = c.mModes.AddSubMenuItem("Grid", "Grid mode actions")
	if !c.app.GridEnabled() {
		c.mGrid.SetTitle("Grid: Disabled")
		c.mGrid.Disable()
	}

	c.mQuadGrid = c.mModes.AddSubMenuItem("Quad Grid", "Quad Grid mode actions")
	if !c.app.QuadGridEnabled() {
		c.mQuadGrid.SetTitle("Quad Grid: Disabled")
		c.mQuadGrid.Disable()
	}

	systray.AddSeparator()

	c.mReloadConfig = systray.AddMenuItem("Reload Config", "Reload configuration from disk")

	c.mToggleDisable = systray.AddMenuItem("Pause Neru", "Pause Neru")
	c.mToggleEnable = systray.AddMenuItem("Resume Neru", "Resume Neru")
	c.mToggleEnable.Hide() // Initially hide the enable option

	c.mQuit = systray.AddMenuItem("Quit", "Exit the application")

	// Initialize all state-dependent UI elements
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
		c.mToggleDisable.Show()
		c.mToggleEnable.Hide()
	} else {
		systray.SetTitle("⏸️")
		systray.SetTooltip("Neru - Disabled")
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
		case <-c.mQuadGrid.ClickedCh:
			c.app.ActivateMode(domain.ModeQuadGrid)
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
				err := exec.CommandContext(c.ctx, "/usr/bin/open", "https://github.com/y3owk1n/neru/blob/main/docs/CONFIGURATION.md").
					Run()
				if err != nil {
					c.logger.Error("Failed to open configuration docs", zap.Error(err))
				}
			}()
		case <-c.mDocsCLI.ClickedCh:
			go func() {
				err := exec.CommandContext(c.ctx, "/usr/bin/open", "https://github.com/y3owk1n/neru/blob/main/docs/CLI.md").
					Run()
				if err != nil {
					c.logger.Error("Failed to open CLI docs", zap.Error(err))
				}
			}()
		case <-c.mReportIssue.ClickedCh:
			go func() {
				err := exec.CommandContext(c.ctx, "/usr/bin/open", "https://github.com/y3owk1n/neru/issues/new").
					Run()
				if err != nil {
					c.logger.Error("Failed to open issue report", zap.Error(err))
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
		case <-c.mQuit.ClickedCh:
			systray.Quit()

			return
		case <-c.stateUpdateSignal:
			c.updateMenuItems(c.latestState.Load())
		}
	}
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
