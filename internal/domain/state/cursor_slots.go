package state

import (
	"image"
	"maps"
	"sync"
)

// DefaultCursorSlot is the slot a save or restore uses when none is named.
//
// It is an ordinary name rather than a reserved empty string, so the slots are
// one flat namespace: the default has no privileges the others lack, and
// `--slot default` and no flag at all are the same request. That also keeps the
// status payload free of an empty JSON key.
const DefaultCursorSlot = "default"

// CursorSlots stores cursor positions under names.
//
// One unnamed slot was enough while only a hotkey binding could save a
// position, because a binding runs to completion before the next one starts.
// It stopped being enough once a sequence could invoke another one: an inner
// macro that saves the cursor would overwrite the outer sequence's save, and
// the outer restore would then move the cursor somewhere it never asked for —
// silently, since neither side had any way to notice the collision.
type CursorSlots struct {
	mu    sync.RWMutex
	slots map[string]image.Point
}

// NewCursorSlots creates an empty slot store.
func NewCursorSlots() *CursorSlots {
	return &CursorSlots{slots: make(map[string]image.Point)}
}

// Save stores pos under name, replacing whatever that slot held.
func (c *CursorSlots) Save(name string, pos image.Point) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.slots == nil {
		c.slots = make(map[string]image.Point)
	}

	c.slots[name] = pos
}

// Take removes the named slot and reports what it held.
//
// Reading and clearing are one operation because a restore consumes the slot:
// were they separate, two restores racing on the same slot could both see it
// occupied and both move the cursor.
func (c *CursorSlots) Take(name string) (image.Point, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	pos, ok := c.slots[name]
	if !ok {
		return image.Point{}, false
	}

	delete(c.slots, name)

	return pos, true
}

// Snapshot returns a copy of the occupied slots, for reporting. The result is
// never nil, so a caller reporting it always has an object to encode rather
// than sometimes a null.
func (c *CursorSlots) Snapshot() map[string]image.Point {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.slots == nil {
		return make(map[string]image.Point)
	}

	return maps.Clone(c.slots)
}
