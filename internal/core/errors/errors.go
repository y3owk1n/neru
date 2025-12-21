package errors

import (
	"errors"
	"fmt"
)

// Code represents a domain-specific error code.
type Code string

// Error codes for different failure scenarios.
const (
	// CodeAccessibilityDenied indicates accessibility permissions are not granted.
	CodeAccessibilityDenied Code = "ACCESSIBILITY_DENIED"

	// CodeAccessibilityFailed indicates an accessibility API call failed.
	CodeAccessibilityFailed Code = "ACCESSIBILITY_FAILED"

	// CodeElementNotFound indicates a UI element could not be found.
	CodeElementNotFound Code = "ELEMENT_NOT_FOUND"

	// CodeInvalidConfig indicates configuration validation failed.
	CodeInvalidConfig Code = "INVALID_CONFIG"

	// CodeInvalidInput indicates invalid input parameters.
	CodeInvalidInput Code = "INVALID_INPUT"

	// CodeIPCFailed indicates IPC communication failed.
	CodeIPCFailed Code = "IPC_FAILED"

	// CodeIPCAlreadyRunning indicates the IPC server is already running.
	CodeIPCAlreadyRunning Code = "IPC_ALREADY_RUNNING"

	// CodeIPCServerNotRunning indicates the IPC server is not running.
	CodeIPCServerNotRunning Code = "IPC_SERVER_NOT_RUNNING"

	// CodeOverlayFailed indicates overlay rendering failed.
	CodeOverlayFailed Code = "OVERLAY_FAILED"

	// CodeHintGenerationFailed indicates hint generation failed.
	CodeHintGenerationFailed Code = "HINT_GENERATION_FAILED"

	// CodeActionFailed indicates an action execution failed.
	CodeActionFailed Code = "ACTION_FAILED"

	// CodeContextCanceled indicates the operation was canceled.
	CodeContextCanceled Code = "CONTEXT_CANCELED"

	// CodeTimeout indicates the operation timed out.
	CodeTimeout Code = "TIMEOUT"

	// CodeInternal indicates an internal error occurred.
	CodeInternal Code = "INTERNAL"

	// CodeLoggingFailed indicates logger initialization or I/O failed.
	CodeLoggingFailed Code = "LOGGING_FAILED"

	// CodeConfigIOFailed indicates configuration file I/O failed.
	CodeConfigIOFailed Code = "CONFIG_IO_FAILED"

	// CodeVersionMismatch indicates an IPC protocol or version mismatch.
	CodeVersionMismatch Code = "VERSION_MISMATCH"

	// CodeHotkeyRegisterFailed indicates a hotkey registration error.
	CodeHotkeyRegisterFailed Code = "HOTKEY_REGISTER_FAILED"

	// CodeExecFailed indicates a shell execution error.
	CodeExecFailed Code = "EXEC_FAILED"

	// CodeSerializationFailed indicates JSON/TOML serialization/deserialization failed.
	CodeSerializationFailed Code = "SERIALIZATION_FAILED"

	// CodeBridgeFailed indicates a failure in native bridge interactions.
	CodeBridgeFailed Code = "BRIDGE_FAILED"

	// CodeSecureInputEnabled indicates secure input mode is active on macOS.
	CodeSecureInputEnabled Code = "SECURE_INPUT_ENABLED"
)

// Error represents a domain error with code, message, and optional cause.
type Error struct {
	code    Code
	message string
	cause   error
	context map[string]any
}

// New creates a new domain error with the given code and message.
func New(code Code, message string) *Error {
	return &Error{
		code:    code,
		message: message,
	}
}

// Newf creates a new domain error with formatted message.
func Newf(code Code, format string, args ...any) *Error {
	return &Error{
		code:    code,
		message: fmt.Sprintf(format, args...),
	}
}

// Code returns the error code.
func (e *Error) Code() Code {
	return e.code
}

// Message returns the error message.
func (e *Error) Message() string {
	return e.message
}

// Cause returns the underlying cause error.
func (e *Error) Cause() error {
	return e.cause
}

// Context returns the error context.
func (e *Error) Context() map[string]any {
	return e.context
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.code, e.message, e.cause)
	}

	return fmt.Sprintf("[%s] %s", e.code, e.message)
}

// Unwrap returns the underlying cause error.
func (e *Error) Unwrap() error {
	return e.cause
}

// Is implements error matching for errors.Is.
func (e *Error) Is(target error) bool {
	targetError, ok := target.(*Error)
	if !ok {
		return false
	}

	return e.code == targetError.code
}

// WithContext adds context information to the error.
func (e *Error) WithContext(key string, value any) *Error {
	if e.context == nil {
		e.context = make(map[string]any)
	}

	e.context[key] = value

	return e
}

// Wrap wraps an existing error with a domain error code and message.
func Wrap(err error, code Code, message string) *Error {
	if err == nil {
		return nil
	}

	return &Error{
		code:    code,
		message: message,
		cause:   err,
	}
}

// Wrapf wraps an existing error with formatted message.
func Wrapf(err error, code Code, format string, args ...any) *Error {
	if err == nil {
		return nil
	}

	return &Error{
		code:    code,
		message: fmt.Sprintf(format, args...),
		cause:   err,
	}
}

// IsCode checks if an error has the specified error code.
func IsCode(err error, code Code) bool {
	var domainErr *Error
	if errors.As(err, &domainErr) {
		return domainErr.code == code
	}

	return false
}

// GetCode extracts the error code from an error, or returns CodeInternal if not a domain error.
func GetCode(err error) Code {
	var domainErr *Error
	if errors.As(err, &domainErr) {
		return domainErr.code
	}

	return CodeInternal
}

// Is is a helper function that checks if an error is of a specific type.
func Is(err error, target error) bool {
	return errors.Is(err, target)
}

// IsAccessibilityError checks if an error is accessibility-related.
func IsAccessibilityError(err error) bool {
	return IsCode(err, CodeAccessibilityDenied) || IsCode(err, CodeAccessibilityFailed)
}

// IsUserError checks if an error is due to user input/configuration.
func IsUserError(err error) bool {
	return IsCode(err, CodeInvalidConfig) || IsCode(err, CodeInvalidInput)
}

// IsTransient checks if an error is potentially transient (retryable).
func IsTransient(err error) bool {
	return IsCode(err, CodeTimeout) || IsCode(err, CodeIPCFailed)
}
