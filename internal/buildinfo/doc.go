// Package buildinfo holds the build-stamped identity of the binary.
//
// It exists so that code needing the version — the CLI's `--version`, the man
// page generator, the tray's "Version:" entry — does not have to import
// internal/cli to get it. The tray menu used to do exactly that, which pointed
// an application component at the outermost command layer.
//
// Version, GitCommit and BuildDate are set with -ldflags -X at build time; see
// LDFLAGS in the justfile and nix/package.nix. They keep working with their
// defaults in `go build` and `go test` runs that pass no ldflags.
package buildinfo
