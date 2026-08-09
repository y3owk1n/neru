package logger_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/logger"
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
	err := logger.Init("info", "", true, 10, 5, 30, &buf)
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

	err := logger.Init("info", "", true, 10, 5, 30, &bytes.Buffer{})
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

	err := logger.Init("info", "", true, 10, 5, 30, &bytes.Buffer{})
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

	err := logger.Init("info", "", true, 10, 5, 30, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// With should return a logger
	childLogger := logger.With(zap.String("component", "test"))
	if childLogger == nil {
		t.Error("With() returned nil")
	}
}

// errSinkFailed stands in for a sink that genuinely failed to flush.
var errSinkFailed = errors.New("sink failed to flush")

// syncStubWriter is a console sink whose Sync always fails with syncErr. It
// implements zapcore.WriteSyncer, so Init installs it verbatim and the failure
// surfaces through the package's own Sync/Close.
type syncStubWriter struct {
	syncErr error
}

func (w syncStubWriter) Write(p []byte) (int, error) { return len(p), nil }

func (w syncStubWriter) Sync() error { return w.syncErr }

// stdStreamSyncErr is what os.Stderr.Sync() returns when stderr is a pipe
// rather than a syncable device — the zap-on-stderr caveat that floods test
// teardown with warnings.
func stdStreamSyncErr(errno syscall.Errno) error {
	return &fs.PathError{Op: "sync", Path: "/dev/stderr", Err: errno}
}

func TestSyncAndClose_IgnoreStandardStreamSyncFailure(t *testing.T) {
	tests := []struct {
		name  string
		errno syscall.Errno
	}{
		{name: "bad file descriptor", errno: syscall.EBADF},
		{name: "invalid argument", errno: syscall.EINVAL},
		{name: "inappropriate ioctl for device", errno: syscall.ENOTTY},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			logger.Reset()
			t.Cleanup(logger.Reset)

			initErr := logger.Init(
				"info", "", true, 10, 5, 30,
				syncStubWriter{syncErr: stdStreamSyncErr(testCase.errno)},
			)
			if initErr != nil {
				t.Fatalf("Init() error = %v", initErr)
			}

			syncErr := logger.Sync()
			if syncErr != nil {
				t.Errorf("Sync() error = %v, want nil", syncErr)
			}

			closeErr := logger.Close()
			if closeErr != nil {
				t.Errorf("Close() error = %v, want nil", closeErr)
			}
		})
	}
}

func TestSyncAndClose_ReportGenuineSinkFailure(t *testing.T) {
	logger.Reset()
	t.Cleanup(logger.Reset)

	initErr := logger.Init("info", "", true, 10, 5, 30, syncStubWriter{syncErr: errSinkFailed})
	if initErr != nil {
		t.Fatalf("Init() error = %v", initErr)
	}

	syncErr := logger.Sync()
	if !errors.Is(syncErr, errSinkFailed) {
		t.Errorf("Sync() error = %v, want it to wrap %v", syncErr, errSinkFailed)
	}

	closeErr := logger.Close()
	if !errors.Is(closeErr, errSinkFailed) {
		t.Errorf("Close() error = %v, want it to wrap %v", closeErr, errSinkFailed)
	}
}

// A log-file sink that fails to sync stays a real failure even when its errno
// matches the standard-stream caveat: only the standard streams get the benefit
// of the doubt.
func TestSyncAndClose_ReportFileSinkFailureWithSameErrno(t *testing.T) {
	fileSyncErr := &fs.PathError{
		Op:   "sync",
		Path: filepath.Join(t.TempDir(), "neru.log"),
		Err:  syscall.EBADF,
	}

	logger.Reset()
	t.Cleanup(logger.Reset)

	initErr := logger.Init("info", "", true, 10, 5, 30, syncStubWriter{syncErr: fileSyncErr})
	if initErr != nil {
		t.Fatalf("Init() error = %v", initErr)
	}

	syncErr := logger.Sync()
	if !errors.Is(syncErr, fileSyncErr) {
		t.Errorf("Sync() error = %v, want it to wrap %v", syncErr, fileSyncErr)
	}

	closeErr := logger.Close()
	if !errors.Is(closeErr, fileSyncErr) {
		t.Errorf("Close() error = %v, want it to wrap %v", closeErr, fileSyncErr)
	}
}

// The teardown path that floods the gate: Get() builds a fallback development
// logger on os.Stderr, and closing it must stay quiet even when the test
// binary's stderr is a pipe that cannot be synced.
func TestClose_QuietOnFallbackStderrLogger(t *testing.T) {
	logger.Reset()
	t.Cleanup(logger.Reset)

	logger.Get().Info("teardown smoke check")

	closeErr := logger.Close()
	if closeErr != nil {
		t.Errorf("Close() error = %v, want nil for the stderr fallback logger", closeErr)
	}
}

func TestRaceCondition(t *testing.T) {
	var waitGroup sync.WaitGroup

	logger.Reset()

	// Concurrent logging (exercises Get)
	for range 5 {
		waitGroup.Go(func() {
			for range 100 {
				logger.Info("background logging")
			}
		})
	}

	// Concurrent Init (write path)
	waitGroup.Go(func() {
		_ = logger.Init("info", "", true, 10, 5, 30, os.Stdout)
	})

	// Concurrent Sync (read-copy-unlock path)
	waitGroup.Go(func() {
		for range 50 {
			_ = logger.Sync()
		}
	})
	// Concurrent Reset (write path)
	waitGroup.Go(func() {
		for range 10 {
			logger.Reset()
		}
	})
	// Concurrent Close (write path)
	waitGroup.Go(func() {
		for range 10 {
			_ = logger.Close()
		}
	})

	waitGroup.Wait()

	// The race detector is the primary check. Also assert the package still
	// hands out a usable logger afterwards, so the test is not vacuous without
	// -race.
	if logger.Get() == nil {
		t.Fatal("Get() returned nil after concurrent init/close")
	}

	// Logging must not panic on a closed logger.
	logger.Get().Info("post-race smoke check")
}
