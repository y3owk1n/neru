package app

import (
	"context"
	"sync"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/metrics"
	"github.com/y3owk1n/neru/internal/core/ports"
	"github.com/y3owk1n/neru/internal/ui"
	"go.uber.org/zap"
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

	appState    *state.AppState
	cursorState *state.CursorState

	// Core services
	overlayManager OverlayManager
	hotkeyManager  HotkeyService
	eventTap       ports.EventTapPort
	ipcServer      ports.IPCPort
	appWatcher     Watcher
	metrics        metrics.Collector

	modes *modes.Handler

	// Control channels
	stopChan chan struct{}
	stopOnce sync.Once

	// New Architecture Services
	hintService   *services.HintService
	gridService   *services.GridService
	actionService *services.ActionService
	scrollService *services.ScrollService
	configService *config.Service

	// Feature components
	hintsComponent   *components.HintsComponent
	gridComponent    *components.GridComponent
	scrollComponent  *components.ScrollComponent
	systrayComponent SystrayComponent

	// Lifecycle management
	gcCancel         context.CancelFunc
	gcAggressiveMode bool

	// Renderer
	renderer *ui.OverlayRenderer

	// IPC Controller
	ipcController *IPCController
}
