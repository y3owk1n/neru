// Package ax is the contract every accessibility backend implements.
//
// It is a leaf package: the backends import it to satisfy it, and the adapter
// that selects a backend imports the backends. Both sides depend on this rather
// than on each other, which is what keeps the graph acyclic.
//
// Nothing declared here is platform-specific. A method one platform cannot
// support is still part of the contract, and that backend reports
// derrors.CodeNotSupported — see SupportsSupplementaryElements for the shape
// that takes when a whole family of behavior is absent.
package ax
