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
// Syncing the process's standard streams is expected to fail whenever they are
// not a syncable device — a pipe, a terminal, or a descriptor already torn down
// by an exiting test binary. That is the long-standing zap-on-stdout/stderr
// caveat (uber-go/zap#328, #370), not a lost log line, so those errors are
// dropped here. Every other error — a log file that failed to flush, above all
// — is returned unchanged so callers still report it.
//
// zap combines the per-sink errors of a tee core with multierr, so each error is
// classified individually and only the survivors are returned.
func genuineSyncFailure(err error) error {
	if err == nil {
		return nil
	}

	var failures error

	for _, sinkErr := range multierr.Errors(err) {
		if isStandardStreamSyncError(sinkErr) {
			continue
		}

		failures = multierr.Append(failures, sinkErr)
	}

	return failures
}

// isStandardStreamSyncError reports whether err is a failed sync of stdout or
// stderr with one of the errnos those streams produce when they cannot be
// synced at all.
func isStandardStreamSyncError(err error) bool {
	var pathErr *fs.PathError

	if !errors.As(err, &pathErr) || pathErr.Op != syncOp {
		return false
	}

	if pathErr.Path != os.Stdout.Name() && pathErr.Path != os.Stderr.Name() {
		return false
	}

	// EINVAL: pipes on Linux. ENOTTY: terminals and pipes on macOS.
	// EBADF: the descriptor is already gone, as during process teardown.
	return errors.Is(pathErr, syscall.EINVAL) ||
		errors.Is(pathErr, syscall.ENOTTY) ||
		errors.Is(pathErr, syscall.EBADF)
}
