package errors_test

import (
	"errors"
	"testing"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

func TestNew(t *testing.T) {
	err := derrors.New(derrors.CodeInvalidInput, "test error")
	if err == nil {
		t.Fatal("New() returned nil")
	}

	if err.Code() != derrors.CodeInvalidInput {
		t.Errorf("Expected code %v, got %v", derrors.CodeInvalidInput, err.Code())
	}

	if err.Message() != "test error" {
		t.Errorf("Expected message 'test error', got '%s'", err.Message())
	}
}

func TestNewf(t *testing.T) {
	err := derrors.Newf(derrors.CodeInvalidConfig, "invalid value: %d", 42)
	if err == nil {
		t.Fatal("Newf() returned nil")
	}

	if err.Code() != derrors.CodeInvalidConfig {
		t.Errorf("Expected code %v, got %v", derrors.CodeInvalidConfig, err.Code())
	}

	expected := "invalid value: 42"
	if err.Message() != expected {
		t.Errorf("Expected message '%s', got '%s'", expected, err.Message())
	}
}

func TestError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *derrors.Error
		expected string
	}{
		{
			name:     "error without cause",
			err:      derrors.New(derrors.CodeElementNotFound, "element not found"),
			expected: "[ELEMENT_NOT_FOUND] element not found",
		},
		{
			name: "error with cause",
			err: derrors.Wrap(
				errors.New("underlying error"), //nolint:err113 // dynamic errors needed for testing
				derrors.CodeAccessibilityFailed,
				"failed to get element",
			),
			expected: "[ACCESSIBILITY_FAILED] failed to get element: underlying error",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := testCase.err.Error()
			if got != testCase.expected {
				t.Errorf("Error() = %q, want %q", got, testCase.expected)
			}
		})
	}
}

func TestError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error") //nolint:err113 // dynamic errors needed for testing
	err := derrors.Wrap(cause, derrors.CodeIPCFailed, "IPC failed")

	unwrapped := err.Unwrap()
	if unwrapped != cause { //nolint:err113,errorlint // dynamic errors needed for testing
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}

	// Test error without cause
	errNoCause := derrors.New(derrors.CodeIPCFailed, "IPC failed")

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

	if err.Cause() != cause { //nolint:err113,errorlint // dynamic errors needed for testing
		t.Errorf("Wrap() cause = %v, want %v", err.Cause(), cause)
	}

	if err.Code() != derrors.CodeActionFailed {
		t.Errorf("Wrap() code = %v, want %v", err.Code(), derrors.CodeActionFailed)
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

	if errWithContext.Context() == nil {
		t.Fatal("WithContext() context is nil")
	}

	if val, ok := errWithContext.Context()["element_id"]; !ok || val != "test-123" {
		t.Errorf("WithContext() context['element_id'] = %v, want 'test-123'", val)
	}

	// Add another context value
	_ = errWithContext.WithContext("count", 5)

	if val, ok := errWithContext.Context()["count"]; !ok || val != 5 {
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

func TestWrapf(t *testing.T) {
	cause := derrors.New(derrors.CodeInternal, "underlying error")

	err := derrors.Wrapf(
		cause,
		derrors.CodeActionFailed,
		"action %s failed with code %d",
		"click",
		42,
	)
	if err == nil {
		t.Fatal("Wrapf() returned nil")
	}

	if !errors.Is(err.Cause(), cause) {
		t.Errorf("Wrapf() cause = %v, want %v", err.Cause(), cause)
	}

	if err.Code() != derrors.CodeActionFailed {
		t.Errorf("Wrapf() code = %v, want %v", err.Code(), derrors.CodeActionFailed)
	}

	expectedMsg := "action click failed with code 42"
	if err.Message() != expectedMsg {
		t.Errorf("Wrapf() message = %q, want %q", err.Message(), expectedMsg)
	}

	// Test Wrapf with nil error
	nilErr := derrors.Wrapf(nil, derrors.CodeActionFailed, "action failed")
	if nilErr != nil {
		t.Error("Wrapf() should return nil for nil error")
	}
}

func TestIsCode(t *testing.T) {
	domainErr := derrors.New(derrors.CodeInvalidInput, "test error")
	stdErr := derrors.New(derrors.CodeInternal, "standard error")

	tests := []struct {
		name string
		err  error
		code derrors.Code
		want bool
	}{
		{"domain error matching code", domainErr, derrors.CodeInvalidInput, true},
		{"domain error non-matching code", domainErr, derrors.CodeInvalidConfig, false},
		{"standard error", stdErr, derrors.CodeInvalidInput, false},
		{"nil error", nil, derrors.CodeInvalidInput, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := derrors.IsCode(testCase.err, testCase.code)
			if got != testCase.want {
				t.Errorf(
					"IsCode(%v, %v) = %v, want %v",
					testCase.err,
					testCase.code,
					got,
					testCase.want,
				)
			}
		})
	}
}

func TestGetCode(t *testing.T) {
	domainErr := derrors.New(derrors.CodeInvalidInput, "test error")
	stdErr := derrors.New(derrors.CodeInternal, "standard error")

	tests := []struct {
		name string
		err  error
		want derrors.Code
	}{
		{"domain error", domainErr, derrors.CodeInvalidInput},
		{"standard error", stdErr, derrors.CodeInternal},
		{"nil error", nil, derrors.CodeInternal},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := derrors.GetCode(testCase.err)
			if got != testCase.want {
				t.Errorf("GetCode(%v) = %v, want %v", testCase.err, got, testCase.want)
			}
		})
	}
}

