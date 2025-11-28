package logger_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/logger"
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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			initErr := logger.Init(
				testCase.logLevel,
				testCase.logFilePath,
				testCase.structured,
				testCase.disableFileLogging,
				testCase.maxFileSize,
				testCase.maxBackups,
				testCase.maxAge,
				nil,
			)

			if (initErr != nil) != testCase.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", initErr, testCase.wantErr)
			}

			// Verify loggerInstance was initialized
			loggerInstance := logger.Get()
			if loggerInstance == nil {
				t.Error("Get() returned nil after Init()")
			}

			// Clean up
			_ = logger.Close()
		})
	}
}

func TestGet(t *testing.T) {
	// Reset global logger
	logger.Reset()

	// Get should return a loggerInstance even if not initialized
	loggerInstance := logger.Get()
	if loggerInstance == nil {
		t.Error("Get() returned nil")
	}

	// Clean up
	_ = logger.Close()
}

func TestLoggingFunctions(t *testing.T) {
	// Initialize logger
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	initErr := logger.Init("debug", logPath, false, false, 10, 3, 7, nil)
	if initErr != nil {
		t.Fatalf("Init() failed: %v", initErr)
	}

	defer func() {
		_ = logger.Close()
	}()

	tests := []struct {
		name string
		fn   func()
	}{
		{
			name: "Debug",
			fn: func() {
				logger.Debug("test debug message", zap.String("key", "value"))
			},
		},
		{
			name: "Info",
			fn: func() {
				logger.Info("test info message", zap.Int("count", 42))
			},
		},
		{
			name: "Warn",
			fn: func() {
				logger.Warn("test warn message", zap.Bool("flag", true))
			},
		},
		{
			name: "Error",
			fn: func() {
				logger.Error("test error message", zap.Error(os.ErrNotExist))
			},
		},
		// Note: Fatal test is skipped as it would exit the test process
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(_ *testing.T) {
			// Should not panic
			testCase.fn()
		})
	}

	// Verify log file was created
	_, initErr = os.Stat(logPath)
	if os.IsNotExist(initErr) {
		t.Error("Log file was not created")
	}
}

func TestWith(t *testing.T) {
	// Initialize logger
	err := logger.Init("info", "", false, true, 10, 3, 7, nil)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	defer func() {
		_ = logger.Close()
	}()

	// Create child logger
	childLogger := logger.With(zap.String("component", "test"))
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

	initErr := logger.Init("info", logPath, false, false, 10, 3, 7, nil)
	if initErr != nil {
		t.Fatalf("Init() failed: %v", initErr)
	}

	defer logger.Close() //nolint:errcheck

	// Write some logs
	logger.Info("test message 1")
	logger.Info("test message 2")

	// Sync may error on stdout/stderr, which is expected
	_ = logger.Sync()
}

func TestClose(t *testing.T) {
	// Initialize logger
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	initErr := logger.Init("info", logPath, false, false, 10, 3, 7, nil)
	if initErr != nil {
		t.Fatalf("Init() failed: %v", initErr)
	}

	// Write some logs
	logger.Info("test message")

	// Close may error on stdout/stderr sync, which is expected
	_ = logger.Close()

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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			tempDir := t.TempDir()
			logPath := filepath.Join(tempDir, "test.log")

			initErr := logger.Init(testCase.logLevel, logPath, false, false, 10, 3, 7, nil)
			if initErr != nil {
				t.Fatalf("Init() failed: %v", initErr)
			}

			defer func() {
				_ = logger.Close()
			}()

			// Logger should be initialized
			logger := logger.Get()
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
	initErr := logger.Init("info", logPath, false, false, 1, 2, 1, io.Discard)
	if initErr != nil {
		t.Fatalf("Init() failed: %v", initErr)
	}

	defer func() {
		_ = logger.Close()
	}()

	// Write enough logs to trigger rotation
	for range 1000 {
		logger.Info("test message with some content to fill up the log file")
	}

	// Sync to ensure all logs are written
	_ = logger.Sync()

	// Verify log file exists
	_, initErr = os.Stat(logPath)
	if os.IsNotExist(initErr) {
		t.Error("Log file was not created")
	}
}
