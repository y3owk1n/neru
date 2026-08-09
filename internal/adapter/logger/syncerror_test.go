package logger

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"go.uber.org/multierr"
)

// errFileSinkFailed stands in for a log file that failed to flush.
var errFileSinkFailed = errors.New("log file failed to flush")

// pipeStream returns a stream standing in for a standard stream that has been
// piped — the shape a test binary's stderr has, and one a flush can never
// succeed on.
func pipeStream(t *testing.T) *os.File {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}

	t.Cleanup(func() {
		_ = writer.Close()
		_ = reader.Close()
	})

	return writer
}

// regularFileStream returns a stream standing in for a standard stream that has
// been redirected to a real file, which a flush can and should succeed on.
func regularFileStream(t *testing.T) *os.File {
	t.Helper()

	file, err := os.Create(filepath.Join(t.TempDir(), "redirected.log"))
	if err != nil {
		t.Fatalf("os.Create() error = %v", err)
	}

	t.Cleanup(func() { _ = file.Close() })

	return file
}

func syncFailure(stream *os.File, errno syscall.Errno) *fs.PathError {
	return &fs.PathError{Op: syncOp, Path: stream.Name(), Err: errno}
}

// A tee core reports its sinks with multierr: the unflushable-stream noise must
// be dropped without taking the file sink's real failure with it.
func TestGenuineSyncFailure_KeepsRealFailureFromCombinedError(t *testing.T) {
	stream := pipeStream(t)

	combined := multierr.Append(syncFailure(stream, syscall.EBADF), errFileSinkFailed)

	got := genuineSyncFailureFor(combined, stream)
	if !errors.Is(got, errFileSinkFailed) {
		t.Errorf("genuineSyncFailureFor() = %v, want it to keep %v", got, errFileSinkFailed)
	}

	if errors.Is(got, syscall.EBADF) {
		t.Errorf("genuineSyncFailureFor() = %v, want the stream error dropped", got)
	}
}

func TestGenuineSyncFailure_NilWhenEverySinkErrorIsStreamNoise(t *testing.T) {
	first, second := pipeStream(t), pipeStream(t)

	combined := multierr.Append(
		syncFailure(first, syscall.EBADF),
		syncFailure(second, syscall.ENOTTY),
	)

	got := genuineSyncFailureFor(combined, first, second)
	if got != nil {
		t.Errorf("genuineSyncFailureFor() = %v, want nil", got)
	}
}

// A standard stream keeps its name when it is redirected, so a real file behind
// it must still be reported — the name alone must not buy silence.
func TestGenuineSyncFailure_ReportsFailureOnRedirectedRegularFile(t *testing.T) {
	stream := regularFileStream(t)

	failure := syncFailure(stream, syscall.EBADF)

	got := genuineSyncFailureFor(failure, stream)
	if !errors.Is(got, failure) {
		t.Errorf("genuineSyncFailureFor() = %v, want it to keep %v", got, failure)
	}
}

// Only a failed *flush* of a stream is noise — anything else about it is a real
// error.
func TestGenuineSyncFailure_KeepsNonSyncStreamError(t *testing.T) {
	stream := pipeStream(t)

	writeErr := &fs.PathError{Op: "write", Path: stream.Name(), Err: syscall.EBADF}

	got := genuineSyncFailureFor(writeErr, stream)
	if !errors.Is(got, writeErr) {
		t.Errorf("genuineSyncFailureFor() = %v, want it to keep %v", got, writeErr)
	}
}

// A flush failure with an errno that says the destination is fine stays a real
// failure whatever the stream is.
func TestGenuineSyncFailure_KeepsUnexpectedErrnoOnStream(t *testing.T) {
	stream := pipeStream(t)

	failure := syncFailure(stream, syscall.EIO)

	got := genuineSyncFailureFor(failure, stream)
	if !errors.Is(got, failure) {
		t.Errorf("genuineSyncFailureFor() = %v, want it to keep %v", got, failure)
	}
}

func TestGenuineSyncFailure_NilForNil(t *testing.T) {
	got := genuineSyncFailure(nil)
	if got != nil {
		t.Errorf("genuineSyncFailure(nil) = %v, want nil", got)
	}
}
