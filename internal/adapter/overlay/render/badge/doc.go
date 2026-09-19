// Package badge holds the platform-neutral geometry and color math shared by
// the overlay badge renderers: indicator badges, hint labels and grid text
// backgrounds all size themselves the same way and read colors from the same
// hex notation. Text is measured by the platform's text layer where it can be
// (TextWidth, FontFit) and estimated from the font size where it cannot. The Linux (Cairo) and Windows (GDI)
// managers and the shared render styles all call into here; keeping the math
// in one place keeps badge sizing and color parsing identical across
// platforms.
package badge
