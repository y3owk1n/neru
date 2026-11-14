// Package heldmotion drives the held-key glide. The direction keys currently
// held, from either key path, feed one fixed-rate loop that moves the cursor.
package heldmotion

import (
	"context"
	"image"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/motion"
	"github.com/y3owk1n/neru/internal/ports"
)

// TickInterval is the motion loop's period.
const TickInterval = 10 * time.Millisecond

// tapWindowScale sets a press's tap window in units of one interval plus the
// easing. A diagonal key reaches its step after about 1.41 intervals plus the
// easing, and twice that leaves headroom without reaching a long hold.
const tapWindowScale = 2

// MaxTickDelta caps the time a late tick may integrate, so a stalled
// scheduler produces a stutter rather than a jump.
const MaxTickDelta = 4 * TickInterval

type heldKey struct {
	group, key string
}

// heldEntry is what one held key contributes: where it points and how far its
// binding steps, which is what sets the glide's speed.
type heldEntry struct {
	dir  motion.Direction
	step float64
	// traveled is how far the glide has moved along dir since the press, so a
	// release knows what is left of one step.
	traveled float64
	// tapUntil ends the press's tap window. Past it a free glide has moved
	// the key a full step, even on a diagonal where each key is credited 0.71
	// of the travel, so a key still short of one was held against an edge or
	// canceled by its opposite, and is owed nothing.
	tapUntil time.Time
}

// tap is travel still owed to a key released before the glide moved it a
// full step. A tap on a stepped binding always moved one step.
type tap struct {
	dir      motion.Direction
	distance float64
}

// Controller owns the held-key set and the loop that consumes it. Its mutex
// is a leaf. Nothing under it calls out, so callers may hold their own locks.
type Controller struct {
	ctx    context.Context //nolint:containedctx // bounds the loop to the daemon's lifetime
	system ports.SystemPort
	ramp   func() motion.Ramp
	logger *zap.Logger

	mu      sync.Mutex
	held    map[heldKey]heldEntry
	taps    []tap
	current motion.Ramp
	running bool
	loopID  uint64
}

// New wires a controller. ramp is read on every press, and the loop reads
// the latest value on every tick, so a reload takes effect at the next key
// down even mid-glide. It runs inside Press under whatever lock the caller
// holds, so it must not block or reach the mode handler. A nil logger is
// replaced with a no-op.
func New(
	ctx context.Context,
	system ports.SystemPort,
	ramp func() motion.Ramp,
	logger *zap.Logger,
) *Controller {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Controller{
		ctx:    ctx,
		system: system,
		ramp:   ramp,
		logger: logger.Named("heldmotion"),
		held:   map[heldKey]heldEntry{},
	}
}

// Group namespaces keys for one key path, so a path releases only what it
// pressed while both still steer the same cursor. Every method is nil-safe,
// so a caller wired without a controller can call through unconditionally.
type Group struct {
	c    *Controller
	name string
}

// Group returns the namespace for one key path.
func (c *Controller) Group(name string) *Group {
	if c == nil {
		return nil
	}

	return &Group{c: c, name: name}
}

// Press adds key to the held set, pointing in dir with a binding that steps
// stepPx per interval, and starts the loop when idle.
func (g *Group) Press(key string, dir motion.Direction, stepPx float64) {
	if g == nil {
		return
	}

	g.c.press(heldKey{group: g.name, key: key}, heldEntry{dir: dir, step: stepPx})
}

// Release drops key from the held set, reporting whether it was held.
func (g *Group) Release(key string) bool {
	if g == nil {
		return false
	}

	return g.c.release(heldKey{group: g.name, key: key})
}

// IsHeld reports whether key is in this group's held set.
func (g *Group) IsHeld(key string) bool {
	if g == nil {
		return false
	}

	g.c.mu.Lock()
	defer g.c.mu.Unlock()

	_, ok := g.c.held[heldKey{group: g.name, key: key}]

	return ok
}

// ReleaseAll drops every key this group holds.
func (g *Group) ReleaseAll() {
	if g == nil {
		return
	}

	g.c.mu.Lock()
	defer g.c.mu.Unlock()

	for key := range g.c.held {
		if key.group == g.name {
			delete(g.c.held, key)
		}
	}
}

func (c *Controller) press(key heldKey, entry heldEntry) {
	ramp := c.ramp()
	entry.tapUntil = time.Now().Add(tapWindowScale * (ramp.Interval + motion.Smoothing))

	c.mu.Lock()
	defer c.mu.Unlock()

	c.current = ramp
	c.held[key] = entry

	if c.running {
		return
	}

	c.running = true
	c.loopID++

	go c.run(c.loopID)
}

