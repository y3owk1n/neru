package axobserver

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/ports"
)

const (
	defaultMessagingTimeout = 0.25
	defaultCloseTimeout     = 2 * time.Second
	commandBuffer           = 64
)

// Config configures a Manager.
type Config struct {
	// SelfPID is neru's own process id, excluded from observation so its overlay
	// redraws cannot trigger a refresh loop.
	SelfPID int
	// SelfBundleID is neru's own bundle id, also excluded.
	SelfBundleID string
	// WatchValueChanged enables kAXValueChanged for the front window (noisy).
	WatchValueChanged bool
	// MessagingTimeout bounds synchronous AX calls, in seconds; 0 uses a default.
	MessagingTimeout float64
	// OnChange is invoked (must be fast and non-blocking) when a non-stale
	// notification arrives from any observed process. Typically the refresh
	// coordinator's request function.
	OnChange func()
	// CloseTimeout bounds Manager.Close waiting for the actor to drain; 0 uses a
	// default.
	CloseTimeout time.Duration
}

// handle is one armed observer, owned exclusively by the actor goroutine.
type handle struct {
	pid      int
	bundleID string
	ptr      unsafe.Pointer // opaque native handle from platformArmObserver
	mask     uint32
}

type command interface{ isCommand() }

type reconcileCmd struct {
	targets []ports.ObservationTarget
	gen     uint64
}
type terminateCmd struct{ bundleID string }
type closeCmd struct{}
type syncCmd struct{ done chan struct{} }
type wakeCmd struct{}

func (reconcileCmd) isCommand() {}
func (terminateCmd) isCommand() {}
func (closeCmd) isCommand()     {}
func (syncCmd) isCommand()      {}
func (wakeCmd) isCommand()      {}

// Manager owns all AX observers, structured as an actor. Every mutation is a
// command processed on one goroutine, so callers on different threads never
// touch the observer map directly and no cross-process AX call runs under a
// caller's lock. Observers exist only while hints mode is active; when the last
// one is disarmed the dedicated run-loop thread is stopped and joined, so an
// idle neru has zero observer threads and zero background cost.
type Manager struct {
	logger *zap.Logger
	cfg    Config
	plat   platform

	cmds chan command
	done chan struct{}

	curEpoch   atomic.Uint64 // read by the callback sink, bumped by the actor
	liveCount  atomic.Int64  // armed observer count, for tests
	desiredGen atomic.Uint64 // bumped by DisarmAll under the caller's lock

	closeOnce sync.Once

	// owned exclusively by the actor goroutine:
	handles         map[int]*handle
	threadUp        bool
	lastDisarmedGen uint64
}

// NewManager creates and starts a Manager. The actor goroutine runs until Close.
func NewManager(logger *zap.Logger, cfg Config) *Manager {
	return newManager(logger, cfg, realPlatform{})
}

// newManager is the testable constructor: it accepts an injectable platform.
func newManager(logger *zap.Logger, cfg Config, plat platform) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	if cfg.MessagingTimeout <= 0 {
		cfg.MessagingTimeout = defaultMessagingTimeout
	}

	if cfg.CloseTimeout <= 0 {
		cfg.CloseTimeout = defaultCloseTimeout
	}

	m := &Manager{
		logger:  logger.Named("axobserver"),
		cfg:     cfg,
		plat:    plat,
		cmds:    make(chan command, commandBuffer),
		done:    make(chan struct{}),
		handles: make(map[int]*handle),
	}
	m.curEpoch.Store(1)

	m.plat.setSink(m.onNotification)

	go m.run()

	return m
}

// Reconcile sets the desired set of observed processes. The frontmost app pid
// may appear under several sources (window, menubar); their masks merge. This is
// the primary mutator, called after each hint scan. Non-blocking: if the actor's
// mailbox is momentarily full the command is dropped, and the next scan's
// Reconcile corrects it (reconcile is idempotent and self-healing).
func (m *Manager) Reconcile(targets []ports.ObservationTarget) {
	m.enqueue(reconcileCmd{targets: targets, gen: m.desiredGen.Load()})
}

// DisarmAll tears down every observer and stops the run-loop thread. Called when
// hints mode exits, under the mode-transition lock, so it must never block.
//
// It bumps an atomic generation. The actor tears down once per new generation
// before processing the next command, and any reconcile that was already queued
// from the exiting session carries an older generation and is skipped, so a
// backed-up mailbox can neither lose the teardown nor let a trailing reconcile
// re-arm observers after it. The wake is a best-effort nudge for an idle actor.
func (m *Manager) DisarmAll() {
	m.desiredGen.Add(1)

	select {
	case m.cmds <- wakeCmd{}:
	default:
	}
}

// HandleAppTerminated proactively disarms observers for a quit application
// (matched by bundle id), skipping the notification-unregister IPC to the dead
// process.
func (m *Manager) HandleAppTerminated(bundleID string) {
	if bundleID == "" {
		return
	}

	m.enqueue(terminateCmd{bundleID: bundleID})
}

// Close disarms everything, stops and joins the run-loop thread, and stops the
// actor. Idempotent. Bounded so a wedged app cannot hang shutdown forever.
func (m *Manager) Close() {
	m.closeOnce.Do(func() {
		select {
		case m.cmds <- closeCmd{}:
		case <-time.After(m.cfg.CloseTimeout):
			m.logger.Warn("axobserver close enqueue timed out")
			m.plat.setSink(nil)

			return
		}

		select {
		case <-m.done:
		case <-time.After(m.cfg.CloseTimeout):
			m.logger.Warn("axobserver close drain timed out")
		}

		m.plat.setSink(nil)
	})
}

