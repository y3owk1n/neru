// Package logger provides logging functionality for the Neru application.
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	globalLogger *zap.Logger
	logFile      *lumberjack.Logger
	logFileMu    sync.Mutex
)

// Package logger provides structured logging functionality for the Neru application.
// using the zap logging library with file rotation support.

// Init initializes the global logger.
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
		err := logFile.Close()
		if err != nil {
			return fmt.Errorf("failed to close existing log file: %w", err)
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

	// Set time encoding
	consoleEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	fileEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Set level encoding - no colors for file output
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
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			logFilePath = filepath.Join(homeDir, "Library", "Logs", "neru", "app.log")
		}

		// Create log directory
		logDir := filepath.Dir(logFilePath)
		err := os.MkdirAll(logDir, 0o750)
		if err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
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

// Get returns the global logger.
func Get() *zap.Logger {
	if globalLogger == nil {
		// Fallback to development logger
		globalLogger, _ = zap.NewDevelopment()
	}
	return globalLogger
}

// Sync flushes any buffered log entries.
func Sync() error {
	if globalLogger != nil {
		err := globalLogger.Sync()
		if err != nil {
			return fmt.Errorf("failed to sync logger: %w", err)
		}
	}
	return nil
}

// Close closes the log file and syncs the logger.
func Close() error {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	if globalLogger != nil {
		err := globalLogger.Sync()
		if err != nil {
			// Ignore common sync errors that occur during shutdown
			if !strings.Contains(err.Error(), "invalid argument") &&
				!strings.Contains(err.Error(), "inappropriate ioctl for device") {
				return fmt.Errorf("failed to sync logger: %w", err)
			}
		}
		globalLogger = nil
	}

	if logFile != nil {
		// lumberjack.Logger doesn't have a Sync method, but Close will flush
		err := logFile.Close()
		if err != nil {
			return fmt.Errorf("failed to close log file: %w", err)
		}
		logFile = nil
	}

	return nil
}

// Debug logs a debug message.
func Debug(msg string, fields ...zap.Field) {
	Get().Debug(msg, fields...)
}

// Info logs an info message.
func Info(msg string, fields ...zap.Field) {
	Get().Info(msg, fields...)
}

// Warn logs a warning message.
func Warn(msg string, fields ...zap.Field) {
	Get().Warn(msg, fields...)
}

// Error logs an error message.
func Error(msg string, fields ...zap.Field) {
	Get().Error(msg, fields...)
}

// Fatal logs a fatal message and exits.
func Fatal(msg string, fields ...zap.Field) {
	Get().Fatal(msg, fields...)
}

// With creates a child logger with the given fields.
func With(fields ...zap.Field) *zap.Logger {
	return Get().With(fields...)
}
