// Package sequence holds the vocabulary of an action sequence: how deeply one
// may nest, how a failing step is treated, and what running one produced.
//
// The executor itself still lives in internal/app, because running a step means
// dispatching it through the app. What lives here is everything about a
// sequence that does not need the app — which is what lets the IPC controller
// describe a sequence run without importing the application package.
package sequence
