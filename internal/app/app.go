package app

import (
	"context"
	"io"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/ipcctrl"
	"github.com/y3owk1n/neru/internal/app/keybinding"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/sequence"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
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

// SystrayComponent is the interface for systray functionality.
type SystrayComponent interface {
	OnReady()
	OnExit()
	Close()
}

// App is the main application instance containing all state and dependencies.
type App struct {
	ctx    context.Context //nolint:containedctx // Root context for all App operations
	cancel context.CancelFunc
	config *config.Config
	// writtenConfig is config before the loader settled its derived values,
	// handed to the config service so a later `neru config set` derives from
	// what the user wrote rather than from what was derived for them.
	writtenConfig *config.Config
	// configWarnings are what the launch-time load found wrong but loadable,
	// logged once the logger exists (WithConfigWarnings).
	configWarnings []string
	ConfigPath     string
	logger         *zap.Logger

	systemPort    ports.SystemPort
	accessibility ports.AccessibilityPort

	appState    *state.AppState
	cursorState *state.CursorState

	// Core services
	//
	// overlayManager is the backend the composition root built the port over.
	// Only the composition root names it — construction in phase 1, the port in
	// phase 3, its render components in phase 4 — and everything the running
	// app says to the overlay goes through overlayPort instead.
	overlayManager OverlayManager
	// overlayPort is the one contract the app has with the overlay: frames for
	// transitions, calls for updates, and the configuration and theme changes
	// the overlay resolves its own Styles from (ADR 0003).
	overlayPort   ports.OverlayPort
	hotkeyManager HotkeyService
	hotkeys       *keybinding.Binder
	eventTap      ports.EventTapPort
	textInput     ports.TextInputPort
	keyFeed       ports.KeyFeedPort
	ipcServer     ports.IPCPort
	appWatcher    Watcher

	modes *modes.Handler

	// sequenceExecutor runs action sequences. It is built during initialization
	// from the App's own components; see newSequenceExecutor.
	sequenceExecutor *sequence.Executor

	// axClient is stored so it can be closed during Cleanup.
	// On Linux this resets AT-SPI accessibility status and releases the
	// D-Bus connection; on other platforms it is a no-op.
	axClient io.Closer

	// Control channels
	stopChan    chan struct{}
	stopOnce    sync.Once
	cleanupOnce sync.Once

	// configMu serializes access to config-dependent component state between
	// concurrent writers (theme change observer, IPC config reload, systray reload).
	configMu sync.RWMutex

	// New Architecture Services
	hintService   *services.HintService
	gridService   *services.GridService
	actionService *services.ActionService
	scrollService *services.ScrollService
	// indicators owns one service per indicator: mode, sticky modifiers,
	// virtual pointer.
	indicators    indicatorServices
	configService *loader.Service

	// Feature components
	hintsComponent         *components.HintsComponent
	gridComponent          *components.GridComponent
	scrollComponent        *components.ScrollComponent
	recursiveGridComponent *components.RecursiveGridComponent
	systrayComponent       SystrayComponent

	// Lifecycle management
	gcCancel         context.CancelFunc
	gcAggressiveMode bool

	// Per-instance observer teardown and hooks, assigned by the platform
	// setup functions and consumed by the shared entry points in
	// observers.go. Closures keep platform types off this struct.
	//
	// observerMu guards the three: Run assigns them after the IPC server is
	// already accepting a reload, and the reload path reads postReloadVerify.
	observerMu        sync.Mutex
	themeObserverStop func()
	sleepObserverStop func()
	postReloadVerify  func()

	// State subscriptions
	screenShareSubscriptionID uint64

	ipcController *ipcctrl.Controller
}
