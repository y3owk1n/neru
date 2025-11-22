// Package errors provides domain-specific error types and utilities.
//
// This package implements a structured error handling system with error codes,
// wrapping, and context information. It follows Go 1.13+ error handling patterns
// with errors.Is and errors.As support.
//
// # Usage
//
//	// Creating errors
//	err := errors.New(errors.CodeInvalidInput, "username cannot be empty")
//	err := errors.Newf(errors.CodeElementNotFound, "element %s not found", id)
//
//	// Wrapping errors
//	if err := doSomething(); err != nil {
//		return errors.Wrap(err, errors.CodeAccessibilityFailed, "failed to get elements")
//	}
//
//	// Adding context
//	err := errors.New(errors.CodeActionFailed, "click failed").
//		WithContext("element_id", elemID).
//		WithContext("action", "left_click")
//
//	// Checking error codes
//	if errors.IsCode(err, errors.CodeAccessibilityDenied) {
//		// Handle permission error
//	}
//
//	// Using errors.Is
//	if errors.Is(err, errors.New(errors.CodeTimeout, "")) {
//		// Handle timeout
//	}
//
// # Error Codes
//
// Error codes are organized by domain:
//   - Accessibility: CodeAccessibilityDenied, CodeAccessibilityFailed
//   - Configuration: CodeInvalidConfig
//   - IPC: CodeIPCFailed, CodeIPCServerNotRunning
//   - Actions: CodeActionFailed, CodeElementNotFound
//   - System: CodeTimeout, CodeContextCancelled, CodeInternal
//
// # Design Principles
//
//   - Structured: Errors have codes, messages, and optional context
//   - Wrappable: Errors can wrap underlying causes
//   - Matchable: Support for errors.Is and errors.As
//   - Informative: Context can be attached for debugging
package errors
