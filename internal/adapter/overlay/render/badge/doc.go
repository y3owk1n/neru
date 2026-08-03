// Package badge holds the platform-neutral geometry and color math shared by
// the overlay badge renderers: indicator badges, hint labels and grid text
// backgrounds all size themselves from the same font-based estimates and read
// colors from the same hex notation. The Linux (Cairo) and Windows (GDI)
// managers and the shared render styles all call into here; keeping the math
// in one place keeps badge sizing and color parsing identical across
// platforms.
package badge
