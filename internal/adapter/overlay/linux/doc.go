// Package linux draws the overlay on X11 and Wayland.
//
// Which surface is used is decided at runtime from the detected backend, so
// both compile into the same binary. The directory is linux-only, so its
// filenames carry no platform suffix.
package linux
