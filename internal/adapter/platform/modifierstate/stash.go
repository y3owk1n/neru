package modifierstate

import (
	"slices"
	"sync"
)

// Stash carries a Plan from the call that took it to the call that undoes it.
//
// A press and its release are two separate actions with a drag in between, so
// the hold a press takes cannot be a scoped one: the keys it pressed stay
// pressed for as long as the button is down, and the keys it suppressed stay
// suppressed. Only the release knows the drag is over, and it may reach the
// display over a different connection than the press did — which is safe,
// because the identifiers in a Plan name keys on the display server rather than
// anything the connection owns.
//
// Keys are whatever identifies the held button to the backend that stashed it;
// the stash only ever passes them back. The zero value is ready to use.
type Stash struct {
	mu    sync.Mutex
	plans map[uint32]Plan
}

// Put records the plan a press took, so the matching release can undo it.
//
// A second press of a button already down merges rather than replaces. Dropping
// the earlier plan would strand both of its halves: nothing would release the
// key it pressed, and nothing would press back the key it suppressed, which the
// later press cannot see held precisely because the earlier one released it.
//
// Where the two plans name the same key on opposite sides, the later one is
// dropped: the earlier plan already records what that key was doing before Neru
// touched it, and that is the state the release has to land on.
func (s *Stash) Put(button uint32, plan Plan) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.plans == nil {
		s.plans = make(map[uint32]Plan, 1)
	}

	s.plans[button] = s.plans[button].mergedWith(plan)
}

// Take reports the plan the matching press stashed and forgets it, so no
// release undoes the same press twice.
//
// A release with no press behind it — a bare release action, or one that
// outlived the press's own bookkeeping — reports false rather than an empty
// plan, because "nothing to undo" and "undo nothing" are different instructions
// to the caller: the first leaves it free to present its own modifiers.
func (s *Stash) Take(button uint32) (Plan, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	plan, held := s.plans[button]
	if !held {
		return Plan{}, false
	}

	delete(s.plans, button)

	return plan, true
}

// mergedWith folds a later plan into this one, keeping every edit that still
// needs undoing and dropping the ones the two plans cancel between them.
func (p Plan) mergedWith(next Plan) Plan {
	return Plan{
		Suppress: appendUnclaimed(p.Suppress, next.Suppress, p.Press),
		Press:    appendUnclaimed(p.Press, next.Press, p.Suppress),
	}
}

// appendUnclaimed adds the edits of next to edits, skipping any naming a key
// that edits already carries or that the opposite half of the earlier plan
// already answers for.
func appendUnclaimed(edits, next, opposite []Edit) []Edit {
	merged := edits

	for _, edit := range next {
		if namesKey(merged, edit.Keycode) || namesKey(opposite, edit.Keycode) {
			continue
		}

		merged = append(merged, edit)
	}

	return merged
}

// namesKey reports whether any edit injects keycode. Edits are compared by the
// key they name rather than whole, because two plans can reach the same key
// under different modifier names — an X11 layout that resolves Meta onto the
// Super key gives one keycode two of them.
func namesKey(edits []Edit, keycode uint32) bool {
	return slices.ContainsFunc(edits, func(edit Edit) bool { return edit.Keycode == keycode })
}
