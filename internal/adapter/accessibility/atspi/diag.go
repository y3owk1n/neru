//go:build linux

// Diagnostics for a frame scan: records per-application call failures and
// distinguishes fatal bus errors from apps that are merely uncooperative, so a
// scan that finds nothing can say why.

package atspi

import (
	"context"
	"errors"
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/derrors"
)

// atspiScanDiag records diagnostics for a single ClickableNodes scan. It rides
// on the scan context so the leaf D-Bus helpers can report a scan-fatal failure
// (a per-call timeout, a closed connection, or the target app disappearing from
// the bus) without threading an extra parameter through the whole walk. A scan
// that ends up empty for one of those reasons can then surface an actionable
// error instead of a silent empty result; benign per-node errors are ignored.
type atspiScanDiag struct {
	mu  sync.Mutex
	err error
}

// note records the first scan-fatal error. Safe for concurrent use by the
// materialize/walk goroutines.
func (d *atspiScanDiag) note(err error) {
	d.mu.Lock()
	if d.err == nil {
		d.err = err
	}
	d.mu.Unlock()
}

// fatalErr returns the first scan-fatal error, or nil. Call after the scan's
// goroutines have joined.
func (d *atspiScanDiag) fatalErr() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.err
}

// atspiScanDiagKey is the context key for the active scan's *atspiScanDiag.
type atspiScanDiagKey struct{}

// isScanFatalErr reports whether err means the whole scan is failing rather than
// a single benign node: a per-call context deadline, a closed connection, or the
// application having disappeared from the bus.
func isScanFatalErr(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, dbus.ErrClosed) {
		return true
	}

	if dbusErr, ok := errors.AsType[dbus.Error](err); ok {
		switch dbusErr.Name {
		case atspiErrServiceUnknown, atspiErrNoReply, atspiErrDisconnected:
			return true
		}
	}

	return false
}

// noteCallErr records a scan-fatal error on the scan's diagnostic (if one is
// attached to ctx). Benign errors, and contexts with no diag attached (e.g. the
// frame-selection reads in findActiveFrame), are ignored.
func (c *Client) noteCallErr(ctx context.Context, err error) {
	if !isScanFatalErr(err) {
		return
	}

	if d, ok := ctx.Value(atspiScanDiagKey{}).(*atspiScanDiag); ok {
		d.note(err)
	}
}

// scanFailureError turns a scan-fatal error into a user-facing hints error,
// distinguishing an unresponsive app (per-call deadline) from one that has gone.
func scanFailureError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return derrors.New(derrors.CodeTimeout,
			"AT-SPI element scan timed out; the app may be slow or "+
				"unresponsive to accessibility queries")
	}

	return derrors.Wrap(err, derrors.CodeAccessibilityFailed,
		"AT-SPI element scan failed; the app may have exited or lost its "+
			"accessibility connection")
}
