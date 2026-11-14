package motion

// MinCursorAnimationDuration is the floor for every smooth cursor animation
// in ms. The animators raise shorter animations to it. It is also the
// smallest accepted smooth_cursor.relative_movement_duration. Anything below
// would be silently rounded up rather than honored, so the validator rejects
// it.
const MinCursorAnimationDuration = 10

// SmoothCursor is how the cursor animates a move. Durations are in ms. The
// fields match the smooth_cursor table one for one. Config converts to it
// directly, so a field added to only one side fails to compile.
type SmoothCursor struct {
	MoveMouseEnabled         bool
	Steps                    int
	MaxDuration              int
	DurationPerPixel         float64
	RelativeMovementDuration int
}

// SmoothScroll is how a scroll animates. Durations are in ms. The fields
// match the smooth_scroll table one for one, as SmoothCursor's do.
type SmoothScroll struct {
	Enabled          bool
	Steps            int
	MaxDuration      int
	DurationPerPixel float64
}

// Animation holds the cursor and scroll settings a platform adapter reads on
// each move.
type Animation struct {
	Cursor SmoothCursor
	Scroll SmoothScroll
}

// AnimationSource returns the current Animation, and false when there is none
// yet. A platform adapter calls it on each move rather than keeping a copy,
// so a configuration reload takes effect on the next one.
type AnimationSource func() (Animation, bool)
