//go:build darwin

package vision

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// TestRunHelper_PipesTheRequestAndTheAnswer covers the helper's process: it
// reads the request from stdin and its stdout comes back, a helper that fails
// the read, and one still running when the read is canceled is killed.
func TestRunHelper_PipesTheRequestAndTheAnswer(t *testing.T) {
	tests := []struct {
		name       string
		executable string
		arg        string
		cancel     time.Duration
		want       string
		wantErr    bool
	}{
		{name: "echoes stdin", executable: "/bin/cat", arg: "-", want: "pixels in, pieces out"},
		{
			name:       "a failed exit fails the read",
			executable: "/usr/bin/false",
			arg:        "-",
			wantErr:    true,
		},
		{
			name:       "a canceled read kills it",
			executable: "/bin/sleep",
			arg:        "30",
			cancel:     50 * time.Millisecond,
			wantErr:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()

			if test.cancel > 0 {
				var cancel context.CancelFunc

				ctx, cancel = context.WithTimeout(ctx, test.cancel)
				defer cancel()
			}

			started := time.Now()
			got, err := runHelper(
				ctx,
				test.executable,
				test.arg,
				bytes.NewBufferString("pixels in, pieces out"),
			)

			if test.wantErr != (err != nil) {
				t.Fatalf("err = %v, want an error: %v", err, test.wantErr)
			}

			if !test.wantErr && string(got) != test.want {
				t.Fatalf("stdout = %q, want %q", got, test.want)
			}

			if elapsed := time.Since(started); elapsed > 5*time.Second {
				t.Fatalf("took %v, the helper was not killed on cancel", elapsed)
			}
		})
	}
}
