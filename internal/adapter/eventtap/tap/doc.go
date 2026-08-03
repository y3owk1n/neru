// Package tap is the contract every event-tap backend implements.
//
// It exists as its own package for the same reason accessibility/ax does: the
// backends must name these types to satisfy them, and the adapter that selects
// a backend must import the backends. Both sides depend on this leaf instead of
// on each other.
package tap
