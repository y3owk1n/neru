//go:build linux && cgo

// Package wlr_protocol compiles the wayland-scanner generated protocol
// implementation files (.c) exactly once.
//
// The .c/.h files here are generated, not hand-written: `just
// generate-all-protocols` regenerates them from the XML in protocol/. This file
// is the only hand-maintained Go source, and it exists to give those generated
// sources a package to compile in — which is also why the package comment lives
// here rather than in a doc.go.  Other Go packages that need
// the wlr protocol types only #include the .h headers; the linker
// resolves the interface symbols from the objects built here.
//
// Any Go package whose C code references these symbols (e.g.
// zwlr_layer_shell_v1_interface) must blank-import this package:
//
//	import _ "github.com/y3owk1n/neru/internal/adapter/platform/linux/wlr_protocol"
//
// The pkg-config directive below supplies the wayland-client include path, so
// the .c files cgo compiles from this directory resolve #include
// "wayland-util.h". cgo compiles every .c file here automatically.
package wlr_protocol

/*
#cgo linux pkg-config: wayland-client
*/
import "C"
