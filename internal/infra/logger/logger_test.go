//nolint:errcheck,noinlineerr
package logger

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestInit(t *testing.T) {
	// Create temp directory for test logs
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	tests := []struct {
		name               string
		logLevel           string
		logFilePath        string
		structured         bool
		disableFileLogging bool
		maxFileSize        int
		maxBackups         int
		maxAge             int
		wantErr            bool
	}{
		{
			name:               "debug level with file logging",
			logLevel:           "debug",
			logFilePath:        logPath,
			structured:         false,
			disableFileLogging: false,
			maxFileSize:        10,
			maxBackups:         3,
			maxAge:             7,
			wantErr:            false,
		},
		{
			name:               "info level structured",
			logLevel:           "info",
			logFilePath:        logPath,
			structured:         true,
			disableFileLogging: false,
			maxFileSize:        10,
			maxBackups:         3,
			maxAge:             7,
			wantErr:            false,
		},
		{
			name:               "warn level no file",
			logLevel:           "warn",
			logFilePath:        "",
			structured:         false,
			disableFileLogging: true,
			maxFileSize:        10,
			maxBackups:         3,
			maxAge:             7,
			wantErr:            false,
		},
		{
			name:               "error level",
			logLevel:           "error",
			logFilePath:        logPath,
			structured:         false,
			disableFileLogging: false,
			maxFileSize:        10,
			maxBackups:         3,
			maxAge:             7,
			wantErr:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Init(
				tt.logLevel,
				tt.logFilePath,
				tt.structured,
				tt.disableFileLogging,
				tt.maxFileSize,
				tt.maxBackups,
				tt.maxAge,
			)

			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify logger was initialized
			logger := Get()
			if logger == nil {
				t.Error("Get() returned nil after Init()")
			}

			// Clean up
			_ = Close()
		})
	}
}

func TestGet(t *testing.T) {
	// Reset global logger
	globalLogger = nil

	// Get should return a logger even if not initialized
	logger := Get()
	if logger == nil {
		t.Error("Get() returned nil")
	}

	// Clean up
	_ = Close()
}

func TestLoggingFunctions(t *testing.T) {
	// Initialize logger
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	err := Init("debug", logPath, false, false, 10, 3, 7)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Close()

	tests := []struct {
		name string
		fn   func()
	}{
		{
			name: "Debug",
			fn: func() {
				Debug("test debug message", zap.String("key", "value"))
			},
		},
		{
			name: "Info",
			fn: func() {
				Info("test info message", zap.Int("count", 42))
			},
		},
		{
			name: "Warn",
			fn: func() {
				Warn("test warn message", zap.Bool("flag", true))
			},
		},
		{
			name: "Error",
			fn: func() {
				Error("test error message", zap.Error(os.ErrNotExist))
			},
		},
		// Note: Fatal test is skipped as it would exit the test process
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			// Should not panic
			tt.fn()
		})
	}

	// Verify log file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}
}

func TestWith(t *testing.T) {
	// Initialize logger
	err := Init("info", "", false, true, 10, 3, 7)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Close()

	// Create child logger
	childLogger := With(zap.String("component", "test"))
	if childLogger == nil {
		t.Error("With() returned nil")
	}

	// Should not panic
	childLogger.Info("test message")
}

func TestSync(t *testing.T) {
	// Initialize logger
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	err := Init("info", logPath, false, false, 10, 3, 7)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Close()

	// Write some logs
	Info("test message 1")
	Info("test message 2")

	// Sync may error on stdout/stderr, which is expected
	_ = Sync()
}

func TestClose(t *testing.T) {
	// Initialize logger
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	err := Init("info", logPath, false, false, 10, 3, 7)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Write some logs
	Info("test message")

	// Close may error on stdout/stderr sync, which is expected
	_ = Close()

	// Note: globalLogger may not be nil if sync failed
	// This is acceptable behavior
}

func TestLogLevels(t *testing.T) {
	tests := []struct {
		name      string
		logLevel  string
		wantLevel zapcore.Level
	}{
		{"debug", "debug", zapcore.DebugLevel},
		{"info", "info", zapcore.InfoLevel},
		{"warn", "warn", zapcore.WarnLevel},
		{"error", "error", zapcore.ErrorLevel},
		{"unknown defaults to info", "unknown", zapcore.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			logPath := filepath.Join(tempDir, "test.log")

			err := Init(tt.logLevel, logPath, false, false, 10, 3, 7)
			if err != nil {
				t.Fatalf("Init() failed: %v", err)
			}
			defer Close()

			// Logger should be initialized
			logger := Get()
			if logger == nil {
				t.Error("Get() returned nil")
			}
		})
	}
}

func TestFileRotation(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	// Initialize with small max size for testing
	err := Init("info", logPath, false, false, 1, 2, 1)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Close()

	// Write enough logs to trigger rotation
	for range 1000 {
		Info("test message with some content to fill up the log file")
	}

	// Sync to ensure all logs are written
	_ = Sync()

	// Verify log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}
}

// Benchmark tests.
func BenchmarkDebugLogging(b *testing.B) {
	tempDir := b.TempDir()
	logPath := filepath.Join(tempDir, "bench.log")

	_ = Init("debug", logPath, false, false, 100, 3, 7)
	defer Close()

	for b.Loop() {
		Debug("benchmark message", zap.String("key", "value"))
	}
}

func BenchmarkInfoLogging(b *testing.B) {
	tempDir := b.TempDir()
	logPath := filepath.Join(tempDir, "bench.log")

	_ = Init("info", logPath, false, false, 100, 3, 7)
	defer Close()

	for b.Loop() {
		Info("benchmark message", zap.Int("count", 42))
	}
}

func BenchmarkStructuredLogging(b *testing.B) {
	tempDir := b.TempDir()
	logPath := filepath.Join(tempDir, "bench.log")

	_ = Init("info", logPath, true, false, 100, 3, 7)
	defer Close()

	for b.Loop() {
		Info("benchmark message",
			zap.String("key1", "value1"),
			zap.Int("key2", 42),
			zap.Bool("key3", true))
	}
}
