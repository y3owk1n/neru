// Package manager is the overlay window management contract.
//
// It is a leaf package: the per-platform backends import it to satisfy it, and
// the package that selects a backend imports the backends. Both sides depend on
// this rather than on each other.
//
// The application layer names these types as overlay.Mode and
// overlay.ManagerInterface, which are aliases in the parent package, so nothing
// above the adapter has to know the contract lives here.
//
// Capabilities only some backends have — holding the keyboard, drawing the
// monitor-select overlay — are optional extensions reached by type assertion
// rather than methods every platform must declare.
package manager
