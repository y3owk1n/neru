package compositorcli_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/platform/compositorcli"
	"github.com/y3owk1n/neru/internal/derrors"
)

// The test binary doubles as the compositor CLI under test. A query spawns a
// process and reads its stdout, and the only way to pin what happens when that
// process is missing, fails, hangs or answers nonsense is to have one that
// really does — but a shell script would only run on two of the three operating
// systems this package is built for. So TestMain re-enters this binary as the
// fake CLI when the mode variable is set, and the parent sets it with t.Setenv
// for the length of one case.
const (
	fakeCLIModeEnv = "NERU_TEST_COMPOSITOR_CLI_MODE"

	// modeAnswers prints the JSON a well-behaved compositor CLI would.
	modeAnswers = "answers"
	// modeGarbage exits successfully having printed something that is not JSON.
	modeGarbage = "garbage"
	// modeFails exits non-zero, as a CLI whose compositor refused does.
	modeFails = "fails"
	// modeHangs never answers, so only the deadline ends the query.
	modeHangs = "hangs"
)

// fakeCLIExitCode is the status modeFails exits with. Any non-zero value would
// do; naming it keeps the assertion and the process agreeing.
const fakeCLIExitCode = 3

// fakeCLIAnswer is the JSON modeAnswers prints, and answerPayload the shape it
// decodes into.
const fakeCLIAnswer = `{"width": 946, "height": 942}`

type answerPayload struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// fakeCLIGarbage is what modeGarbage prints instead of JSON. It stands in for
// whatever a real compositor CLI writes when it will not answer — a sentence,
// often carrying a window title — and is spelled so that finding it inside an
// error message can only mean the error repeated it.
const fakeCLIGarbage = "Refusing: the focused window is <MyPrivateDocument.txt>"

// fakeCLIHang is longer than any deadline a case gives the query, so the case
// measures the deadline rather than this sleep.
const fakeCLIHang = 30 * time.Second

func TestMain(m *testing.M) {
	switch os.Getenv(fakeCLIModeEnv) {
	case "":
		os.Exit(m.Run())
	case modeAnswers:
		_, _ = os.Stdout.WriteString(fakeCLIAnswer)

		os.Exit(0)
	case modeGarbage:
		_, _ = os.Stdout.WriteString(fakeCLIGarbage + "\n")

		os.Exit(0)
	case modeFails:
		os.Exit(fakeCLIExitCode)
	case modeHangs:
		time.Sleep(fakeCLIHang)
		os.Exit(0)
	default:
		os.Exit(1)
	}
}

// fakeCLI puts this binary in the given mode for the length of the test and
// returns the name to query it under.
func fakeCLI(t *testing.T, mode string) string {
	t.Helper()
	t.Setenv(fakeCLIModeEnv, mode)

	return os.Args[0]
}

// TestQueryContext_DecodesWhatTheCLIAnswers is the ordinary path: a compositor
// CLI that answers gets decoded into the caller's struct and reports no
// failure.
func TestQueryContext_DecodesWhatTheCLIAnswers(t *testing.T) {
	var answer answerPayload

	err := compositorcli.QueryContext(
		context.Background(), &answer, fakeCLI(t, modeAnswers), "-j", "activewindow",
	)
	if err != nil {
		t.Fatalf("QueryContext() error = %v, want nil", err)
	}

	if answer.Width != 946 || answer.Height != 942 {
		t.Errorf("decoded %+v, want the width and height the CLI printed", answer)
	}
}

// TestQueryContext_ReportsEveryWayAQueryCanFail is the whole point of this
// package.
//
// A query that could not be run, that the compositor refused, that outlived its
// deadline or that answered with something undecodable are four failures, and
// none of them is an answer. Reporting them as one — as a bare false, which is
// what a compositor with nothing focused also returns — is how a wedged
// swaymsg came to read as an unfocused desktop and send every caller silently
// to the whole screen (#1493).
//
// Each reason has to name the CLI that failed and say what went wrong, because
// the person reading it is looking at a log line and deciding whether their
// compositor, their PATH or Neru is broken.
func TestQueryContext_ReportsEveryWayAQueryCanFail(t *testing.T) {
	cases := []struct {
		name       string
		mode       string
		cli        string
		timeout    time.Duration
		wantReason string
		wantCode   derrors.Code
	}{
		{
			name:       "a CLI that is not installed",
			cli:        "neru-no-such-compositor-cli",
			wantReason: "could not be run",
			wantCode:   derrors.CodeBridgeFailed,
		},
		{
			name:       "a compositor that refused",
			mode:       modeFails,
			wantReason: "exited with an error",
			wantCode:   derrors.CodeBridgeFailed,
		},
		{
			// The one failure the rest of the tree already has a word for, so
			// it carries that word rather than a second spelling of it.
			name:       "a CLI that never answered",
			mode:       modeHangs,
			timeout:    50 * time.Millisecond,
			wantReason: "did not answer",
			wantCode:   derrors.CodeTimeout,
		},
		{
			name:       "an answer that is not JSON",
			mode:       modeGarbage,
			wantReason: "could not be decoded",
			wantCode:   derrors.CodeBridgeFailed,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			cli := testCase.cli
			if cli == "" {
				cli = fakeCLI(t, testCase.mode)
			}

			ctx := context.Background()

			if testCase.timeout > 0 {
				timed, cancel := context.WithTimeout(ctx, testCase.timeout)
				defer cancel()

				ctx = timed
			}

			var answer answerPayload

			err := compositorcli.QueryContext(ctx, &answer, cli, "-j", "activewindow")
			if err == nil {
				t.Fatal("QueryContext() reported success; a failed query must not " +
					"read as a compositor with nothing to report")
			}

			if !derrors.IsCode(err, testCase.wantCode) {
				t.Errorf("QueryContext() code = %q, want %q",
					derrors.GetCode(err), testCase.wantCode)
			}

			message := derrors.Message(err)
			if !strings.Contains(message, cli) {
				t.Errorf("message %q does not name the CLI %q that failed", message, cli)
			}

			if !strings.Contains(message, testCase.wantReason) {
				t.Errorf("message %q does not say %q, so it does not say what went wrong",
					message, testCase.wantReason)
			}
		})
	}
}

