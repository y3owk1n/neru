package core

import (
	"context"
	"errors"
	"fmt"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// Error handling helpers for common error wrapping patterns

// WrapContextCanceled wraps a context.Canceled error with a standardized message.
func WrapContextCanceled(ctx context.Context, operation string) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled,
			operation+" canceled")
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled,
			operation+" timed out")
	}

	return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled,
		fmt.Sprintf("%s failed: %s", operation, ctx.Err().Error()))
}

// WrapActionFailed wraps an action-related error with a standardized message.
func WrapActionFailed(err error, action string) error {
	return derrors.Wrap(err, derrors.CodeActionFailed,
		fmt.Sprintf("failed to perform %s action", action))
}

// WrapOverlayFailed wraps an overlay-related error with a standardized message.
func WrapOverlayFailed(err error, operation string) error {
	return derrors.Wrap(err, derrors.CodeOverlayFailed,
		fmt.Sprintf("overlay %s failed", operation))
}

// WrapIPCFailed wraps an IPC-related error with a standardized message.
func WrapIPCFailed(err error, operation string) error {
	return derrors.Wrap(err, derrors.CodeIPCFailed,
		fmt.Sprintf("IPC %s failed", operation))
}

// WrapSerializationFailed wraps a serialization-related error with a standardized message.
func WrapSerializationFailed(err error, operation string) error {
	return derrors.Wrap(err, derrors.CodeSerializationFailed,
		fmt.Sprintf("serialization %s failed", operation))
}

// WrapAccessibilityFailed wraps an accessibility-related error with a standardized message.
func WrapAccessibilityFailed(err error, operation string) error {
	return derrors.Wrap(err, derrors.CodeAccessibilityFailed,
		fmt.Sprintf("accessibility %s failed", operation))
}

// WrapConfigFailed wraps a configuration-related error with a standardized message.
func WrapConfigFailed(err error, operation string) error {
	return derrors.Wrap(err, derrors.CodeInvalidConfig,
		fmt.Sprintf("configuration %s failed", operation))
}

// WrapIOFailed wraps an I/O-related error with a standardized message.
func WrapIOFailed(err error, operation string) error {
	return derrors.Wrap(err, derrors.CodeConfigIOFailed,
		fmt.Sprintf("I/O %s failed", operation))
}

// WrapInternalFailed wraps an internal error with a standardized message.
func WrapInternalFailed(err error, operation string) error {
	return derrors.Wrap(err, derrors.CodeInternal,
		fmt.Sprintf("internal %s failed", operation))
}
