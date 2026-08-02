// Package manager is the overlay window management contract.
//
// It is a leaf so the per-platform backends can satisfy it and the package
// that selects a backend can import them, without a cycle. The application
// layer keeps saying overlay.Mode and overlay.ManagerInterface — those are
// aliases in the parent package, because a split beneath the app is no reason
// to rename anything above it.
package manager
