// Package sequence runs action sequences: ordered steps that may be an
// action, a shell command, a mode name, or a macro expanding to another
// sequence. Hotkeys, held-key repeat, --on-exit and the "run" command all
// execute through Executor, so a sequence behaves the same wherever written.
// Executor's dependencies are functions and one-method interfaces, so it
// drives modes and IPC without importing either.
package sequence
