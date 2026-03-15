package app

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/app/services/modeindicator"
	"github.com/y3owk1n/neru/internal/app/services/stickyindicator"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/ports"
	"github.com/y3owk1n/neru/internal/ui"
)

// Mode is the current mode of the application.
type Mode = domain.Mode

// Mode constants from domain package.
const (
	ModeIdle   = domain.ModeIdle
	ModeHints  = domain.ModeHints
	ModeGrid   = domain.ModeGrid
	ModeScroll = domain.ModeScroll
)

// SystrayComponent defines the interface for systray functionality.
type SystrayComponent interface {
	OnReady()
	OnExit()
	Close()
}

// App represents the main application instance containing all state and dependencies.
type App struct {
	config     *config.Config
	ConfigPath string
	logger     *zap.Logger

	systemPort ports.SystemPort

	appState    *state.AppState
	cursorState *state.CursorState

	// Core services
	overlayManager OverlayManager
	hotkeyManager  HotkeyService
	eventTap       ports.EventTapPort
	ipcServer      ports.IPCPort
	appWatcher     Watcher

	modes *modes.Handler

	// Control channels
	stopChan chan struct{}
	stopOnce sync.Once

	// configMu serializes access to config-dependent component state between
	// concurrent writers (theme change observer, IPC config reload, systray reload).
	configMu sync.RWMutex

	// New Architecture Services
	hintService            *services.HintService
	gridService            *services.GridService
	actionService          *services.ActionService
	scrollService          *services.ScrollService
	modeIndicatorService   *modeindicator.Service
	stickyIndicatorService *stickyindicator.Service
	configService          *config.Service

	// Feature components
	hintsComponent           *components.HintsComponent
	gridComponent            *components.GridComponent
	scrollComponent          *components.ScrollComponent
	modeIndicatorComponent   *components.ModeIndicatorComponent
	stickyIndicatorComponent *components.StickyIndicatorComponent
	recursiveGridComponent   *components.RecursiveGridComponent
	systrayComponent         SystrayComponent

	// Lifecycle management
	gcCancel         context.CancelFunc
	gcAggressiveMode bool
	axCacheStop      func() // stops the accessibility InfoCache cleanup goroutine

	// State subscriptions
	screenShareSubscriptionID uint64

	// Renderer
	renderer *ui.OverlayRenderer

	// IPC Controller
	ipcController *IPCController
}
