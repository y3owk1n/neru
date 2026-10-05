package logger

import (
	"io"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/term"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/y3owk1n/neru/internal/derrors"
)

const (
	// DefaultDirPerms is the default directory permissions.
	DefaultDirPerms = 0o750
)

var (
	// globalLogger is the global logger instance.
	globalLogger *zap.Logger
	// logFile is the rotating file sink underneath the global logger, held as an
	// io.WriteCloser because writing to it and closing it is all this package
	// asks of it — and because a test can then stand in for it.
	logFile   io.WriteCloser
	logFileMu sync.RWMutex
)

// Init configures and initializes the global logger with the specified settings.
// It supports both console and file output with configurable log levels and file rotation.
// Console output is one tracing-style line per entry, colored on a terminal.
// File output is JSON for machine parsing.
func Init(
	logLevel, logFilePath string,
	disableFileLogging bool,
	maxFileSize, maxBackups, maxAge int,
	consoleWriter io.Writer,
) error {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	// Close existing log file if any
	if logFile != nil {
		closeErr := logFile.Close()
		if closeErr != nil {
			return derrors.Wrap(
				closeErr,
				derrors.CodeLoggingFailed,
				"failed to close existing log file",
			)
		}

		logFile = nil
	}

	// Determine log level
	level := zapcore.InfoLevel

	switch logLevel {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	// Determine the effective console writer early so terminal detection
	// targets the actual output rather than always checking os.Stdout.
	if consoleWriter == nil {
		consoleWriter = os.Stdout
	}

	isTerminal := false

	if f, ok := consoleWriter.(*os.File); ok {
		isTerminal = term.IsTerminal(int(f.Fd()))
	}

	fileEncoderConfig := zap.NewProductionEncoderConfig()

	fileEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	fileEncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	cores := []zapcore.Core{
		newConsoleCore(zapcore.AddSync(consoleWriter), level, isTerminal),
	}

	// Add file logging if not disabled
	if !disableFileLogging {
		// Determine log file path
		if logFilePath == "" {
			defaultLogFilePath, err := defaultLogFilePath()
			if err != nil {
				return derrors.Wrap(
					err,
					derrors.CodeLoggingFailed,
					"failed to determine default log file path",
				)
			}

			logFilePath = defaultLogFilePath
		}

		// Create log directory
		logDir := filepath.Dir(logFilePath)

		mkdirErr := os.MkdirAll(logDir, DefaultDirPerms)
		if mkdirErr != nil {
			return derrors.Wrap(
				mkdirErr,
				derrors.CodeLoggingFailed,
				"failed to create log directory",
			)
		}

		// Create lumberjack logger for file rotation
		logFile = &lumberjack.Logger{
			Filename:   logFilePath,
			MaxSize:    maxFileSize, // Size in MB
			MaxBackups: maxBackups,  // Maximum number of old log files to retain
			MaxAge:     maxAge,      // Maximum number of days to retain old log files
			Compress:   true,        // Compress old log files
		}

		// Create file encoder (JSON for machine parsing)
		fileEncoder := zapcore.NewJSONEncoder(fileEncoderConfig)

		cores = append(cores, zapcore.NewCore(fileEncoder, zapcore.AddSync(logFile), level))
	}

	// Create core with both console and file output (if enabled)
	core := zapcore.NewTee(cores...)

	// Create logger
	globalLogger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return nil
}

// Get retrieves the global logger instance.
// If the logger hasn't been initialized, it returns a development logger as a fallback.
func Get() *zap.Logger {
	logFileMu.RLock()

	if globalLogger != nil {
		logger := globalLogger

		logFileMu.RUnlock()

		return logger
	}

	logFileMu.RUnlock()

	logFileMu.Lock()
	defer logFileMu.Unlock()

	if globalLogger == nil {
		globalLogger, _ = zap.NewDevelopment()
	}

	return globalLogger
}

// Reset resets the global logger instance.
func Reset() {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	globalLogger = nil
}

// Sync flushes any buffered log entries to their outputs.
// Pending log messages are written before the process exits.
func Sync() error {
	logFileMu.RLock()

	defer logFileMu.RUnlock()

	if globalLogger != nil {
		err := genuineSyncFailure(globalLogger.Sync())
		if err != nil {
			return derrors.Wrap(err, derrors.CodeLoggingFailed, "failed to sync logger")
		}
	}

	return nil
}

// Close releases all logger resources and ensures all pending log entries are written.
// It synchronizes the logger and closes the log file if file logging is enabled.
//
// Every teardown step runs even when an earlier one fails, and the failures are
// reported together: a logger that could not be flushed still gets its log file
// closed and its global cleared, rather than leaking the file handle behind a
// stale logger. Close therefore leaves nothing behind to close twice, so the
// second Close of a shutdown path that already failed is a quiet no-op.
func Close() error {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	var failures error

	if globalLogger != nil {
		failures = appendLoggingFailure(
			failures,
			genuineSyncFailure(globalLogger.Sync()),
			"failed to sync logger",
		)

		globalLogger = nil
	}

	if logFile != nil {
		// lumberjack.Logger doesn't have a Sync method, but Close will flush
		failures = appendLoggingFailure(failures, logFile.Close(), "failed to close log file")

		logFile = nil
	}

	return failures
}

// appendLoggingFailure adds err, under the given message, to the failures a
// teardown has collected so far. A step that succeeded adds nothing, so the
// caller can run every step and report whatever the run collected.
func appendLoggingFailure(failures, err error, message string) error {
	if err == nil {
		return failures
	}

	return multierr.Append(failures, derrors.Wrap(err, derrors.CodeLoggingFailed, message))
}