// release drops the key and owes the loop what is left of one step, since a
// tap on a stepped binding always moved one. The glide eases in, so a tap may
// have moved less than a step or not ticked at all.
func (c *Controller) release(key heldKey) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.held[key]
	delete(c.held, key)

	if !ok || entry.traveled >= entry.step || time.Now().After(entry.tapUntil) {
		return ok
	}

	c.taps = append(c.taps, tap{dir: entry.dir, distance: entry.step - entry.traveled})

	return true
}

// advance runs one tick on integrator and returns the position to post. It
// pays the owed taps, steps the held set's glide, and credits that travel to
// the keys it moved, all under one lock hold, so a release cannot land between
// the move and its credit and be paid for travel it already made. When
// nothing is held and nothing is owed it marks the loop finished under the
// same hold, so a press racing the last release either joins this loop or
// starts the next one, never neither. The integrator is pure arithmetic, so
// the hold stays short.
func (c *Controller) advance(
	integrator *motion.Integrator,
	elapsed time.Duration,
) (image.Point, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.held) == 0 && len(c.taps) == 0 {
		c.running = false

		return image.Point{}, false
	}

	for _, owed := range c.taps {
		integrator.Nudge(owed.dir, owed.distance)
	}

	c.taps = nil

	var step float64

	dirs := make([]motion.Direction, 0, len(c.held))
	for _, entry := range c.held {
		dirs = append(dirs, entry.dir)
		step = max(step, entry.step)
	}

	pos := integrator.Step(c.current.ParamsFor(step), motion.Combine(dirs...), elapsed)

	// Each key is credited along its own heading, so a key steering one axis
	// of a diagonal counts only its share.
	movedX, movedY := integrator.Moved()
	for key, entry := range c.held {
		entry.traveled += max(0, movedX*entry.dir.X+movedY*entry.dir.Y)
		c.held[key] = entry
	}

	return pos, true
}

// finish marks loop id as no longer running, unless a newer loop has since
// taken over.
func (c *Controller) finish(id uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.loopID == id {
		c.running = false
	}
}

func (c *Controller) run(id uint64) {
	defer c.finish(id)

	ctx := c.ctx

	// A glide still in flight would fight the loop for the cursor.
	if settler, ok := c.system.(ports.CursorSettler); ok {
		err := settler.SettleCursor(ctx)
		if err != nil {
			c.logger.Debug("Failed to settle cursor before motion", zap.Error(err))
		}
	}

	start, err := c.system.CursorPosition(ctx)
	if err != nil {
		c.logger.Warn("Held-key glide cannot read the cursor position", zap.Error(err))

		return
	}

	integrator := motion.NewIntegrator(start, c.screenUnion(ctx))

	ticker := time.NewTicker(TickInterval)
	defer ticker.Stop()

	last := time.Now()
	posted := start
	postFailed := false

	for {
		var now time.Time

		select {
		case <-ctx.Done():
			return
		case now = <-ticker.C:
		}

		pos, ok := c.advance(integrator, min(now.Sub(last), MaxTickDelta))
		if !ok {
			return
		}

		last = now

		if pos == posted {
			continue
		}

		err := c.post(ctx, pos)
		if err != nil {
			if !postFailed {
				c.logger.Warn("Held-key glide failed to move the cursor", zap.Error(err))
			}

			// A platform that cannot move the cursor at all will not start to
			// on the next tick. Ticking on would only spin until release.
			if derrors.IsNotSupported(err) {
				return
			}

			postFailed = true

			continue
		}

		posted = pos
	}
}

// screenUnion is the rectangle spanning every screen, so a held key can
// cross monitors. It falls back to the active screen, and to no clamp at all,
// when the platform cannot say.
func (c *Controller) screenUnion(ctx context.Context) image.Rectangle {
	var union image.Rectangle

	screens, err := c.system.Screens(ctx)
	if err == nil {
		for _, screen := range screens {
			union = union.Union(screen.Bounds)
		}
	}

	if union.Empty() {
		bounds, err := c.system.ScreenBounds(ctx)
		if err == nil {
			union = bounds
		}
	}

	return union
}

func (c *Controller) post(ctx context.Context, pos image.Point) error {
	if mover, ok := c.system.(ports.InstantCursorMover); ok {
		return mover.MoveCursorInstantly(ctx, pos)
	}

	return c.system.MoveCursorToPoint(ctx, pos, true)
}
