package manager

import (
	"sync"
	"time"
)

// vimSpinInterval is how often the notice spinner takes a step, so its twelve
// steps make a turn in 0.9 seconds, as on macOS.
const vimSpinInterval = 75 * time.Millisecond

// VimSpin turns the notice spinner of a vim frame drawn from LayoutVim. It
// calls redraw with the next step on a timer, holding lock, until stopped.
// macOS turns its own spinner with Core Animation.
//
// lock is the backend's render lock, the one its draws already hold, so a
// step never interleaves with a draw. The caller of Stop holds it too, which
// lets a stopped spin skip the step it was waiting to draw.
type VimSpin struct {
	stopped bool
	done    chan struct{}
}

// StartVimSpin starts a spin that redraws under lock.
func StartVimSpin(lock sync.Locker, redraw func(step int)) *VimSpin {
	spin := &VimSpin{done: make(chan struct{})}

	go func() {
		ticker := time.NewTicker(vimSpinInterval)
		defer ticker.Stop()

		for step := 1; ; step++ {
			select {
			case <-spin.done:
				return
			case <-ticker.C:
			}

			lock.Lock()

			if !spin.stopped {
				redraw(step)
			}

			lock.Unlock()
		}
	}()

	return spin
}

// Stop ends the spin. The caller holds the lock StartVimSpin was given. A
// nil spin is stopped already.
func (spin *VimSpin) Stop() {
	if spin == nil || spin.stopped {
		return
	}

	spin.stopped = true
	close(spin.done)
}
