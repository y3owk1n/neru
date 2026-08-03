//go:build linux && cgo

// Package wlr_protocol compiles the wayland-scanner generated .c files exactly
// once; `just generate-all-protocols` regenerates them from protocol/*.xml.
// This is the only hand-written Go source here — it gives the generated
// sources a package to compile in, which is also why the package comment is
// not in a doc.go. A Go package whose C code references these symbols must
// blank-import this one; the pkg-config directive below is what lets the .c
// files resolve #include "wayland-util.h".
package wlr_protocol

/*
#cgo linux pkg-config: wayland-client
*/
import "C"
