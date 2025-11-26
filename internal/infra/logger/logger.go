package logger

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	derrors "github.com/y3owk1n/neru/internal/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	// DefaultDirPerms is the default directory permissions.
	DefaultDirPerms = 0o750
)

var (
	// globalLogger is the global logger instance.
	globalLogger *zap.Logger
	logFile      *lumberjack.Logger
	logFileMu    sync.Mutex
)

// Init configures and initializes the global logger with the specified settings.
// It supports both console and file output with configurable log levels, file rotation,
// and structured or unstructured logging formats.
func Init(
	logLevel, logFilePath string,
	structured bool,
	disableFileLogging bool,
	maxFileSize, maxBackups, maxAge int,
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

	// Configure encoder
	var consoleEncoderConfig, fileEncoderConfig zapcore.EncoderConfig
	if structured {
		consoleEncoderConfig = zap.NewProductionEncoderConfig()
		fileEncoderConfig = zap.NewProductionEncoderConfig()
	} else {
		consoleEncoderConfig = zap.NewDevelopmentEncoderConfig()
		fileEncoderConfig = zap.NewDevelopmentEncoderConfig()
	}

	consoleEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	fileEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	fileEncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// Create console encoder
	consoleEncoder := zapcore.NewConsoleEncoder(consoleEncoderConfig)

	// Create cores slice
	cores := []zapcore.Core{
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level),
	}

	// Add file logging if not disabled
	if !disableFileLogging {
		// Determine log file path
		if logFilePath == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return derrors.Wrap(err, derrors.CodeLoggingFailed, "failed to get home directory")
			}

			logFilePath = filepath.Join(homeDir, "Library", "Logs", "neru", "app.log")
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

		// Create file encoder (no colors)
		var fileEncoder zapcore.Encoder
		if structured {
			fileEncoder = zapcore.NewJSONEncoder(fileEncoderConfig)
		} else {
			fileEncoder = zapcore.NewConsoleEncoder(fileEncoderConfig)
		}

		// Add file core
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
	if globalLogger == nil {
		// Fallback to development logger
		globalLogger, _ = zap.NewDevelopment()
	}

	return globalLogger
}

// Reset resets the global logger instance.
func Reset() {
	globalLogger = nil
}

// Sync flushes any buffered log entries to their outputs.
// This ensures that all pending log messages are written before the application exits.
func Sync() error {
	if globalLogger != nil {
		err := globalLogger.Sync()
		if err != nil {
			return derrors.Wrap(err, derrors.CodeLoggingFailed, "failed to sync logger")
		}
	}

	return nil
}

// Close releases all logger resources and ensures all pending log entries are written.
// It synchronizes the logger and closes the log file if file logging is enabled.
func Close() error {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	if globalLogger != nil {
		err := globalLogger.Sync()
		if err != nil {
			// Ignore common sync errors that occur during shutdown
			if !strings.Contains(err.Error(), "invalid argument") &&
				!strings.Contains(err.Error(), "inappropriate ioctl for device") {
				return derrors.Wrap(err, derrors.CodeLoggingFailed, "failed to sync logger")
			}
		}

		globalLogger = nil
	}

	if logFile != nil {
		// lumberjack.Logger doesn't have a Sync method, but Close will flush
		err := logFile.Close()
		if err != nil {
			return derrors.Wrap(err, derrors.CodeLoggingFailed, "failed to close log file")
		}

		logFile = nil
	}

	return nil
}

// Debug logs a debug-level message with optional structured fields.
// Debug messages are typically used for detailed diagnostic information.
func Debug(msg string, fields ...zap.Field) {
	Get().Debug(msg, fields...)
}

// Info logs an info-level message with optional structured fields.
// Info messages are used for general operational information.
func Info(msg string, fields ...zap.Field) {
	Get().Info(msg, fields...)
}

// Warn logs a warning-level message with optional structured fields.
// Warning messages indicate potentially harmful situations.
func Warn(msg string, fields ...zap.Field) {
	Get().Warn(msg, fields...)
}

// Error logs an error-level message with optional structured fields.
// Error messages indicate serious problems that need attention.
func Error(msg string, fields ...zap.Field) {
	Get().Error(msg, fields...)
}

// Fatal logs a fatal-level message and immediately exits the application.
// Fatal messages indicate unrecoverable errors that require immediate termination.
func Fatal(msg string, fields ...zap.Field) {
	Get().Fatal(msg, fields...)
}

// With creates a new child logger instance with the specified fields added to all log entries.
// This is useful for adding context to all logs from a specific component or operation.
func With(fields ...zap.Field) *zap.Logger {
	return Get().With(fields...)
}
