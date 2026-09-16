// Package bisect narrows a rectangle that starts as the screen: every cut
// keeps one half of it, or one quadrant, and the cursor sits at its center.
// Each press is a judgement the user already holds, which side of the
// center the target is on, and the region on screen shows exactly what the
// next press keeps.
//
// Region is the whole of the state, a rectangle and the stack of rectangles
// it was cut from. Backtrack restores the previous one.
package bisect
