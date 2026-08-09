package logger

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// errSyncSinkFailed stands in for a sink whose flush genuinely failed — the one
// case Close() must report without abandoning the rest of the teardown.
var errSyncSinkFailed = errors.New("sink failed to flush")

// errLogFileCloseFailed stands in for a log file that failed to close.
var errLogFileCloseFailed = errors.New("log file failed to close")

// failingSyncWriter is a console sink whose Sync always fails. Init installs a
// zapcore.WriteSyncer verbatim, so the failure surfaces through Close().
type failingSyncWriter struct{}

func (failingSyncWriter) Write(p []byte) (int, error) { return len(p), nil }

func (failingSyncWriter) Sync() error { return errSyncSinkFailed }

// spyLogFile stands in for the rotating log file, counting the closes it gets
// and optionally failing them.
type spyLogFile struct {
	closes   int
	closeErr error
}

func (s *spyLogFile) Write(p []byte) (int, error) { return len(p), nil }

func (s *spyLogFile) Close() error {
	s.closes++

	return s.closeErr
}

// initWithLogFile puts the package into the state a running daemon is in: a
// global logger over the given console sink, plus a log file underneath it.
func initWithLogFile(t *testing.T, consoleWriter io.Writer, file *spyLogFile) {
	t.Helper()

	Reset()
	t.Cleanup(resetAll)

	initErr := Init("info", "", true, 10, 5, 30, consoleWriter)
	if initErr != nil {
		t.Fatalf("Init() error = %v", initErr)
	}

	logFileMu.Lock()
	defer logFileMu.Unlock()

	logFile = file
}

// resetAll clears every piece of package state a test may have installed.
func resetAll() {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	globalLogger = nil
	logFile = nil
}

// assertTornDown checks that Close() left no logger or log file behind.
func assertTornDown(t *testing.T) {
	t.Helper()

	logFileMu.RLock()
	defer logFileMu.RUnlock()

	if globalLogger != nil {
		t.Error("Close() left the global logger set")
	}

	if logFile != nil {
		t.Error("Close() left the log file set")
	}
}

// A sync failure must not cost the teardown below it: the log file is still
// closed and the global logger still cleared, and the failure is still reported.
func TestClose_SyncFailureStillTearsDown(t *testing.T) {
	file := &spyLogFile{}

	initWithLogFile(t, failingSyncWriter{}, file)

	closeErr := Close()
	if !errors.Is(closeErr, errSyncSinkFailed) {
		t.Errorf("Close() error = %v, want it to wrap %v", closeErr, errSyncSinkFailed)
	}

	if file.closes != 1 {
		t.Errorf("log file closed %d times, want 1", file.closes)
	}

	assertTornDown(t)
}

// Both teardown steps can fail; Close() reports both, not just the first.
func TestClose_ReportsEveryTeardownFailure(t *testing.T) {
	file := &spyLogFile{closeErr: errLogFileCloseFailed}

	initWithLogFile(t, failingSyncWriter{}, file)

	closeErr := Close()
	if !errors.Is(closeErr, errSyncSinkFailed) {
		t.Errorf(
			"Close() error = %v, want it to wrap the sync failure %v",
			closeErr,
			errSyncSinkFailed,
		)
	}

	if !errors.Is(closeErr, errLogFileCloseFailed) {
		t.Errorf(
			"Close() error = %v, want it to wrap the log file failure %v",
			closeErr,
			errLogFileCloseFailed,
		)
	}

	assertTornDown(t)
}

// Close() runs on the shutdown path, where a second call after a failed one is
// ordinary: it must report nothing and must not close the log file twice.
func TestClose_SecondCloseAfterFailureIsQuiet(t *testing.T) {
	file := &spyLogFile{closeErr: errLogFileCloseFailed}

	initWithLogFile(t, failingSyncWriter{}, file)

	firstErr := Close()
	if firstErr == nil {
		t.Fatal("Close() error = nil, want the teardown failures")
	}

	secondErr := Close()
	if secondErr != nil {
		t.Errorf("second Close() error = %v, want nil", secondErr)
	}

	if file.closes != 1 {
		t.Errorf("log file closed %d times, want 1", file.closes)
	}
}

// A teardown with nothing to report stays silent.
func TestClose_QuietWhenEveryStepSucceeds(t *testing.T) {
	file := &spyLogFile{}

	initWithLogFile(t, &nopSyncWriter{}, file)

	closeErr := Close()
	if closeErr != nil {
		t.Errorf("Close() error = %v, want nil", closeErr)
	}

	if file.closes != 1 {
		t.Errorf("log file closed %d times, want 1", file.closes)
	}

	assertTornDown(t)
}

// nopSyncWriter is a console sink that flushes cleanly.
type nopSyncWriter struct {
	bytes.Buffer
}

func (*nopSyncWriter) Sync() error { return nil }
