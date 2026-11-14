//go:build linux

package linux

import (
	"sync"

	"github.com/y3owk1n/neru/internal/domain/motion"
)

// animationSource is the process-global source of the smooth cursor and
// scroll settings, set once at startup (internal/app/runtime_config_linux.go).
// The smooth-cursor path here reads it, and so does the scroll path in
// accessibility/native/linux through CurrentAnimation. Nil (tests, or before
// wiring) means callers fall back to the direct, non-animated move.
var (
	animationSourceMu sync.RWMutex
	animationSource   motion.AnimationSource
)

// SetAnimationSource sets where the Linux cursor and scroll paths read the
// smooth cursor and scroll settings.
func SetAnimationSource(source motion.AnimationSource) {
	animationSourceMu.Lock()
	animationSource = source
	animationSourceMu.Unlock()
}

// CurrentAnimation returns the current settings, and false when no source is
// wired or the source has none.
func CurrentAnimation() (motion.Animation, bool) {
	animationSourceMu.RLock()

	source := animationSource

	animationSourceMu.RUnlock()

	if source == nil {
		return motion.Animation{}, false
	}

	return source()
}
