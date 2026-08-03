// Package platform selects and constructs the per-OS implementations of the
// core ports — the only place that decides which adapter a build gets. The
// factories live in build-tagged siblings so each target imports only its own
// adapter package, keeping cgo code out of builds that cannot compile it.
//
// Linux adds a runtime axis: backend_linux.go detects the live compositor,
// since build constraints cannot tell KDE from GNOME. profile.go is the
// contributor-facing summary of backend families and CGO needs; `neru doctor`
// prints it.
package platform
