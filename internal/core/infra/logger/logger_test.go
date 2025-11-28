package logger_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"go.uber.org/zap"
)

func TestGet(t *testing.T) {
	// Initially should return a development logger
	log := logger.Get()
	if log == nil {
		t.Fatal("Get() returned nil")
	}

	// Should be a zap logger
	_ = log.With(zap.String("test", "value")) // Should not panic
}

func TestReset(t *testing.T) {
	// Set a logger
	original := logger.Get()

	// Reset
	logger.Reset()

	// Get should return a new logger
	newLogger := logger.Get()
	if newLogger == nil {
		t.Fatal("Get() returned nil after reset")
	}

	// They should be different instances
	if original == newLogger {
		t.Error("Reset() did not create a new logger instance")
	}
}

func TestInit(t *testing.T) {
	// Reset before test
	logger.Reset()

	var buf bytes.Buffer

	// Test basic initialization
	err := logger.Init("info", "", true, true, 10, 5, 30, &buf)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Logger should be set
	log := logger.Get()
	if log == nil {
		t.Fatal("Get() returned nil after Init")
	}

	// Test logging to buffer
	log.Info("test message", zap.String("key", "value"))
	output := buf.String()

	if !strings.Contains(output, "test message") {
		t.Errorf("Log output does not contain expected message. Got: %s", output)
	}

	if !strings.Contains(output, `"key": "value"`) {
		t.Errorf("Log output does not contain structured field. Got: %s", output)
	}
}

func TestSync(t *testing.T) {
	// Reset and init
	logger.Reset()

	err := logger.Init("info", "", true, true, 10, 5, 30, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Sync should not error
	syncErr := logger.Sync()
	if syncErr != nil {
		t.Errorf("Sync() error = %v", syncErr)
	}
}

func TestClose(t *testing.T) {
	// Reset and init
	logger.Reset()

	err := logger.Init("info", "", true, true, 10, 5, 30, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Close should not error
	closeErr := logger.Close()
	if closeErr != nil {
		t.Errorf("Close() error = %v", closeErr)
	}

	// After close, Get should still return a logger (fallback)
	log := logger.Get()
	if log == nil {
		t.Error("Get() returned nil after Close")
	}
}

func TestWith(t *testing.T) {
	logger.Reset()

	err := logger.Init("info", "", true, true, 10, 5, 30, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// With should return a logger
	childLogger := logger.With(zap.String("component", "test"))
	if childLogger == nil {
		t.Error("With() returned nil")
	}
}
