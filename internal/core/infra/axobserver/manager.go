package axobserver

import (
	"sync"

	"go.uber.org/zap"
)

// Target names a pid to observe and the structural notifications to watch for.
type Target struct {
	PID  int
	Mask Mask
}

// Manager keeps the set of armed observers reconciled with the desired targets.
//
// It is single-owner: Reconcile, DisarmAll, and Close must be called from one
// goroutine, or otherwise serialized by the caller. The change callback passed
// to New is invoked from the platform's callback thread; it must be cheap and
// must not call back into the Manager (that would deadlock against a concurrent
// teardown).
type Manager struct {
	platform Platform
	logger   *zap.Logger

	mu     sync.Mutex
	armed  map[int]Mask
	closed bool
}

// New creates a Manager for the current platform. onChange receives the pid of
// any application whose UI structurally changes while its observer is armed.
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
		armed:    make(map[int]Mask),
	}

	platform.SetSink(func(pid int, notif string) {
		logger.Debug("ax notification", zap.Int("pid", pid), zap.String("notif", notif))

		if onChange != nil {
			onChange(pid)
		}
	})

	return manager
}

// Reconcile arms observers for pids in targets that are not yet armed, disarms
// pids that are no longer wanted, and re-arms a pid whose mask changed. Targets
// with a non-positive pid or an empty mask are ignored.
func (m *Manager) Reconcile(targets []Target) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return
	}

	desired := make(map[int]Mask, len(targets))
	for _, target := range targets {
		if target.PID <= 0 || target.Mask == 0 {
			continue
		}

		desired[target.PID] = target.Mask
	}

	// Arm newly wanted pids, and re-arm any whose mask changed, before disarming
	// gone ones. Arming first means a full swap of targets never drops the live
	// observer count to zero, which would needlessly stop and restart the
	// run-loop thread.
	for pid, mask := range desired {
		if current, ok := m.armed[pid]; ok && current == mask {
			continue
		}

		if m.platform.Arm(pid, mask) {
			m.armed[pid] = mask

			m.logger.Debug("observer armed", zap.Int("pid", pid), zap.Uint32("mask", uint32(mask)))
		} else {
			// A failed arm removed any prior observer for this pid, so drop the
			// entry; a later reconcile retries from a clean state.
			delete(m.armed, pid)

			m.logger.Debug("observer arm failed", zap.Int("pid", pid))
		}
	}

	for pid := range m.armed {
		if _, ok := desired[pid]; !ok {
			m.platform.Disarm(pid)
			delete(m.armed, pid)

			m.logger.Debug("observer disarmed", zap.Int("pid", pid))
		}
	}
}

// DisarmAll removes every armed observer. It is safe to call when nothing is
// armed.
func (m *Manager) DisarmAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.armed) == 0 {
		return
	}

	m.platform.DisarmAll()
	m.armed = make(map[int]Mask)

	m.logger.Debug("observers disarmed")
}

// Close disarms everything and releases the platform. After Close, Reconcile is
// a no-op. It is idempotent.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return
	}

	m.closed = true
	m.platform.Close()
	m.armed = make(map[int]Mask)
}
