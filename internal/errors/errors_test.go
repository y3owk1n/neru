package errors

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(CodeInvalidInput, "test error")

	if err == nil {
		t.Fatal("New() returned nil")
	}

	if err.Code != CodeInvalidInput {
		t.Errorf("Expected code %v, got %v", CodeInvalidInput, err.Code)
	}

	if err.Message != "test error" {
		t.Errorf("Expected message 'test error', got '%s'", err.Message)
	}
}

func TestNewf(t *testing.T) {
	err := Newf(CodeInvalidConfig, "invalid value: %d", 42)

	if err == nil {
		t.Fatal("Newf() returned nil")
	}

	if err.Code != CodeInvalidConfig {
		t.Errorf("Expected code %v, got %v", CodeInvalidConfig, err.Code)
	}

	expected := "invalid value: 42"
	if err.Message != expected {
		t.Errorf("Expected message '%s', got '%s'", expected, err.Message)
	}
}

func TestError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *Error
		expected string
	}{
		{
			name: "error without cause",
			err: &Error{
				Code:    CodeElementNotFound,
				Message: "element not found",
			},
			expected: "[ELEMENT_NOT_FOUND] element not found",
		},
		{
			name: "error with cause",
			err: &Error{
				Code:    CodeAccessibilityFailed,
				Message: "failed to get element",
				Cause:   errors.New("underlying error"),
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
	cause := errors.New("underlying error")
	err := &Error{
		Code:    CodeIPCFailed,
		Message: "IPC failed",
		Cause:   cause,
	}

	unwrapped := err.Unwrap()
	//nolint:errorlint // Direct comparison needed for test
	if unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}

	// Test error without cause
	errNoCause := &Error{
		Code:    CodeIPCFailed,
		Message: "IPC failed",
	}

	if errNoCause.Unwrap() != nil {
		t.Error("Unwrap() should return nil for error without cause")
	}
}

func TestWrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := Wrap(cause, CodeActionFailed, "action failed")

	if err == nil {
		t.Fatal("Wrap() returned nil")
	}

	//nolint:errorlint // Direct comparison needed for test
	if err.Cause != cause {
		t.Errorf("Wrap() cause = %v, want %v", err.Cause, cause)
	}

	if err.Code != CodeActionFailed {
		t.Errorf("Wrap() code = %v, want %v", err.Code, CodeActionFailed)
	}

	// Test Wrap with nil error
	nilErr := Wrap(nil, CodeActionFailed, "action failed")
	if nilErr != nil {
		t.Error("Wrap() should return nil for nil error")
	}
}

func TestError_WithContext(t *testing.T) {
	err := New(CodeHintGenerationFailed, "hint generation failed")

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
	err1 := New(CodeTimeout, "timeout")
	err2 := New(CodeTimeout, "different message")
	err3 := New(CodeInternal, "internal error")

	if !err1.Is(err2) {
		t.Error("Is() should return true for errors with same code")
	}

	if err1.Is(err3) {
		t.Error("Is() should return false for errors with different codes")
	}

	// Test with standard error
	stdErr := errors.New("standard error")
	if err1.Is(stdErr) {
		t.Error("Is() should return false for non-Error types")
	}
}

func TestErrorCodes(t *testing.T) {
	codes := []Code{
		CodeAccessibilityDenied,
		CodeAccessibilityFailed,
		CodeElementNotFound,
		CodeInvalidConfig,
		CodeInvalidInput,
		CodeIPCFailed,
		CodeIPCServerNotRunning,
		CodeOverlayFailed,
		CodeHintGenerationFailed,
		CodeActionFailed,
		CodeContextCanceled,
		CodeTimeout,
		CodeInternal,
	}

	// Verify all codes are unique
	seen := make(map[Code]bool)
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

// Benchmark tests.
func BenchmarkNew(b *testing.B) {
	for b.Loop() {
		_ = New(CodeInvalidInput, "test error")
	}
}

func BenchmarkNewf(b *testing.B) {
	for b.Loop() {
		_ = Newf(CodeInvalidConfig, "invalid value: %d", 42)
	}
}

func BenchmarkError_WithContext(b *testing.B) {
	err := New(CodeActionFailed, "action failed")

	for b.Loop() {
		_ = err.WithContext("key", "value")
	}
}
