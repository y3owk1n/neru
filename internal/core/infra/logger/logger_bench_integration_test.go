//go:build integration

package logger_test

import (
	"path/filepath"
	"testing"

	plogger "github.com/y3owk1n/neru/internal/core/infra/logger"
	"go.uber.org/zap"
)

func BenchmarkDebugLogging(b *testing.B) {
	tempDir := b.TempDir()
	logPath := filepath.Join(tempDir, "bench.log")

	_ = plogger.Init("debug", logPath, false, false, 100, 3, 7, nil)

	defer func() {
		_ = plogger.Close()
	}()

	for b.Loop() {
		plogger.Debug("benchmark message", zap.String("key", "value"))
	}
}

func BenchmarkInfoLogging(b *testing.B) {
	tempDir := b.TempDir()
	logPath := filepath.Join(tempDir, "bench.log")

	_ = plogger.Init("info", logPath, false, false, 100, 3, 7, nil)

	defer plogger.Close() //nolint:errcheck

	for b.Loop() {
		plogger.Info("benchmark message", zap.Int("count", 42))
	}
}

func BenchmarkStructuredLogging(b *testing.B) {
	tempDir := b.TempDir()
	logPath := filepath.Join(tempDir, "bench.log")

	_ = plogger.Init("info", logPath, true, false, 100, 3, 7, nil)

	defer plogger.Close() //nolint:errcheck

	for b.Loop() {
		plogger.Info("benchmark message",
			zap.String("key1", "value1"),
			zap.Int("key2", 42),
			zap.Bool("key3", true))
	}
}
