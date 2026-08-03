// Package manager is the overlay window management contract, a leaf both the
// backends and the backend selector depend on. The application layer sees
// these types as overlay.Mode and overlay.ManagerInterface via aliases in the
// parent. Capabilities only some backends have are optional extensions
// reached by type assertion.
package manager
