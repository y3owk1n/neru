// Package compositorcli asks a Wayland compositor's own command-line client a
// question and decodes the JSON it answers with.
//
// A Wayland client cannot ask the compositor about another client's window, so
// the compositors that answer at all answer through a CLI such as `hyprctl`,
// `swaymsg` or `niri msg`. Three callers ask: the focused window's rectangle
// (SystemPort.FocusedWindowBounds), the focused window's origin, which the
// AT-SPI client offsets window-relative element coordinates by, and the
// pointer's position on the compositors that expose it. All of them spawn a
// process, read its stdout and face the same four ways of getting nothing back.
// That is one derivation with two implementations, and the copies had
// already drifted, one of them bounding the read of a killed CLI's stdout pipe
// and the other not, on the path that holds the keyboard grab. ADR 0007 has the
// reasoning.
//
// What the package exists to keep separate is a query that failed from a
// compositor that answered "nothing". Only the first is a failure. A desktop
// with no focused window, and a niri window whose tiling gives it no on-screen
// position, are answers, and callers must be able to tell them apart without
// warning a person about a working session.
//
// What is deliberately *not* shared is what each caller does with the answer.
// The wire structs and the arithmetic look alike per compositor and are not the
// same derivation. Sway's window rectangle is the focused node's `rect` while
// its hint origin is `rect` plus `window_rect`, which excludes decorations, and
// niri's rectangle takes the window's own size while its origin is checked
// against the AT-SPI frame extents before it may be used at all. Those are two
// questions with two right answers, so each keeps its own. ADR 0007 says to
// name a near-miss like this out loud rather than collapse it.
package compositorcli