func TestIsAccessibilityError(t *testing.T) {
	tests := []struct {
		name string
		code derrors.Code
		want bool
	}{
		{"accessibility denied", derrors.CodeAccessibilityDenied, true},
		{"accessibility failed", derrors.CodeAccessibilityFailed, true},
		{"other error", derrors.CodeInvalidInput, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := derrors.New(testCase.code, "test error")

			got := derrors.IsAccessibilityError(err)
			if got != testCase.want {
				t.Errorf("IsAccessibilityError(%v) = %v, want %v", err, got, testCase.want)
			}
		})
	}

	// Test with non-domain error
	stdErr := derrors.New(derrors.CodeInternal, "standard error")
	if derrors.IsAccessibilityError(stdErr) {
		t.Error("IsAccessibilityError should return false for non-domain errors")
	}
}

func TestIsUserError(t *testing.T) {
	tests := []struct {
		name string
		code derrors.Code
		want bool
	}{
		{"invalid config", derrors.CodeInvalidConfig, true},
		{"invalid input", derrors.CodeInvalidInput, true},
		{"other error", derrors.CodeIPCFailed, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := derrors.New(testCase.code, "test error")

			got := derrors.IsUserError(err)
			if got != testCase.want {
				t.Errorf("IsUserError(%v) = %v, want %v", err, got, testCase.want)
			}
		})
	}

	// Test with non-domain error
	stdErr := derrors.New(derrors.CodeInternal, "standard error")
	if derrors.IsUserError(stdErr) {
		t.Error("IsUserError should return false for non-domain errors")
	}
}

func TestIsTransient(t *testing.T) {
	tests := []struct {
		name string
		code derrors.Code
		want bool
	}{
		{"timeout", derrors.CodeTimeout, true},
		{"ipc failed", derrors.CodeIPCFailed, true},
		{"other error", derrors.CodeInvalidInput, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := derrors.New(testCase.code, "test error")

			got := derrors.IsTransient(err)
			if got != testCase.want {
				t.Errorf("IsTransient(%v) = %v, want %v", err, got, testCase.want)
			}
		})
	}

	// Test with non-domain error
	stdErr := derrors.New(derrors.CodeInternal, "standard error")
	if derrors.IsTransient(stdErr) {
		t.Error("IsTransient should return false for non-domain errors")
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