// TestQueryContext_EndsWithTheCallersDeadline keeps a wedged CLI off the
// activation path.
//
// These queries run under the mode handler's lock, where a native call that
// waits is a keyboard that has stopped responding. The deadline has to end the
// query itself and not merely the process — a CLI killed with its stdout still
// open would otherwise leave the read waiting for the pipe.
func TestQueryContext_EndsWithTheCallersDeadline(t *testing.T) {
	const deadline = 50 * time.Millisecond

	// Spawning a process is the bulk of what this case measures, and on a
	// loaded runner that is not fast. The slack is wide enough that the
	// assertion is about the deadline having ended the query at all, rather
	// than about how quickly a machine can fork.
	const spawnSlack = 5 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	var answer answerPayload

	start := time.Now()
	err := compositorcli.QueryContext(ctx, &answer, fakeCLI(t, modeHangs), "-j", "activewindow")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("QueryContext() reported success from a CLI that never answered")
	}

	if elapsed > deadline+spawnSlack {
		t.Errorf("QueryContext() took %v to give up on a %v deadline; a wedged "+
			"compositor CLI must not outlive it", elapsed, deadline)
	}
}

// TestQuery_BoundsACallerThatBroughtNoDeadline pins the other entry point. Most
// callers have no deadline of their own — hint activation reads the focused
// window's geometry from wherever it stands — so the query supplies one rather
// than inheriting the absence of one.
func TestQuery_BoundsACallerThatBroughtNoDeadline(t *testing.T) {
	var answer answerPayload

	start := time.Now()
	err := compositorcli.Query(&answer, fakeCLI(t, modeHangs), "-j", "activewindow")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Query() reported success from a CLI that never answered")
	}

	if elapsed >= fakeCLIHang {
		t.Fatalf("Query() waited %v for a CLI that never answers; it must bound "+
			"itself at %v", elapsed, compositorcli.QueryTimeout)
	}
}

// TestQuery_LeavesTheCallersValueAloneWhenTheQueryFails keeps a failed query
// from looking like an answer of zeroes. A caller that ignored the error would
// otherwise read a window at the origin with no size.
func TestQuery_LeavesTheCallersValueAloneWhenTheQueryFails(t *testing.T) {
	answer := answerPayload{Width: 946, Height: 942}

	err := compositorcli.Query(&answer, "neru-no-such-compositor-cli")
	if err == nil {
		t.Fatal("Query() reported success from a CLI that does not exist")
	}

	if answer.Width != 946 || answer.Height != 942 {
		t.Errorf("a failed query overwrote the caller's value with %+v", answer)
	}
}

// TestQueryContext_QuotesNothingTheCompositorSaid keeps a compositor's output
// out of the log line the failure becomes.
//
// A compositor CLI prints window titles, workspace names and application
// identifiers, none of which Neru logs (AGENTS.md, Conventions). A JSON decoder
// asked to parse that output complains by quoting where it stopped, so the
// reason a query failed is classified here rather than repeated.
func TestQueryContext_QuotesNothingTheCompositorSaid(t *testing.T) {
	var answer answerPayload

	err := compositorcli.QueryContext(
		context.Background(), &answer, fakeCLI(t, modeGarbage), "-j", "activewindow",
	)
	if err == nil {
		t.Fatal("QueryContext() reported success from a CLI that printed no JSON")
	}

	if strings.Contains(err.Error(), "MyPrivateDocument") {
		t.Errorf("error %q repeats what the CLI printed; the reason must be "+
			"classified, not quoted", err.Error())
	}
}

// TestQueryContext_CarriesTheUnderlyingFailure keeps the wrapped cause
// reachable, so a reader is not left with the sentence alone when the sentence
// is not enough.
func TestQueryContext_CarriesTheUnderlyingFailure(t *testing.T) {
	var answer answerPayload

	err := compositorcli.QueryContext(
		context.Background(), &answer, "neru-no-such-compositor-cli",
	)

	if !errors.Is(err, exec.ErrNotFound) && !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Query() error = %v, want it to carry why the CLI could not be run", err)
	}
}
