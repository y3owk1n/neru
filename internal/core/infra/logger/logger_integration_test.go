//go:build integration

package logger_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/y3owk1n/neru/internal/core/infra/logger"
)

const (
	logLevelDebug = "debug"
	logLevelInfo  = "info"
	logLevelWarn  = "warn"
	logLevelError = "error"
)

func TestInitIntegration(t *testing.T) {
	// Create temp directory for test logs
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	tests := []struct {
		name               string
		logLevel           string
		logFilePath        string
		disableFileLogging bool
		maxFileSize        int
		maxBackups         int
		maxAge             int
		wantErr            bool
	}{
		{
			name:               "debug level with file logging",
			logLevel:           logLevelDebug,
			logFilePath:        logPath,
			disableFileLogging: false,
			maxFileSize:        10,
			maxBackups:         3,
			maxAge:             7,
			wantErr:            false,
		},
		{
			name:               "info level with file logging",
			logLevel:           logLevelInfo,
			logFilePath:        logPath,
			disableFileLogging: false,
			maxFileSize:        10,
			maxBackups:         3,
			maxAge:             7,
			wantErr:            false,
		},
		{
			name:               "warn level no file",
			logLevel:           logLevelWarn,
			logFilePath:        "",
			disableFileLogging: true,
			maxFileSize:        10,
			maxBackups:         3,
			maxAge:             7,
			wantErr:            false,
		},
		{
			name:               "error level",
			logLevel:           logLevelError,
			logFilePath:        logPath,
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

func TestGetIntegration(t *testing.T) {
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

	initErr := logger.Init(logLevelDebug, logPath, false, 10, 3, 7, nil)
	if initErr != nil {
		t.Fatalf("Init() failed: %v", initErr)
	}

	defer func() {
		_ = logger.Close()
	}()

	tests := []struct {
		name string
		fn   func()
		// wantLevel is the level the entry must be recorded at, and wantFields
		// the structured fields that must survive into the file. Asserting the
		// level matters because a mis-wired level would silently downgrade
		// error reporting while still "not panicking"; asserting the fields
		// catches an encoder that drops structured context.
		wantLevel  string
		wantFields map[string]any
	}{
		{
			name: "Debug",
			fn: func() {
				logger.Debug("test debug message", zap.String("key", "value"))
			},
			wantLevel:  "DEBUG",
			wantFields: map[string]any{"key": "value"},
		},
		{
			name: "Info",
			fn: func() {
				logger.Info("test info message", zap.Int("count", 42))
			},
			wantLevel:  "INFO",
			wantFields: map[string]any{"count": float64(42)},
		},
		{
			name: "Warn",
			fn: func() {
				logger.Warn("test warn message", zap.Bool("flag", true))
			},
			wantLevel:  "WARN",
			wantFields: map[string]any{"flag": true},
		},
		{
			name: "Error",
			fn: func() {
				logger.Error("test error message", zap.Error(os.ErrNotExist))
			},
			wantLevel:  "ERROR",
			wantFields: map[string]any{"error": os.ErrNotExist.Error()},
		},
		// Note: Fatal test is skipped as it would exit the test process
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			wantMessage := "test " + strings.ToLower(testCase.name) + " message"

			testCase.fn()

			// Flush so the entry is on disk before we read it back.
			_ = logger.Sync()

			entry := findLogEntry(t, logPath, wantMessage)

			if got := entry["level"]; got != testCase.wantLevel {
				t.Errorf(
					"entry %q logged at level %v, want %q",
					wantMessage,
					got,
					testCase.wantLevel,
				)
			}

			for key, want := range testCase.wantFields {
				got, present := entry[key]
				if !present {
					t.Errorf("entry %q is missing field %q; got %v", wantMessage, key, entry)

					continue
				}

				if got != want {
					t.Errorf("entry %q field %q = %v (%T), want %v (%T)",
						wantMessage, key, got, got, want, want)
				}
			}
		})
	}

	// Verify log file was created
	_, initErr = os.Stat(logPath)
	if os.IsNotExist(initErr) {
		t.Error("Log file was not created")
	}
}

// findLogEntry reads the JSON-lines log at path and returns the single entry
// whose "msg" equals message, failing the test if there is not exactly one.
// Decoding rather than substring-matching means a field has to be genuinely
// encoded as structured data, not merely appear somewhere in the text.
func findLogEntry(t *testing.T, path, message string) map[string]any {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	var matches []map[string]any

	for lineNo, line := range strings.Split(strings.TrimSpace(string(contents)), "\n") {
		if line == "" {
			continue
		}

		var entry map[string]any

		err := json.Unmarshal([]byte(line), &entry)
		if err != nil {
			t.Fatalf("log line %d is not valid JSON (%v): %s", lineNo+1, err, line)
		}

		if entry["msg"] == message {
			matches = append(matches, entry)
		}
	}

	if len(matches) != 1 {
		t.Fatalf("found %d log entries with msg %q, want exactly 1", len(matches), message)
	}

	return matches[0]
}

func TestWithIntegration(t *testing.T) {
	// Initialize logger
	err := logger.Init(logLevelInfo, "", true, 10, 3, 7, nil)
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

func TestSyncIntegration(t *testing.T) {
	// Initialize logger
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	initErr := logger.Init(logLevelInfo, logPath, false, 10, 3, 7, nil)
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

func TestCloseIntegration(t *testing.T) {
	// Initialize logger
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	initErr := logger.Init(logLevelInfo, logPath, false, 10, 3, 7, nil)
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
		{logLevelDebug, logLevelDebug, zapcore.DebugLevel},
		{logLevelInfo, logLevelInfo, zapcore.InfoLevel},
		{logLevelWarn, logLevelWarn, zapcore.WarnLevel},
		{logLevelError, logLevelError, zapcore.ErrorLevel},
		{"unknown defaults to info", "unknown", zapcore.InfoLevel},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			tempDir := t.TempDir()
			logPath := filepath.Join(tempDir, "test.log")

			initErr := logger.Init(testCase.logLevel, logPath, false, 10, 3, 7, nil)
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
	initErr := logger.Init(logLevelInfo, logPath, false, 1, 2, 1, io.Discard)
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
