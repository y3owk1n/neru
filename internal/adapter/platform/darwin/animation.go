//go:build darwin

package darwin

import "github.com/y3owk1n/neru/internal/domain/motion"

var animationSourceSlot cgoSlot[motion.AnimationSource]

// SetAnimationSource sets where the mouse and scroll helpers read the smooth
// cursor and scroll settings.
func SetAnimationSource(source motion.AnimationSource) {
	animationSourceSlot.Set(source)
}

// currentAnimation returns the current settings, and false when no source is
// wired or the source has none.
func currentAnimation() (motion.Animation, bool) {
	var (
		anim  motion.Animation
		found bool
	)

	animationSourceSlot.withValid(func(source motion.AnimationSource) {
		anim, found = source()
	})

	return anim, found
}
