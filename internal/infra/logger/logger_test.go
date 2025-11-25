package logger_test

import (
	"os"
	"path/filepath"
	"testing"

	plogger "github.com/y3owk1n/neru/internal/infra/logger"
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			initErr := plogger.Init(
				test.logLevel,
				test.logFilePath,
				test.structured,
				test.disableFileLogging,
				test.maxFileSize,
				test.maxBackups,
				test.maxAge,
			)

			if (initErr != nil) != test.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", initErr, test.wantErr)
			}

			// Verify logger was initialized
			logger := plogger.Get()
			if logger == nil {
				t.Error("Get() returned nil after Init()")
			}

			// Clean up
			_ = plogger.Close()
		})
	}
}

func TestGet(t *testing.T) {
	// Reset global logger
	plogger.GlobalLogger = nil

	// Get should return a logger even if not initialized
	logger := plogger.Get()
	if logger == nil {
		t.Error("Get() returned nil")
	}

	// Clean up
	_ = plogger.Close()
}

func TestLoggingFunctions(t *testing.T) {
	// Initialize logger
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	initErr := plogger.Init("debug", logPath, false, false, 10, 3, 7)
	if initErr != nil {
		t.Fatalf("Init() failed: %v", initErr)
	}

	defer func() {
		_ = plogger.Close()
	}()

	tests := []struct {
		name string
		fn   func()
	}{
		{
			name: "Debug",
			fn: func() {
				plogger.Debug("test debug message", zap.String("key", "value"))
			},
		},
		{
			name: "Info",
			fn: func() {
				plogger.Info("test info message", zap.Int("count", 42))
			},
		},
		{
			name: "Warn",
			fn: func() {
				plogger.Warn("test warn message", zap.Bool("flag", true))
			},
		},
		{
			name: "Error",
			fn: func() {
				plogger.Error("test error message", zap.Error(os.ErrNotExist))
			},
		},
		// Note: Fatal test is skipped as it would exit the test process
	}

	for _, test := range tests {
		t.Run(test.name, func(_ *testing.T) {
			// Should not panic
			test.fn()
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
	err := plogger.Init("info", "", false, true, 10, 3, 7)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	defer func() {
		_ = plogger.Close()
	}()

	// Create child logger
	childLogger := plogger.With(zap.String("component", "test"))
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

	initErr := plogger.Init("info", logPath, false, false, 10, 3, 7)
	if initErr != nil {
		t.Fatalf("Init() failed: %v", initErr)
	}

	defer plogger.Close() //nolint:errcheck

	// Write some logs
	plogger.Info("test message 1")
	plogger.Info("test message 2")

	// Sync may error on stdout/stderr, which is expected
	_ = plogger.Sync()
}

func TestClose(t *testing.T) {
	// Initialize logger
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	initErr := plogger.Init("info", logPath, false, false, 10, 3, 7)
	if initErr != nil {
		t.Fatalf("Init() failed: %v", initErr)
	}

	// Write some logs
	plogger.Info("test message")

	// Close may error on stdout/stderr sync, which is expected
	_ = plogger.Close()

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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tempDir := t.TempDir()
			logPath := filepath.Join(tempDir, "test.log")

			initErr := plogger.Init(test.logLevel, logPath, false, false, 10, 3, 7)
			if initErr != nil {
				t.Fatalf("Init() failed: %v", initErr)
			}

			defer func() {
				_ = plogger.Close()
			}()

			// Logger should be initialized
			logger := plogger.Get()
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
	initErr := plogger.Init("info", logPath, false, false, 1, 2, 1)
	if initErr != nil {
		t.Fatalf("Init() failed: %v", initErr)
	}

	defer func() {
		_ = plogger.Close()
	}()

	// Write enough logs to trigger rotation
	for range 1000 {
		plogger.Info("test message with some content to fill up the log file")
	}

	// Sync to ensure all logs are written
	_ = plogger.Sync()

	// Verify log file exists
	_, initErr = os.Stat(logPath)
	if os.IsNotExist(initErr) {
		t.Error("Log file was not created")
	}
}
