//go:build linux

package linux

import (
	"sync"
	"time"

	"go.uber.org/zap"

	eventtaplinux "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

const waylandStopTimeout = 3 * time.Second

// Manager handles the registration, unregistration, and dispatching of global hotkeys.
type Manager struct {
	callbacks map[ports.HotkeyID]ports.HotkeyCallback
	keys      map[ports.HotkeyID]string
	logger    *zap.Logger
	rawLogger *zap.Logger
	nextID    ports.HotkeyID
	backend   platform.LinuxBackend
	mu        sync.RWMutex

	// waylandHotkeys honors config keybindings on Wayland via passive evdev
	// reads, since compositors do not expose global hotkeys to clients.
	waylandHotkeys *eventtaplinux.GlobalHotkeyListener
	waylandStarted bool
	// waylandUnsupportedLogged latches the CodeNotSupported refusal so the
	// build-is-missing-evdev warning is said once. Registration retries Start
	// per hotkey, and the sleep/reload recovery loop retries registration up to
	// ten times, so an unlatched warning would repeat dozens of times for an
	// answer fixed at compile time.
	waylandUnsupportedLogged bool
}

// NewManager creates and creates a new hotkey manager instance.
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	mgr := &Manager{
		callbacks: make(map[ports.HotkeyID]ports.HotkeyCallback),
		keys:      make(map[ports.HotkeyID]string),
		logger:    logger.Named("hotkeys"),
		rawLogger: logger,
		nextID:    1,
		backend:   platformBackend(),
	}

	if mgr.backend.IsWayland() {
		mgr.waylandHotkeys = eventtaplinux.NewGlobalHotkeyListener(logger)
	}

	return mgr
}

// Register adds a new global hotkey (Linux stub).
func (m *Manager) Register(
	keyString string,
	callback ports.HotkeyCallback,
) (ports.HotkeyID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	hotkeyID := m.nextID
	m.nextID++
	m.callbacks[hotkeyID] = callback
	m.keys[hotkeyID] = keyString

	switch m.backend {
	case platform.BackendX11:
		err := m.registerX11Hotkey(hotkeyID, keyString)
		if err != nil {
			delete(m.callbacks, hotkeyID)
			delete(m.keys, hotkeyID)

			return 0, err
		}
	case platform.BackendWaylandWlroots, platform.BackendWaylandKDE,
		platform.BackendWaylandGNOME, platform.BackendWaylandOther:
		m.rebuildWaylandBindings()
		m.ensureWaylandStarted()
	case platform.BackendUnknown:
		m.logger.Debug(
			"Registering hotkey in Linux manager",
			zap.String("key", keyString),
			zap.String("backend", m.backend.String()),
		)
	default:
		m.logger.Debug(
			"Registering hotkey in Linux manager",
			zap.String("key", keyString),
			zap.String("backend", m.backend.String()),
		)
	}

	return hotkeyID, nil
}

// Unregister removes a previously registered hotkey by its ID (Linux stub).
func (m *Manager) Unregister(hotkeyID ports.HotkeyID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.backend == platform.BackendX11 {
		m.unregisterX11Hotkey(hotkeyID)
	}

	delete(m.callbacks, hotkeyID)
	delete(m.keys, hotkeyID)

	if m.backend.IsWayland() {
		m.rebuildWaylandBindings()

		if len(m.callbacks) == 0 {
			m.stopWayland()
		}
	}
}

// UnregisterAll removes all currently registered hotkeys (Linux stub).
func (m *Manager) UnregisterAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.backend == platform.BackendX11 {
		m.unregisterAllX11Hotkeys()
	}

	m.callbacks = make(map[ports.HotkeyID]ports.HotkeyCallback)
	m.keys = make(map[ports.HotkeyID]string)

	if m.backend.IsWayland() {
		m.stopWayland()
	}
}

// Ensure the Linux Manager keeps satisfying the optional health-reporting
// extension. The sleep/resume handler reaches it by type assertion, so a
// signature drift would silently stop re-registering hotkeys after an X11
// restart or compositor reload instead of failing to compile.
var _ ports.HotkeyHealthReporter = (*Manager)(nil)

// HealthCheck returns true when the global hotkey listener is healthy.
// On non-Wayland backends (X11) it always returns true because there is no
// passive evdev listener to monitor. Callers (the app health-check loop)
// reinitialize the listener if this returns false.
func (m *Manager) HealthCheck() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.backend.IsWayland() {
		return true
	}

	if m.waylandHotkeys == nil {
		return true
	}

	if len(m.callbacks) == 0 {
		return true
	}

	if !m.waylandStarted {
		return false
	}

	if !m.waylandHotkeys.IsRunning() {
		return false
	}

	return m.waylandHotkeys.DeviceCount() > 0
}

// rebuildWaylandBindings re-syncs the evdev listener with the current set of
// registered hotkeys. Callers must hold m.mu.
func (m *Manager) rebuildWaylandBindings() {
	if m.waylandHotkeys == nil {
		return
	}

	m.waylandHotkeys.ClearBindings()

	for id, key := range m.keys {
		m.waylandHotkeys.SetBinding(key, m.callbacks[id])
	}
}

// ensureWaylandStarted lazily starts the evdev listener on first registration.
// Callers must hold m.mu.
func (m *Manager) ensureWaylandStarted() {
	if m.waylandHotkeys == nil {
		return
	}

	if m.waylandStarted {
		if m.waylandHotkeys.IsRunning() {
			return
		}

		m.waylandStarted = false
	}

	err := m.waylandHotkeys.Start()
	if err != nil {
		m.logWaylandStartFailure(err)

		return
	}

	m.waylandStarted = true

	m.logger.Info("Wayland global hotkeys enabled via evdev; config keybindings are active")
}

// logWaylandStartFailure explains a failed Start in terms the user can act on.
// Callers must hold m.mu.
//
// The two failures need different advice. A CodeNotSupported refusal means this
// binary was built without cgo and carries no evdev reader at all, so telling
// the user to join the `input` group sends them after a fix that cannot work —
// and nothing about a compile-time answer changes on a later attempt, which is
// why it is said once. Everything else is a live listener that could not read
// `/dev/input`, which permissions can still fix, so that one keeps warning per
// attempt.
func (m *Manager) logWaylandStartFailure(err error) {
	if derrors.IsNotSupported(err) {
		if m.waylandUnsupportedLogged {
			return
		}

		m.waylandUnsupportedLogged = true

		m.logger.Warn(
			"Wayland global hotkeys unavailable: this build has no evdev support "+
				"(built without cgo). Bind `neru <mode>` in your compositor instead, "+
				"or use a cgo-enabled build",
			zap.Error(err),
		)

		return
	}

	m.logger.Warn(
		"Wayland global hotkeys unavailable; grant read access to /dev/input "+
			"(add your user to the `input` group) or bind `neru <mode>` in your compositor instead",
		zap.Error(err),
	)
}

func (m *Manager) stopWayland() {
	if m.waylandHotkeys == nil || !m.waylandStarted {
		return
	}

	if !m.waylandHotkeys.StopWithTimeout(waylandStopTimeout) {
		m.logger.Warn("Replacing stuck evdev hotkey listener with fresh instance")
		m.waylandHotkeys = eventtaplinux.NewGlobalHotkeyListener(m.rawLogger)
	}

	m.waylandStarted = false
}

// SetGlobalManager assigns the global manager instance (Linux stub).
func SetGlobalManager(_ *Manager) {}

func platformBackend() platform.LinuxBackend {
	return platform.DetectLinuxBackend()
}
