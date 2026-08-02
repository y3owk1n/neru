// Package ax is the contract every accessibility backend implements.
//
// It exists as its own package to break a cycle: the backends
// (atspi, macos, uia) must name these types to satisfy them, and the adapter
// that selects a backend must import the backends. Both sides depend on this
// leaf instead of on each other.
//
// Nothing here is platform-specific. A method that one platform cannot support
// is still declared, and the backend reports derrors.CodeNotSupported — see
// SupportsSupplementaryElements for the shape that takes when a whole family
// of behavior is absent.
package ax
