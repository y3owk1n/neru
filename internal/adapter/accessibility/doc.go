// Package accessibility implements ports.AccessibilityPort: finding clickable
// elements, identifying the focused application, and walking element trees.
//
// It holds the adapter and the filtering that shape a backend's raw results
// into what the hint services expect. The backends themselves are subpackages:
//
//   - ax      the contract they implement
//   - atspi   the Linux backend, walking the AT-SPI tree over D-Bus
//   - native  the client built on each OS's own API
//
// factory_linux.go and factory_other.go are the only files that know which
// backend exists.
package accessibility
