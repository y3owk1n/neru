// Package platform selects and constructs the per-OS implementations of the
// core ports.
//
// It is the only place that decides which adapter a build gets. NewSystemPort
// and NewFontResolver live in build-tagged siblings (factory_darwin.go,
// factory_linux.go, factory_windows.go, factory_other.go) so that each target
// imports only its own adapter package — which is what keeps CGO-backed
// platform code out of builds that cannot compile it.
//
// Linux carries a second, runtime axis on top of build tags: build constraints
// cannot tell KDE from GNOME, since both are linux+Wayland at compile time.
// backend_linux.go detects the live compositor from the environment and the
// factory routes to it, returning CodeNotSupported for backends Neru has no
// path on (see factory_messages_linux.go).
//
// profile.go is the contributor-facing companion: per-subsystem backend family,
// primary-modifier expectations, and whether a backend needs CGO. It performs
// no I/O and is what `neru doctor` prints.
package platform
