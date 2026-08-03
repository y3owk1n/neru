// Package sequence runs action sequences.
//
// A sequence is an ordered list of steps — an action, a shell command, a mode
// name, or a macro that expands to another sequence. Global hotkeys, per-mode
// hotkeys, held-key repeat, a mode's --on-exit and the "run" command all
// execute through Executor, so a sequence behaves the same wherever it is
// written.
//
// The package also holds the vocabulary that describes one: how deeply
// sequences may nest, how a failing step is treated, and what running one
// produced.
//
// Executor's dependencies are functions and one-method interfaces rather than
// components, so it drives modes and IPC without importing either.
package sequence
