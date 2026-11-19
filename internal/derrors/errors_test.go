package derrors_test

import (
	"errors"
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
)

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

	nilErr := derrors.Wrap(nil, derrors.CodeActionFailed, "action failed")
	if nilErr != nil {
		t.Error("Wrap() should return nil for nil error")
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

	nilErr := derrors.Wrapf(nil, derrors.CodeActionFailed, "action failed")
	if nilErr != nil {
		t.Error("Wrapf() should return nil for nil error")
	}
}

func TestIsCode(t *testing.T) {
	domainErr := derrors.New(derrors.CodeInvalidInput, "test error")
	stdErr := errors.New("standard error") //nolint:err113 // dynamic errors needed for testing

	tests := []struct {
		name string
		err  error
		code derrors.Code
		want bool
	}{
		{"domain error matching code", domainErr, derrors.CodeInvalidInput, true},
		{"domain error non-matching code", domainErr, derrors.CodeInvalidConfig, false},
		{"standard error", stdErr, derrors.CodeInternal, false},
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
	stdErr := errors.New("standard error") //nolint:err113 // dynamic errors needed for testing

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
