package axobserver

import (
	"sync"

	"go.uber.org/zap"
)

// Manager keeps an AX observer armed on the one application it is told to
// watch.
//
// Watch, Unwatch, and Close must be called from one goroutine, or otherwise
// serialized by the caller. The change callback passed to New runs on the
// platform's callback thread; it must be cheap and must not call back into the
// Manager, which would deadlock against a concurrent teardown.
type Manager struct {
	platform Platform
	logger   *zap.Logger

	mu       sync.Mutex
	armedPID int
	closed   bool
}

// New creates a Manager for the current platform. The caller supplies onChange
// to receive callbacks: while an application's observer is armed, onChange
// fires with that application's pid each time its UI changes. It runs on the
// platform's callback thread. logger may be nil.
func New(onChange func(pid int), logger *zap.Logger) *Manager {
	return newWithPlatform(newPlatform(), onChange, logger)
}

func newWithPlatform(platform Platform, onChange func(pid int), logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	manager := &Manager{
		platform: platform,
		logger:   logger,
	}

	platform.SetChangeHandler(func(pid int, notif string) {
		logger.Debug("ax notification", zap.Int("pid", pid), zap.String("notif", notif))

		if onChange != nil {
			onChange(pid)
		}
	})

	return manager
}

// Watch arms an observer on pid, replacing whatever application was being
// watched. A non-positive pid, or the pid already watched, is a no-op. The new
// observer is armed before the previous one is disarmed, so switching
// applications never drops the live observer count to zero, which would stop
// and restart the platform's run-loop thread.
func (m *Manager) Watch(pid int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed || pid <= 0 || pid == m.armedPID {
		return
	}

	prev := m.armedPID

	if m.platform.Arm(pid) {
		m.armedPID = pid

		m.logger.Debug("observer armed", zap.Int("pid", pid))
	} else {
		// A failed arm leaves the slot empty, so a later Watch of the same pid
		// retries instead of treating it as already armed.
		m.armedPID = 0

		m.logger.Debug("observer arm failed", zap.Int("pid", pid))
	}

	if prev != 0 {
		m.platform.Disarm(prev)

		m.logger.Debug("observer disarmed", zap.Int("pid", prev))
	}
}

// Unwatch stops watching the current application. It is a no-op when nothing
// is armed.
func (m *Manager) Unwatch() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.armedPID == 0 {
		return
	}

	m.platform.DisarmAll()
	m.armedPID = 0

	m.logger.Debug("observers disarmed")
}

// Close disarms everything and releases the platform. After Close, Watch is a
// no-op. It is idempotent.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return
	}

	m.closed = true
	m.platform.Close()
	m.armedPID = 0
}
