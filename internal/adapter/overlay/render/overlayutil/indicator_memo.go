package overlayutil

import (
	"sync"
	"time"
)

// IndicatorReassertInterval is how long a shown indicator goes without a
// native show while nothing about it changes. A native show also runs the
// macOS check that repairs a window the WindowServer pinned to one Space
// (#1181). That check runs at most once a second, so this stays well under a
// second.
const IndicatorReassertInterval = 250 * time.Millisecond

// IndicatorMemo remembers what an indicator window last showed, so the
// indicator poll can skip a show, a draw or a hide that would change nothing.
// On macOS each of those is a round trip to the main thread, and the poll asks
// for them every tick even when the cursor is still.
//
// A window starts hidden. Hide, Clear and Invalidate all forget the last draw,
// and a draw that raced one of them is not remembered, so the next draw after
// any of them always goes through.
type IndicatorMemo[K comparable] struct {
	mu    sync.Mutex
	shown bool
	drawn bool
	key   K
	epoch uint64
	// shownAt is when the window last got a native show.
	shownAt time.Time
}

// Show reports whether the window needs a native show at now, and records it
// as shown. A hidden window always does. A shown one does again once
// IndicatorReassertInterval has passed.
func (m *IndicatorMemo[K]) Show(now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shown && now.Sub(m.shownAt) < IndicatorReassertInterval {
		return false
	}

	m.shown = true
	m.shownAt = now

	return true
}

// Hide reports whether the window needs hiding, and records that it is hidden.
func (m *IndicatorMemo[K]) Hide() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.shown {
		return false
	}

	m.shown = false
	m.forgetLocked()

	return true
}

// Clear reports whether the window's content needs clearing. Nobody sees a
// hidden window's content, and the next draw that sizes the window replaces it.
func (m *IndicatorMemo[K]) Clear() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.forgetLocked()

	return m.shown
}

// Invalidate forgets the last draw, for a change the key does not carry, such
// as a new configuration or theme.
func (m *IndicatorMemo[K]) Invalidate() {
	m.mu.Lock()
	m.forgetLocked()
	m.mu.Unlock()
}

// BeginDraw reports whether a draw of key would repeat what a shown window
// already has. When it would not, the caller draws and then hands the
// returned epoch to EndDraw.
func (m *IndicatorMemo[K]) BeginDraw(key K) (bool, uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.shown && m.drawn && m.key == key, m.epoch
}

// EndDraw records key as drawn, unless a hide, clear or invalidation landed
// since the BeginDraw that returned epoch.
func (m *IndicatorMemo[K]) EndDraw(key K, epoch uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if epoch != m.epoch {
		return
	}

	m.drawn = true
	m.key = key
}

func (m *IndicatorMemo[K]) forgetLocked() {
	m.drawn = false
	m.epoch++
}
