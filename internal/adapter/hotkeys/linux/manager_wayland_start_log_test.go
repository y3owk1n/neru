//go:build linux

package linux

import (
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The manager is the only caller of the evdev listener's Start, and it is where
// a refusal becomes advice the user reads. Two different failures arrive on the
// same error return: "this build has no evdev reader compiled in"
// (CodeNotSupported, from the no-cgo stub) and "the reader exists but could not
// read /dev/input" (anything else). Only the second is fixable by joining the
// `input` group, so only the second may say so.
//
// The count matters as much as the wording. Registration calls
// ensureWaylandStarted once per hotkey, and the Linux sleep/reload recovery in
// `internal/app/sleep_handler_linux.go` retries registration up to ten times, so
// an unlatched warning repeats dozens of times for an answer that cannot change
// before the next build.

const inputGroupAdvice = "`input` group"

func newObservedManager(t *testing.T) (*Manager, *observer.ObservedLogs) {
	t.Helper()

	core, logs := observer.New(zapcore.DebugLevel)

	return NewManager(zap.New(core)), logs
}

func warnMessages(logs *observer.ObservedLogs) []string {
	var messages []string

	for _, entry := range logs.All() {
		if entry.Level == zapcore.WarnLevel {
			messages = append(messages, entry.Message)
		}
	}

	return messages
}

func TestManager_LogWaylandStartFailure_NotSupportedDropsInputGroupAdvice(t *testing.T) {
	mgr, logs := newObservedManager(t)

	mgr.logWaylandStartFailure(
		derrors.New(
			derrors.CodeNotSupported,
			"Wayland global hotkeys require CGO-enabled Linux builds",
		),
	)

	warns := warnMessages(logs)
	if len(warns) != 1 {
		t.Fatalf("got %d warnings, want exactly 1: %v", len(warns), warns)
	}

	if strings.Contains(warns[0], inputGroupAdvice) || strings.Contains(warns[0], "/dev/input") {
		t.Errorf(
			"the no-cgo warning tells the user to fix /dev/input access, which cannot help "+
				"a build with no evdev reader in it: %q",
			warns[0],
		)
	}

	if !strings.Contains(warns[0], "cgo") {
		t.Errorf("the no-cgo warning does not name the build as the cause: %q", warns[0])
	}
}

func TestManager_LogWaylandStartFailure_NotSupportedWarnsOnce(t *testing.T) {
	mgr, logs := newObservedManager(t)

	err := derrors.New(derrors.CodeNotSupported, "no evdev without cgo")
	for range 12 {
		mgr.logWaylandStartFailure(err)
	}

	if warns := warnMessages(logs); len(warns) != 1 {
		t.Errorf(
			"got %d warnings for a compile-time answer, want 1; the reload and sleep "+
				"recovery loops call this on every attempt",
			len(warns),
		)
	}
}

func TestManager_LogWaylandStartFailure_RecoverableKeepsInputGroupAdvice(t *testing.T) {
	mgr, logs := newObservedManager(t)

	err := derrors.New(
		derrors.CodeHotkeyRegisterFailed,
		"open /dev/input/event0: permission denied",
	)
	for range 3 {
		mgr.logWaylandStartFailure(err)
	}

	warns := warnMessages(logs)
	if len(warns) != 3 {
		t.Fatalf(
			"got %d warnings, want 3: a permissions failure can be fixed while Neru runs, "+
				"so every attempt still reports it",
			len(warns),
		)
	}

	if !strings.Contains(warns[0], inputGroupAdvice) {
		t.Errorf(
			"the recoverable warning lost the remediation that can actually work: %q",
			warns[0],
		)
	}
}
