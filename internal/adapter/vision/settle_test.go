package vision

import (
	"context"
	"image"
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/ports"
)

// fakeScreen is a region whose captures follow a script, one value per
// capture, holding the last one once the script runs out.
type fakeScreen struct {
	script   []int
	captures int
}

func (s *fakeScreen) frames() settleFrames[int] {
	return settleFrames[int]{
		capture: func() (int, error) {
			at := min(s.captures, len(s.script)-1)
			s.captures++

			return s.script[at], nil
		},
		same:    func(a, b int) bool { return a == b },
		release: func(int) {},
	}
}

func TestCaptureOnceChanged_WaitsForTheAppToMoveAndStop(t *testing.T) {
	tests := []struct {
		name      string
		script    []int
		before    int
		hasBefore bool
		want      int
		unchanged bool
	}{
		{
			name:   "the old screen twice is not settled, the new one twice is",
			script: []int{0, 0, 1, 2, 2},
			before: 0, hasBefore: true,
			want: 2,
		},
		{
			name:   "a screen that moved before the first capture",
			script: []int{3, 3},
			before: 0, hasBefore: true,
			want: 3,
		},
		{
			name:   "nothing remembered settles on two equal captures",
			script: []int{5, 5},
			want:   5,
		},
		{
			name:   "a screen that never changes runs out unchanged",
			script: []int{0},
			before: 0, hasBefore: true,
			want: 0, unchanged: true,
		},
		{
			name:   "a screen that keeps changing runs out on the last capture",
			script: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
			before: 0, hasBefore: true,
			want: -1,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			screen := &fakeScreen{script: testCase.script}

			got, unchanged, err := captureOnceChanged(
				context.Background(), testCase.before, testCase.hasBefore, screen.frames(),
			)
			if err != nil {
				t.Fatalf("captureOnceChanged: %v", err)
			}

			if testCase.want >= 0 && got != testCase.want {
				t.Fatalf("returned capture %d, want %d", got, testCase.want)
			}

			if testCase.want < 0 &&
				got != screen.script[min(screen.captures, len(screen.script))-1] {
				t.Fatalf("returned capture %d, want the last one taken", got)
			}

			if unchanged != testCase.unchanged {
				t.Fatalf("unchanged = %v, want %v", unchanged, testCase.unchanged)
			}
		})
	}
}

func TestCaptureOnceChanged_StopsWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	screen := &fakeScreen{script: []int{0}}

	_, _, err := captureOnceChanged(ctx, 0, true, screen.frames())
	if err == nil {
		t.Fatal("captureOnceChanged kept watching a canceled read")
	}
}

// TestLastRead_RememberAndRecall_AreSafeAcrossGoroutines covers the two
// readers of the last read: the read of a scrolled screen, off the mode lock,
// and a read for a key under it, which can overlap.
func TestLastRead_RememberAndRecall_AreSafeAcrossGoroutines(t *testing.T) {
	var memory lastRead

	region := image.Rect(0, 0, 10, 10)
	lines := [][]ports.TextPiece{{{Text: "a"}}}

	var zero readFrame

	var writers sync.WaitGroup

	writers.Go(func() {
		for range 500 {
			memory.remember(region, zero, lines)
		}
	})

	for range 500 {
		frame, recalled, ok := memory.recall(region)
		releaseFrame(frame)

		if ok && len(recalled) != 1 {
			t.Fatalf("recalled %d lines, want the 1 remembered", len(recalled))
		}
	}

	writers.Wait()
}

// TestLastRead_Forget_LeavesNothingToRecall covers ForgetLastRead, which
// vim mode calls on leaving so a whole screen's frame is not kept until the
// next read.
func TestLastRead_Forget_LeavesNothingToRecall(t *testing.T) {
	var memory lastRead

	region := image.Rect(0, 0, 10, 10)

	var zero readFrame

	memory.remember(region, zero, [][]ports.TextPiece{{{Text: "a"}}})
	memory.forget()

	if frame, lines, ok := memory.recall(region); ok {
		releaseFrame(frame)
		t.Fatalf("recalled %v after forget, want nothing", lines)
	}

	// Forgetting with nothing remembered gives back nothing.
	memory.forget()
}
