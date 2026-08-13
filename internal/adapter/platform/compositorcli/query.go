package compositorcli

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/y3owk1n/neru/internal/derrors"
)

// QueryTimeout bounds a query whose caller brought no deadline of its own.
// Every one of these runs on a path that can be holding the mode handler's
// lock, so the bound is short enough that a wedged compositor is a missing
// answer rather than a keyboard that stopped responding.
const QueryTimeout = 500 * time.Millisecond

// pipeGuard bounds how long the read of the CLI's stdout may continue after the
// deadline has killed the process. A CLI that leaked a child inheriting stdout
// would otherwise hold the pipe — and the caller — open past its own deadline.
//
// It is a ceiling on a pathology, not part of the ordinary cost: a killed
// process closes its own stdout, so the guard only ever elapses for a CLI that
// left something else holding the pipe. The honest worst case for one query is
// therefore the deadline plus this, and the reason it is bounded at all is that
// without it there is no worst case.
const pipeGuard = time.Second

// Query runs a compositor CLI under QueryTimeout and decodes its JSON stdout
// into dst.
//
// A nil error means the compositor answered and dst holds what it said. Any
// error means the question went unanswered — which is never the same fact as a
// compositor reporting no focused window, and must not be reported as one.
func Query(dst any, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeout)
	defer cancel()

	return QueryContext(ctx, dst, name, args...)
}

// QueryContext is Query bounded by the caller's context, for call sites that
// already carry a deadline. The deadline is real: CommandContext kills the CLI
// when it expires and pipeGuard keeps the read from waiting past the kill.
func QueryContext(ctx context.Context, dst any, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = pipeGuard

	out, err := cmd.Output()
	if err != nil {
		return derrors.Wrap(err, failureCode(ctx), failureMessage(ctx, name, args, err))
	}

	if json.Unmarshal(out, dst) != nil {
		// The decoder's own complaint is dropped rather than wrapped: it quotes
		// the byte it stopped at, and what a compositor CLI prints is a window
		// title as often as not. Which command failed and that its answer was
		// undecodable is the whole of what a reader can act on anyway.
		return derrors.New(
			derrors.CodeBridgeFailed,
			query(name, args)+": its answer could not be decoded as JSON",
		)
	}

	return nil
}

// failureCode separates the one failure the rest of this tree already has a
// word for. A query that outlived its budget is CodeTimeout, as every other
// bounded wait on Linux reports one; everything else is the bridge itself
// failing, which is what a compositor CLI is here.
func failureCode(ctx context.Context) derrors.Code {
	if ctx.Err() != nil {
		return derrors.CodeTimeout
	}

	return derrors.CodeBridgeFailed
}

// failureMessage says which compositor CLI did not answer and why, in the words
// of the thing the reader can act on: a CLI that is not installed, a compositor
// that refused, and one that never came back are three different problems and
// only one of them is Neru's.
//
// The reason is classified rather than quoted, because what a compositor CLI
// writes to its output is a window's title as often as not, and none of that
// belongs in a log line.
func failureMessage(ctx context.Context, name string, args []string, err error) string {
	switch {
	case ctx.Err() != nil:
		return query(name, args) + " did not answer before the query deadline"
	case errors.As(err, new(*exec.ExitError)):
		return query(name, args) + " exited with an error"
	default:
		return query(name, args) + " could not be run"
	}
}

// query spells the command as it was asked, so the sentence names the
// compositor and the question rather than just the failure. The name and every
// argument are literals in Neru's own source; nothing a compositor said reaches
// here.
func query(name string, args []string) string {
	if len(args) == 0 {
		return name
	}

	return name + " " + strings.Join(args, " ")
}
