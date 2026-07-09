package modes

import (
	"sync"
	"time"
)

// refreshCoordinatorConfig configures a refreshCoordinator.
type refreshCoordinatorConfig struct {
	// debounce is the trailing coalesce interval: a burst of change requests
	// collapses into one refresh once the burst pauses for this long.
	debounce time.Duration
	// idleRetry is how often to re-check the defer predicate while a refresh is
	// held back because the user is mid-selection.
	idleRetry time.Duration
	// maxDefer bounds how long a pending refresh can be held back before it fires
	// regardless: both while the user idles mid-type, and during a sustained
	// sub-debounce storm that would otherwise keep resetting the trailing timer
	// forever. It is the effective refresh floor during a continuous storm.
	maxDefer time.Duration
	// onRefresh performs the actual refresh. Called off the coordinator lock, on
	// a timer goroutine.
	onRefresh func()
	// shouldDefer reports whether a refresh must be held back right now (e.g. the
	// user is part-way through typing a hint label or a search query). May be nil.
	shouldDefer func() bool
	// now is injectable for tests; nil uses time.Now.
	now func() time.Time
}

// refreshCoordinator coalesces refresh requests from the push observer (and any
// other feed) into a single debounced call, and defers a refresh while the user
// is mid-selection so the hint set is never swapped out from under a partially
// typed label. It never blocks a caller: Request only arms a timer.
type refreshCoordinator struct {
	cfg refreshCoordinatorConfig

	mu           sync.Mutex
	timer        *time.Timer
	pendingSince time.Time
	stopped      bool
}

const (
	defaultRefreshDebounce  = 80 * time.Millisecond
	defaultRefreshIdleRetry = 120 * time.Millisecond
	defaultRefreshMaxDefer  = 3 * time.Second
)

func newRefreshCoordinator(cfg refreshCoordinatorConfig) *refreshCoordinator {
	if cfg.debounce <= 0 {
		cfg.debounce = defaultRefreshDebounce
	}

	if cfg.idleRetry <= 0 {
		cfg.idleRetry = defaultRefreshIdleRetry
	}

	if cfg.maxDefer <= 0 {
		cfg.maxDefer = defaultRefreshMaxDefer
	}

	if cfg.now == nil {
		cfg.now = time.Now
	}

	return &refreshCoordinator{cfg: cfg}
}

// Request schedules a coalesced refresh. Safe to call from any goroutine
// (including the observer run-loop thread); it only (re)arms a timer.
func (c *refreshCoordinator) Request() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}

	if c.pendingSince.IsZero() {
		c.pendingSince = c.cfg.now()
	}

	c.armLocked(c.cfg.debounce)
}

// Stop cancels any pending refresh. After Stop, Request is a no-op.
func (c *refreshCoordinator) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stopped = true
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}

	c.pendingSince = time.Time{}
}

func (c *refreshCoordinator) armLocked(d time.Duration) {
	// Cap the delay so the timer fires within maxDefer of the first pending
	// request, even if a continuous sub-debounce storm keeps resetting it. When
	// fire() runs at the cap it sees the request as overdue and refreshes.
	if !c.pendingSince.IsZero() {
		if remaining := c.cfg.maxDefer - c.cfg.now().Sub(c.pendingSince); remaining < d {
			d = remaining
		}

		if d < 0 {
			d = 0
		}
	}

	if c.timer == nil {
		c.timer = time.AfterFunc(d, c.fire)

		return
	}

	c.timer.Reset(d)
}

func (c *refreshCoordinator) fire() {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()

		return
	}

	pendingSince := c.pendingSince
	c.mu.Unlock()

	// Evaluate the defer predicate and the refresh callback OUTSIDE the
	// coordinator lock. onRefresh acquires the handler mutex; keeping c.mu out of
	// that path avoids any lock-ordering coupling between the two mutexes.
	overdue := !pendingSince.IsZero() && c.cfg.now().Sub(pendingSince) >= c.cfg.maxDefer
	if !overdue && c.cfg.shouldDefer != nil && c.cfg.shouldDefer() {
		c.mu.Lock()
		if !c.stopped {
			c.armLocked(c.cfg.idleRetry)
		}
		c.mu.Unlock()

		return
	}

	c.mu.Lock()
	c.pendingSince = time.Time{}
	c.mu.Unlock()

	if c.cfg.onRefresh != nil {
		c.cfg.onRefresh()
	}
}
