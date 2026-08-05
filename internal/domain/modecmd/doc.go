// Package modecmd is the grammar of a mode command.
//
// A mode command — "hints --action left_click" — names a navigation mode and
// the flags it is entered with. It reaches the daemon from three places: typed
// after "neru", written as a step in a hotkey binding, or sent by a script over
// the IPC socket. Each place used to read it for itself, and the three had
// drifted: one rule existed on the command line only, four messages differed by
// door, and a flag the named mode does not accept was stored and dropped in
// silence.
//
// This package owns the whole reading, so there is one of it. It holds the flag
// vocabulary, which modes accept which flag, parsing an argument list into an
// [Activation], validating an Activation, and rendering an Activation back into
// an argument list.
//
// Each flag is one entry in the descriptor table, carrying its own parse, its
// own render, the shape it is written in, and the one sentence that says what
// it does — which is what a command line offers it as. Nothing here switches
// over flag names: a
// switch has a default clause, and a default clause is what let a flag go
// unhandled without anyone noticing. A flag declared without saying how it
// parses, how it renders and what it is for does not compile.
//
// It cannot import internal/config, because the configuration validates the
// bindings it holds and so must import this. The values the flags accept live
// in internal/domain, beneath both.
package modecmd
