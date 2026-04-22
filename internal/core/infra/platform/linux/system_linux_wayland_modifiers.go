//go:build linux

package linux

import "sync"

// wlrootsModifierDispatcher deduplicates overlapping synthetic modifier usage.
// Sticky modifiers keep a virtual key held across navigation, while individual
// clicks may temporarily request the same modifier for a single pointer action.
// We only emit the physical key transition when the first holder appears or the
// last holder releases it.
type wlrootsModifierDispatcher struct {
	mu     sync.Mutex
	counts map[string]int
	send   func(modifier string, isDown bool) error
}

func newWlrootsModifierDispatcher(
	send func(modifier string, isDown bool) error,
) *wlrootsModifierDispatcher {
	return &wlrootsModifierDispatcher{
		counts: make(map[string]int),
		send:   send,
	}
}

func (d *wlrootsModifierDispatcher) event(modifier string, isDown bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	count := d.counts[modifier]

	if isDown {
		if count == 0 {
			if err := d.send(modifier, true); err != nil {
				return err
			}
		}

		d.counts[modifier] = count + 1

		return nil
	}

	if count == 0 {
		return d.send(modifier, false)
	}

	if count > 1 {
		d.counts[modifier] = count - 1

		return nil
	}

	if err := d.send(modifier, false); err != nil {
		return err
	}

	delete(d.counts, modifier)

	return nil
}
