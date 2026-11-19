//go:build integration

package logger_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/y3owk1n/neru/internal/adapter/logger"
)

const (
	logLevelDebug = "debug"
	logLevelInfo  = "info"
	logLevelWarn  = "warn"
	logLevelError = "error"
)

func TestLoggingFunctions(t *testing.T) {
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
		// error reporting while still "not panicking". Asserting the fields
		// catches an encoder that drops structured context.
		wantLevel  string
		wantFields map[string]any
	}{
		{
			name: "Debug",
			fn: func() {
				logger.Get().Debug("test debug message", zap.String("key", "value"))
			},
			wantLevel:  "DEBUG",
			wantFields: map[string]any{"key": "value"},
		},
		{
			name: "Info",
			fn: func() {
				logger.Get().Info("test info message", zap.Int("count", 42))
			},
			wantLevel:  "INFO",
			wantFields: map[string]any{"count": float64(42)},
		},
		{
			name: "Warn",
			fn: func() {
				logger.Get().Warn("test warn message", zap.Bool("flag", true))
			},
			wantLevel:  "WARN",
			wantFields: map[string]any{"flag": true},
		},
		{
			name: "Error",
			fn: func() {
				logger.Get().Error("test error message", zap.Error(os.ErrNotExist))
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

			core := logger.Get().Core()
			if !core.Enabled(testCase.wantLevel) {
				t.Errorf("log_level %q does not enable %v", testCase.logLevel, testCase.wantLevel)
			}

			if core.Enabled(testCase.wantLevel - 1) {
				t.Errorf(
					"log_level %q enables %v, want nothing below %v",
					testCase.logLevel,
					testCase.wantLevel-1,
					testCase.wantLevel,
				)
			}
		})
	}
}