// LiveObservers returns the number of currently armed observers (test hook).
func (m *Manager) LiveObservers() int {
	return int(m.liveCount.Load())
}

// ThreadRunning reports whether the observer run-loop thread is running (test hook).
func (m *Manager) ThreadRunning() bool {
	return m.plat.threadRunning()
}

// onNotification runs on the observer run-loop thread. It must be O(1) and
// non-blocking: it rejects stale-session callbacks by epoch, then signals the
// coordinator.
func (m *Manager) onNotification(pid int, epoch uint64, notif string) {
	if epoch != m.curEpoch.Load() {
		return
	}

	m.logger.Debug("ax notification", zap.Int("pid", pid), zap.String("notif", notif))

	if m.cfg.OnChange != nil {
		m.cfg.OnChange()
	}
}

// flush blocks until all previously enqueued commands have been processed. Used
// by tests to observe the actor deterministically. The send is blocking (not the
// drop-on-full path) so the barrier is never lost.
func (m *Manager) flush() {
	done := make(chan struct{})
	m.cmds <- syncCmd{done: done}
	<-done
}

func (m *Manager) enqueue(cmd command) {
	select {
	case m.cmds <- cmd:
	default:
		m.logger.Warn("axobserver command mailbox full; dropped a command")
	}
}

func (m *Manager) run() {
	for cmd := range m.cmds {
		// A DisarmAll bumps desiredGen. Tear down once per new generation, before
		// processing this command, so a reconcile that predates the disarm cannot
		// revive observers and a full mailbox cannot lose the teardown.
		gen := m.desiredGen.Load()
		if gen != m.lastDisarmedGen {
			m.doDisarmAll()
			m.lastDisarmedGen = gen
		}

		switch c := cmd.(type) {
		case reconcileCmd:
			// Skip a reconcile enqueued before a later DisarmAll; it is stale and
			// would re-arm observers for a session that has already exited.
			if c.gen == gen {
				m.doReconcile(c.targets)
			}
		case terminateCmd:
			m.doTerminate(c.bundleID)
		case wakeCmd:
			// no-op; exists only to wake the actor so the generation check runs
		case syncCmd:
			close(c.done)
		case closeCmd:
			m.doDisarmAll()
			m.stopThreadIfUp()
			close(m.done)

			return
		}
	}
}

func (m *Manager) doReconcile(targets []ports.ObservationTarget) {
	desiredMask := make(map[int]uint32)
	desiredBundle := make(map[int]string)

	for _, t := range targets {
		if t.PID <= 0 || t.PID == m.cfg.SelfPID {
			continue
		}

		if m.cfg.SelfBundleID != "" && t.BundleID == m.cfg.SelfBundleID {
			continue
		}

		desiredMask[t.PID] |= maskForSource(t.Source, m.cfg.WatchValueChanged)
		desiredBundle[t.PID] = t.BundleID
	}

	// Disarm observers that are gone, changed identity (recycled pid), or need a
	// different notification set.
	for pid, h := range m.handles {
		newMask, want := desiredMask[pid]
		if !want || h.bundleID != desiredBundle[pid] || h.mask != newMask {
			m.disarm(pid, true)
		}
	}

	// Arm newly desired processes.
	for pid, mask := range desiredMask {
		if _, ok := m.handles[pid]; ok {
			continue
		}

		m.arm(pid, desiredBundle[pid], mask)
	}

	if len(m.handles) == 0 {
		m.stopThreadIfUp()
	}
}

func (m *Manager) doDisarmAll() {
	// Bump the epoch first so any callback in flight during teardown is rejected.
	m.curEpoch.Add(1)

	for pid := range m.handles {
		m.disarm(pid, true)
	}

	m.stopThreadIfUp()
}

func (m *Manager) doTerminate(bundleID string) {
	for pid, h := range m.handles {
		if h.bundleID == bundleID {
			m.disarm(pid, false)
		}
	}

	if len(m.handles) == 0 {
		m.stopThreadIfUp()
	}
}

func (m *Manager) arm(pid int, bundleID string, mask uint32) {
	if mask == 0 {
		return
	}

	m.ensureThreadUp()

	ptr := m.plat.arm(pid, m.curEpoch.Load(), mask)
	if ptr == nil {
		m.logger.Debug("failed to arm observer",
			zap.Int("pid", pid), zap.String("bundle_id", bundleID))

		if len(m.handles) == 0 {
			m.stopThreadIfUp()
		}

		return
	}

	m.handles[pid] = &handle{pid: pid, bundleID: bundleID, ptr: ptr, mask: mask}
	m.liveCount.Store(int64(len(m.handles)))
}

func (m *Manager) disarm(pid int, live bool) {
	h := m.handles[pid]
	if h == nil {
		return
	}

	delete(m.handles, pid)
	m.plat.disarm(h.ptr, live)
	m.liveCount.Store(int64(len(m.handles)))
}

func (m *Manager) ensureThreadUp() {
	if m.threadUp {
		return
	}

	m.plat.setMessagingTimeout(m.cfg.MessagingTimeout)
	m.plat.startThread()
	m.threadUp = true
}

func (m *Manager) stopThreadIfUp() {
	if !m.threadUp {
		return
	}

	m.plat.stopThread()
	m.threadUp = false
}
