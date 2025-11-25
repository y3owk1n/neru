package errors_test

import (
	"testing"

	derrors "github.com/y3owk1n/neru/internal/errors"
)

func BenchmarkNew(b *testing.B) {
	for b.Loop() {
		_ = derrors.New(derrors.CodeInvalidInput, "test error")
	}
}

func BenchmarkNewf(b *testing.B) {
	for b.Loop() {
		_ = derrors.Newf(derrors.CodeInvalidConfig, "invalid value: %d", 42)
	}
}

func BenchmarkError_WithContext(b *testing.B) {
	err := derrors.New(derrors.CodeActionFailed, "action failed")

	for b.Loop() {
		_ = err.WithContext("key", "value")
	}
}
