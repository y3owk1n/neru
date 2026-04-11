//go:build linux && cgo

// Package wlrprotocol compiles the wayland-scanner generated protocol
// implementation files (.c) exactly once.  Other Go packages that need
// the wlr protocol types only #include the .h headers; the linker
// resolves the interface symbols from the objects built here.
//
// Any Go package whose C code references these symbols (e.g.
// zwlr_layer_shell_v1_interface) must blank-import this package:
//
//	import _ "github.com/y3owk1n/neru/internal/core/infra/platform/linux/wlr_protocol"
package wlr_protocol

// The #cgo directive supplies the wayland-client include path so the
// auto-compiled .c files can resolve #include "wayland-util.h".
// CGO automatically compiles every .c file in this directory.

/*
#cgo linux pkg-config: wayland-client
*/
import "C"
