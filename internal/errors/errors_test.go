package errors_test

import (
	"errors"
	"testing"

	derrors "github.com/y3owk1n/neru/internal/errors"
)

func TestNew(t *testing.T) {
	err := derrors.New(derrors.CodeInvalidInput, "test error")
	if err == nil {
		t.Fatal("New() returned nil")
	}

	if err.Code != derrors.CodeInvalidInput {
		t.Errorf("Expected code %v, got %v", derrors.CodeInvalidInput, err.Code)
	}

	if err.Message != "test error" {
		t.Errorf("Expected message 'test error', got '%s'", err.Message)
	}
}

func TestNewf(t *testing.T) {
	err := derrors.Newf(derrors.CodeInvalidConfig, "invalid value: %d", 42)
	if err == nil {
		t.Fatal("Newf() returned nil")
	}

	if err.Code != derrors.CodeInvalidConfig {
		t.Errorf("Expected code %v, got %v", derrors.CodeInvalidConfig, err.Code)
	}

	expected := "invalid value: 42"
	if err.Message != expected {
		t.Errorf("Expected message '%s', got '%s'", expected, err.Message)
	}
}

func TestError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *derrors.Error
		expected string
	}{
		{
			name: "error without cause",
			err: &derrors.Error{
				Code:    derrors.CodeElementNotFound,
				Message: "element not found",
			},
			expected: "[ELEMENT_NOT_FOUND] element not found",
		},
		{
			name: "error with cause",
			err: &derrors.Error{
				Code:    derrors.CodeAccessibilityFailed,
				Message: "failed to get element",
				Cause: errors.New( //nolint:err113 // dynamic errors needed for testing
					"underlying error",
				),
			},
			expected: "[ACCESSIBILITY_FAILED] failed to get element: underlying error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error") //nolint:err113 // dynamic errors needed for testing
	err := &derrors.Error{
		Code:    derrors.CodeIPCFailed,
		Message: "IPC failed",
		Cause:   cause,
	}

	unwrapped := err.Unwrap()
	if unwrapped != cause { //nolint:err113,errorlint // dynamic errors needed for testing
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}

	// Test error without cause
	errNoCause := &derrors.Error{
		Code:    derrors.CodeIPCFailed,
		Message: "IPC failed",
	}

	if errNoCause.Unwrap() != nil {
		t.Error("Unwrap() should return nil for error without cause")
	}
}

func TestWrap(t *testing.T) {
	cause := errors.New("underlying error") //nolint:err113 // dynamic errors needed for testing

	err := derrors.Wrap(cause, derrors.CodeActionFailed, "action failed")
	if err == nil {
		t.Fatal("Wrap() returned nil")
	}

	if err.Cause != cause { //nolint:err113,errorlint // dynamic errors needed for testing
		t.Errorf("Wrap() cause = %v, want %v", err.Cause, cause)
	}

	if err.Code != derrors.CodeActionFailed {
		t.Errorf("Wrap() code = %v, want %v", err.Code, derrors.CodeActionFailed)
	}

	// Test Wrap with nil error
	nilErr := derrors.Wrap(nil, derrors.CodeActionFailed, "action failed")
	if nilErr != nil {
		t.Error("Wrap() should return nil for nil error")
	}
}

func TestError_WithContext(t *testing.T) {
	err := derrors.New(derrors.CodeHintGenerationFailed, "hint generation failed")

	errWithContext := err.WithContext("element_id", "test-123")

	if errWithContext.Context == nil {
		t.Fatal("WithContext() context is nil")
	}

	if val, ok := errWithContext.Context["element_id"]; !ok || val != "test-123" {
		t.Errorf("WithContext() context['element_id'] = %v, want 'test-123'", val)
	}

	// Add another context value
	_ = errWithContext.WithContext("count", 5)

	if val, ok := errWithContext.Context["count"]; !ok || val != 5 {
		t.Errorf("WithContext() context['count'] = %v, want 5", val)
	}
}

func TestError_Is(t *testing.T) {
	err1 := derrors.New(derrors.CodeTimeout, "timeout")
	err2 := derrors.New(derrors.CodeTimeout, "different message")
	err3 := derrors.New(derrors.CodeInternal, "internal error")

	if !err1.Is(err2) {
		t.Error("Is() should return true for errors with same code")
	}

	if err1.Is(err3) {
		t.Error("Is() should return false for errors with different codes")
	}

	// Test with standard error
	stdErr := errors.New("standard error") //nolint:err113 // dynamic errors needed for testing
	if err1.Is(stdErr) {
		t.Error("Is() should return false for non-Error types")
	}
}

func TestErrorCodes(t *testing.T) {
	codes := []derrors.Code{
		derrors.CodeAccessibilityDenied,
		derrors.CodeAccessibilityFailed,
		derrors.CodeElementNotFound,
		derrors.CodeInvalidConfig,
		derrors.CodeInvalidInput,
		derrors.CodeIPCFailed,
		derrors.CodeIPCServerNotRunning,
		derrors.CodeOverlayFailed,
		derrors.CodeHintGenerationFailed,
		derrors.CodeActionFailed,
		derrors.CodeContextCanceled,
		derrors.CodeTimeout,
		derrors.CodeInternal,
	}

	// Verify all codes are unique
	seen := make(map[derrors.Code]bool)
	for _, code := range codes {
		if seen[code] {
			t.Errorf("Duplicate error code: %v", code)
		}

		seen[code] = true

		// Verify code is not empty
		if code == "" {
			t.Error("Error code should not be empty")
		}
	}
}
