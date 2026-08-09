package logger

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
	"testing"

	"go.uber.org/multierr"
)

// errFileSinkFailed stands in for a log file that failed to flush.
var errFileSinkFailed = errors.New("log file failed to flush")

// A tee core reports its sinks with multierr: the standard-stream noise must be
// dropped without taking the file sink's real failure with it.
func TestGenuineSyncFailure_KeepsRealFailureFromCombinedError(t *testing.T) {
	stdStreamErr := &fs.PathError{Op: syncOp, Path: os.Stderr.Name(), Err: syscall.EBADF}

	combined := multierr.Append(stdStreamErr, errFileSinkFailed)

	got := genuineSyncFailure(combined)
	if !errors.Is(got, errFileSinkFailed) {
		t.Errorf("genuineSyncFailure() = %v, want it to keep %v", got, errFileSinkFailed)
	}

	if errors.Is(got, stdStreamErr) {
		t.Errorf("genuineSyncFailure() = %v, want the standard-stream error dropped", got)
	}
}

func TestGenuineSyncFailure_NilWhenEverySinkErrorIsStandardStreamNoise(t *testing.T) {
	combined := multierr.Append(
		&fs.PathError{Op: syncOp, Path: os.Stderr.Name(), Err: syscall.EBADF},
		&fs.PathError{Op: syncOp, Path: os.Stdout.Name(), Err: syscall.ENOTTY},
	)

	got := genuineSyncFailure(combined)
	if got != nil {
		t.Errorf("genuineSyncFailure() = %v, want nil", got)
	}
}

// Only a failed *sync* of a standard stream is noise — anything else about those
// streams is a real error.
func TestGenuineSyncFailure_KeepsNonSyncStandardStreamError(t *testing.T) {
	writeErr := &fs.PathError{Op: "write", Path: os.Stderr.Name(), Err: syscall.EBADF}

	got := genuineSyncFailure(writeErr)
	if !errors.Is(got, writeErr) {
		t.Errorf("genuineSyncFailure() = %v, want it to keep %v", got, writeErr)
	}
}

func TestGenuineSyncFailure_NilForNil(t *testing.T) {
	got := genuineSyncFailure(nil)
	if got != nil {
		t.Errorf("genuineSyncFailure(nil) = %v, want nil", got)
	}
}
