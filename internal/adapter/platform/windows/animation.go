//go:build windows

package windows

import (
	"sync"

	"github.com/y3owk1n/neru/internal/domain/motion"
)

// animationSource is the process-global source of the smooth cursor and
// scroll settings for the Windows system adapter, mirroring the darwin and
// linux slots. It is set once at daemon startup (see
// internal/app/runtime_config_windows.go). A nil source (tests, or before
// wiring) means callers fall back to the direct, non-animated move.
var (
	animationSourceMu sync.RWMutex
	animationSource   motion.AnimationSource
)

// SetAnimationSource sets where the Windows cursor and scroll paths read the
// smooth cursor and scroll settings.
func SetAnimationSource(source motion.AnimationSource) {
	animationSourceMu.Lock()
	animationSource = source
	animationSourceMu.Unlock()
}

// currentAnimation returns the current settings, and false when no source is
// wired or the source has none.
func currentAnimation() (motion.Animation, bool) {
	animationSourceMu.RLock()

	source := animationSource

	animationSourceMu.RUnlock()

	if source == nil {
		return motion.Animation{}, false
	}

	return source()
}
