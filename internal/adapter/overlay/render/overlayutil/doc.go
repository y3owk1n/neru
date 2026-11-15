// Package overlayutil holds what the overlay render components share.
//
// The callback registry, the style and label caches, and the grid pointer
// draw are macOS only. They hand C heap pointers to the Objective-C bridge and
// free them again, so they call platform/darwin directly. The indicator memo
// is pure Go and builds everywhere.
package overlayutil
