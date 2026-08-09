package logger

import (
	"errors"
	"io/fs"
	"os"
	"syscall"

	"go.uber.org/multierr"
)

// syncOp is the Op an *fs.PathError carries when a flush to the sink failed.
const syncOp = "sync"

// genuineSyncFailure returns the part of a zap Sync error that is a real sink
// failure, or nil when there is none.
//
// Flushing the process's standard streams is expected to fail whenever they are
// not a flushable destination — a pipe, a terminal, or a descriptor already torn
// down by an exiting test binary. That is the long-standing zap-on-stdout/stderr
// caveat (uber-go/zap#328, #370), not a lost log line, so those errors are
// dropped here. Every other error — a log file that failed to flush, above all —
// is returned unchanged so callers still report it.
//
// zap combines the per-sink errors of a tee core with multierr, so each error is
// classified individually and only the survivors are returned.
func genuineSyncFailure(err error) error {
	return genuineSyncFailureFor(err, os.Stdout, os.Stderr)
}

// genuineSyncFailureFor is genuineSyncFailure against an explicit set of
// standard streams, so tests can supply streams they control.
func genuineSyncFailureFor(err error, streams ...*os.File) error {
	if err == nil {
		return nil
	}

	var failures error

	for _, sinkErr := range multierr.Errors(err) {
		if isUnflushableStreamError(sinkErr, streams) {
			continue
		}

		failures = multierr.Append(failures, sinkErr)
	}

	return failures
}

// isUnflushableStreamError reports whether err is a failed flush of one of the
// given streams, with one of the errnos a stream raises when it cannot be
// flushed at all, on a destination that could never have been flushed anyway.
func isUnflushableStreamError(err error, streams []*os.File) bool {
	var pathErr *fs.PathError

	if !errors.As(err, &pathErr) || pathErr.Op != syncOp {
		return false
	}

	// EINVAL: pipes on Linux. ENOTTY: terminals and pipes on macOS.
	// EBADF: the descriptor is already gone, as during process teardown.
	unflushableErrno := errors.Is(pathErr, syscall.EINVAL) ||
		errors.Is(pathErr, syscall.ENOTTY) ||
		errors.Is(pathErr, syscall.EBADF)
	if !unflushableErrno {
		return false
	}

	for _, stream := range streams {
		if pathErr.Path == stream.Name() {
			return isUnflushable(stream)
		}
	}

	return false
}

// isUnflushable reports whether a flush of stream could never have succeeded.
// The standard streams keep their name (/dev/stdout, /dev/stderr) when they are
// redirected, so the name alone says nothing: a redirect to a regular file is
// flushable, and a failure there is real and must be reported. A stream whose
// descriptor is already gone counts as unflushable — there is nothing left to
// flush it to.
func isUnflushable(stream *os.File) bool {
	info, err := stream.Stat()
	if err != nil {
		return true
	}

	return !info.Mode().IsRegular()
}
